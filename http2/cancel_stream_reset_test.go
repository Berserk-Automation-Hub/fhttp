package http2

// [SIGHTGLASS PATCH 11] this file is patch 11's fork-local guard.

// cancelStream() MUST RESET A STREAM EXACTLY WHEN IT HAS NOT ALREADY BEEN RESET.
//
// `didReset` means "we have already sent a RST_STREAM for this stream", so the reset belongs on the
// !didReset branch. This fork had it inverted, which had two consequences: a stream that had already
// been reset got a SECOND RST_STREAM, and a stream that had not been reset got none.
//
// The second reset is a wire divergence. Chrome 153 sent exactly one RST_STREAM per reset stream — 6
// resets on 6 distinct streams across both ground-truth captures, never two on one stream — and the
// duplicate is trivially loggable by any origin. Sightglass reproduced it in 4 of 40 runs (10%)
// through the race between transportResponseBody.Close() and the request context's cancellation.
//
// That race is why this test does NOT drive a request. A 10%-of-runs assertion is not a guard; it is
// a coin flip that happens to be green most of the time. This calls cancelStream() directly with
// each value of didReset and reads the wire, so the condition itself is pinned and the result is the
// same on every run.

import (
	"net"
	"sync"
	"testing"
	"time"
)

// waiterWakeDeadline is how long the harness gives cancelStream() to release a goroutine parked in
// cc.cond.Wait(). On the shipped tree the wake is immediate — cc.cond.Broadcast() runs inside
// cancelStream, before it returns — so this is only ever paid in full by the didReset=true case,
// where no wake is owed and the harness is proving a NEGATIVE.
const waiterWakeDeadline = 2 * time.Second

// cancelHarnessStreamID is the stream the harness plants in cc.streams and cancels. It is named
// rather than written twice so the RST_STREAM assertions can say which stream they expected.
const cancelHarnessStreamID uint32 = 1

// cancelOutcome is everything cancelStream() owes the connection, observed after it returns.
//
// FOUR EFFECTS, NOT ONE. `cc.forgetStreamID(cs.ID)` is a call to `cc.streamByID(id, true)`, and that
// function does four separable things to the connection:
//
//	(1) delete(cc.streams, id)                                — frees one concurrency slot
//	(2) close(cs.done)                                        — releases checkResetOrDone
//	(3) cc.cond.Broadcast()                                   — releases awaitOpenSlotForRequest
//	(4) cc.lastActive / cc.idleTimer.Reset / cc.lastIdle      — lets the connection be reaped
//
// Every one of them needs its own field here, because a partial reimplementation that performs only
// SOME of them is green on the others, and two adversarial readers have now proved it in sequence:
//
//   - the first replaced `cc.forgetStreamID(cs.ID)` with a bare
//     `cc.mu.Lock(); delete(cc.streams, cs.ID); cc.mu.Unlock()`. That defeated a map-only guard AND
//     the Sightglass parity guard that counts free concurrency slots, because the slot really is
//     freed. `doneClosed` was added for it.
//   - the second wrote `cc.mu.Lock(); if cs2 := cc.streams[cs.ID]; cs2 != nil && !cc.closed {
//     delete(cc.streams, cs.ID); close(cs2.done) }; cc.mu.Unlock()` — (1) and (2) but not (3) or
//     (4) — and was green on BOTH layers again. `wokeCondWaiter`, `lastActiveSet`, `lastIdleSet`
//     and `idleTimerRearmed` were added for it.
//
// (3) is not bookkeeping. `awaitOpenSlotForRequest` (transport.go, in the `for` loop that ends in
// `cc.cond.Wait()`) is how a request waits for one of Chrome's 100 concurrency slots on a saturated
// connection, and `cc.cond.Broadcast()` is the ONLY thing that wakes it. Drop the broadcast and a
// request already parked there is never woken when a context-cancelled stream frees its slot: the
// connection wedges permanently for that request, which is the same defect as the slot leak, seen
// from the waiter's side instead of the accountant's.
//
// (4) is a second leak of its own: a connection whose last stream was cancelled never re-arms its
// idle timer, so it is never reaped and sits in the pool forever.
type cancelOutcome struct {
	resets     int  // RST_STREAM frames the peer actually received
	stillInMap bool // was the clientStream still in cc.streams when cancelStream() returned
	doneClosed bool // was cs.done closed by then

	// resetStreams / resetCodes are the IDENTITY of every RST_STREAM the peer received, in order.
	//
	// Counting frames is not enough. `cc.writeStreamReset(cs.ID, ErrCodeCancel, nil)` has three
	// operands and a count pins only the fact that it ran: an adversarial reader reset the WRONG
	// stream (`cs.ID+2`) and sent the WRONG code (`ErrCodeInternal`), and both mutations were green
	// against a harness that read the frame and threw its fields away. PATCHES.md publishes the
	// measured Chrome sequence as `RST_STREAM(N, CANCEL)`, so the stream number and the error code
	// are inside this patch's claim about the wire and have to be read off the wire.
	resetStreams []uint32
	resetCodes   []ErrCode

	// wokeCondWaiter: a goroutine parked in cc.cond.Wait() until len(cc.streams) == 0 was released
	// while the connection was still OPEN, i.e. by cancelStream()'s own broadcast.
	wokeCondWaiter bool
	// waiterFreedByClose: the waiter was released by cc.Close() setting cc.closed instead. This is
	// the harness telling on itself — a wake that came from the teardown proves nothing about
	// cancelStream, so it is reported separately rather than counted as a pass.
	waiterFreedByClose bool

	lastActiveSet    bool // cc.lastActive moved off the zero value the harness planted
	lastIdleSet      bool // cc.lastIdle moved off the zero value the harness planted
	idleTimerRearmed bool // cc.idleTimer actually fired at the rearmed idleTimeout
}

// countResetsAfterCancel sets cs.didReset to the given value, calls cancelStream(), and reports what
// the connection looked like afterwards.
//
// The bookkeeping half is not decoration. cc.forgetStreamID sits INSIDE the same `if` as the reset,
// so the inverted condition dropped all of it along with the frame — see
// TestCancelStreamForgetsTheStream and TestCancelStreamWakesTheSlotWaiter.
func countResetsAfterCancel(t *testing.T, didReset bool) cancelOutcome {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	type seen struct {
		n       int
		streams []uint32
		codes   []ErrCode
	}
	resets := make(chan seen, 1)
	go func() {
		srv, aerr := ln.Accept()
		if aerr != nil {
			resets <- seen{n: -1}
			return
		}
		defer func() { _ = srv.Close() }()
		_ = srv.SetDeadline(time.Now().Add(30 * time.Second))

		// The client preface is 24 bytes that are not a frame; reading it as one desynchronises the
		// framer for everything after it.
		pre := make([]byte, len(clientPreface))
		if _, rerr := readFull(srv, pre); rerr != nil {
			resets <- seen{n: -1}
			return
		}
		fr := NewFramer(srv, srv)
		got := seen{}
		for {
			f, ferr := fr.ReadFrame()
			if ferr != nil {
				break // deadline or EOF: we have seen everything the client sent
			}
			if rf, ok := f.(*RSTStreamFrame); ok {
				got.n++
				got.streams = append(got.streams, rf.StreamID)
				got.codes = append(got.codes, rf.ErrCode)
			}
		}
		resets <- got
	}()

	cli, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = cli.Close() }()

	tr := &Transport{}
	cc, err := tr.NewClientConn(cli)
	if err != nil {
		t.Fatalf("NewClientConn: %v", err)
	}

	// The idle timer is installed BY HAND rather than through Transport.IdleConnTimeout, because
	// NewClientConn would then arm it with cc.onIdleTimeout — which closes the connection out from
	// under the measurement. A long initial duration plus a short idleTimeout makes
	// `cc.idleTimer.Reset(cc.idleTimeout)` inside streamByID directly observable: if the Reset
	// happens the timer fires within idleTimeout, and if it is dropped the timer stays parked for
	// ten minutes and never fires at all.
	const rearmedIdleTimeout = 25 * time.Millisecond
	idleFired := make(chan struct{})
	var fireOnce sync.Once
	idleTimer := time.AfterFunc(10*time.Minute, func() { fireOnce.Do(func() { close(idleFired) }) })
	defer idleTimer.Stop()

	cs := &clientStream{cc: cc, ID: cancelHarnessStreamID, done: make(chan struct{})}
	cc.mu.Lock()
	if cc.streams == nil {
		cc.streams = make(map[uint32]*clientStream)
	}
	cc.streams[cs.ID] = cs
	cs.didReset = didReset
	cc.idleTimeout = rearmedIdleTimeout
	cc.idleTimer = idleTimer
	// Planted zero values, so the assertions below read "cancelStream SET these", not "these were
	// already set by something else". Nothing in this harness writes either field: in this package
	// cc.lastActive and cc.lastIdle are written only by awaitOpenSlotForRequest, which no request
	// reaches here, and by streamByID — which is the code under test.
	cc.lastActive = time.Time{}
	cc.lastIdle = time.Time{}
	cc.mu.Unlock()

	// Park a goroutine exactly where awaitOpenSlotForRequest parks a request that is waiting for one
	// of the connection's concurrency slots: in cc.cond.Wait(), under cc.mu, looping until the
	// stream map drains. Only cc.cond.Broadcast() can release it.
	//
	// The handoff is race-free rather than hopeful: `parked` is closed while this goroutine still
	// holds cc.mu, so cancelStream() — whose first act is cc.mu.Lock() — cannot run until
	// cc.cond.Wait() has atomically released the mutex and parked.
	parked := make(chan struct{})
	waiterDone := make(chan bool, 1) // true if released by cc.closed rather than by an empty map
	go func() {
		cc.mu.Lock()
		close(parked)
		for len(cc.streams) != 0 && !cc.closed {
			cc.cond.Wait()
		}
		freedByClose := cc.closed
		cc.mu.Unlock()
		waiterDone <- freedByClose
	}()
	<-parked

	cs.cancelStream()

	// Read the map, cs.done and the idle bookkeeping BEFORE Close(), which tears down every stream
	// on the connection — closing cs.done and broadcasting on cc.cond itself — and would erase every
	// difference this is measuring.
	cc.mu.Lock()
	_, stillInMap := cc.streams[cs.ID]
	lastActiveSet := !cc.lastActive.IsZero()
	lastIdleSet := !cc.lastIdle.IsZero()
	cc.mu.Unlock()
	doneClosed := false
	select {
	case <-cs.done:
		doneClosed = true
	default:
	}
	out := cancelOutcome{
		stillInMap:    stillInMap,
		doneClosed:    doneClosed,
		lastActiveSet: lastActiveSet,
		lastIdleSet:   lastIdleSet,
	}

	// Wait for the wake and the idle-timer fire TOGETHER against ONE deadline, not one after the
	// other: the didReset=true case is proving that neither happens, and two sequential deadlines
	// would make this harness cost twice what the proof needs. Each channel is nil'd once it has
	// been read, because a closed channel is selectable for ever.
	wake, idle, waiterCollected := waiterDone, idleFired, false
	deadline := time.After(waiterWakeDeadline)
	for wake != nil || idle != nil {
		select {
		case freedByClose := <-wake:
			out.wokeCondWaiter = !freedByClose
			out.waiterFreedByClose = freedByClose
			waiterCollected, wake = true, nil
		case <-idle:
			out.idleTimerRearmed = true
			idle = nil
		case <-deadline:
			// Whatever is left never happened. A waiter still parked here is left for cc.Close()
			// below to free, which it does via closeForError's own cc.cond.Broadcast().
			wake, idle = nil, nil
		}
	}

	// Close the connection so the server's ReadFrame loop ends and reports what it saw.
	_ = cc.Close()
	// Collect the waiter if it was still parked, so the goroutine does not outlive the test.
	if !waiterCollected {
		select {
		case <-waiterDone:
		case <-time.After(waiterWakeDeadline):
			t.Error("the goroutine parked in cc.cond.Wait() was not released even by cc.Close(), " +
				"which broadcasts unconditionally in closeForError — the harness is leaking a " +
				"goroutine and its results cannot be trusted")
		}
	}
	select {
	case got := <-resets:
		if got.n < 0 {
			t.Fatal("server side failed")
		}
		out.resets = got.n
		out.resetStreams = got.streams
		out.resetCodes = got.codes
		return out
	case <-time.After(10 * time.Second):
		t.Fatal("server never reported")
	}
	return out
}

func readFull(c net.Conn, b []byte) (int, error) {
	got := 0
	for got < len(b) {
		n, err := c.Read(b[got:])
		got += n
		if err != nil {
			return got, err
		}
	}
	return got, nil
}

func TestCancelStreamResetsOnlyWhenNotAlreadyReset(t *testing.T) {
	// Not yet reset: cancelStream owes the peer exactly one RST_STREAM.
	if got := countResetsAfterCancel(t, false); got.resets != 1 {
		t.Errorf("didReset=false: peer received %d RST_STREAM, want 1 — a cancelled stream that has "+
			"not been reset must be reset once", got.resets)
	}
	// Already reset: cancelStream must send NOTHING. Chrome never sends two resets on one stream.
	if got := countResetsAfterCancel(t, true); got.resets != 0 {
		t.Errorf("didReset=true: peer received %d RST_STREAM, want 0 — the stream was already reset, "+
			"so this is the duplicate Chrome never sends (measured 6/6 streams, one reset each)", got.resets)
	}
}

// TestCancelStreamResetsTheRightStreamWithCANCEL pins the OPERANDS of the reset, not its arrival.
//
// `cc.writeStreamReset(cs.ID, ErrCodeCancel, nil)` carries a stream number and an error code, and a
// harness that counts frames pins neither. Both were mutated by an adversarial reader and both were
// green: `cs.ID+2` reset a stream the peer never opened, and `ErrCodeInternal` told the origin the
// client had failed rather than that the caller had cancelled. PATCHES.md publishes the sequence
// measured against Chrome 153 as `RST_STREAM(N, CANCEL)` on the SAME stream N that the
// WINDOW_UPDATE brackets, so N and CANCEL are part of what this patch claims about the wire.
func TestCancelStreamResetsTheRightStreamWithCANCEL(t *testing.T) {
	got := countResetsAfterCancel(t, false)
	if len(got.resetStreams) != 1 || len(got.resetCodes) != 1 {
		t.Fatalf("didReset=false: peer received %d RST_STREAM (streams %v, codes %v), want exactly 1 — "+
			"the operand assertions below have nothing to read otherwise",
			got.resets, got.resetStreams, got.resetCodes)
	}
	if got.resetStreams[0] != cancelHarnessStreamID {
		t.Errorf("didReset=false: the RST_STREAM the peer received was for stream %d, want %d. "+
			"cancelStream() must reset the stream it was called on: a reset carrying any other stream "+
			"number leaves the cancelled stream open on the origin AND resets a stream the client never "+
			"cancelled, and the frame count is identical either way.",
			got.resetStreams[0], cancelHarnessStreamID)
	}
	if got.resetCodes[0] != ErrCodeCancel {
		t.Errorf("didReset=false: the RST_STREAM the peer received carried error code %v, want %v. "+
			"PATCHES.md publishes the Chrome 153 sequence as RST_STREAM(N, CANCEL); CANCEL says the "+
			"caller went away, and any other code — INTERNAL_ERROR above all — tells the origin the "+
			"client malfunctioned, which is a wire divergence a frame count cannot see.",
			got.resetCodes[0], ErrCodeCancel)
	}
}

// TestCancelStreamForgetsTheStream pins the OTHER statement inside that `if`.
//
// Patch 11 originally claimed in this file's PATCHES.md entry, and in the comment on the condition
// itself, that "no stream-map leak resulted from the old code — Close() calls forgetStreamID
// unconditionally". That is true only of the path where the caller closes the response body. On the
// path where the caller cancels the request CONTEXT with the body still open, Body.Close() is never
// called, and cancelStream() is the only code that can release the stream: with the condition
// inverted it wrote no reset AND called no forgetStreamID, so the clientStream stayed in cc.streams
// for the life of the connection, holding its accounting and one of the connection's concurrency
// slots.
//
// Measured in Sightglass through its shipped entry point (parity.TestParityHTTP2CancelledStreamsDoNotConsumeSlots):
// with the condition inverted, 100 context-cancelled requests on one connection filled
// ChromeInitialMaxConcurrentStreams = 100 and request 101 could not be sent on it at all. The claim
// is corrected in PATCHES.md and in the comment, and this test is what keeps the correction honest.
func TestCancelStreamForgetsTheStream(t *testing.T) {
	// The cancel-before-any-reset case: cancelStream() is the only writer, so it owes the peer the
	// reset AND owes the connection the slot.
	got := countResetsAfterCancel(t, false)
	if got.stillInMap {
		t.Errorf("didReset=false: the clientStream was still in cc.streams after cancelStream() — " +
			"cc.forgetStreamID sits inside the same `if` as the reset, so an inverted condition " +
			"leaks one clientStream and one concurrency slot per cancelled request, for the life " +
			"of the connection")
	}
	// The slot is only one of the four things forgetStreamID owes. It also closes cs.done, which is
	// how checkResetOrDone learns the stream is over. Deleting the map entry by hand satisfies the
	// check above and the Sightglass slot-count parity guard, and still parks every waiter for the
	// life of the connection — so this fork asserts the signal, not only the accounting.
	if !got.doneClosed {
		t.Errorf("didReset=false: cs.done was still OPEN after cancelStream() returned. Releasing the " +
			"stream means cc.forgetStreamID, which closes cs.done and broadcasts on cc.cond as well as " +
			"deleting the map entry; anything that only deletes the map entry frees the concurrency slot " +
			"and leaves checkResetOrDone and awaitFlowControl blocked forever on a stream that is gone. " +
			"A wake-up leak is the same class of defect as the slot leak patch 11 exists to close.")
	}
}

// TestCancelStreamWakesTheSlotWaiter pins effects (3) and (4) of cc.forgetStreamID: the broadcast
// that releases a request waiting for a concurrency slot, and the idle bookkeeping that lets the
// connection be reaped.
//
// It exists because TestCancelStreamForgetsTheStream and the Sightglass parity guard were BOTH
// defeated by a hand-rolled partial that deleted the map entry and closed cs.done and did nothing
// else. Freeing the slot in the map while never announcing it is not freeing the slot: the only code
// that ever observes a slot becoming free is `awaitOpenSlotForRequest`, parked in `cc.cond.Wait()`,
// and it is woken by `cc.cond.Broadcast()` alone. A request parked there when the connection is
// saturated at Chrome's 100 streams is then wedged for the life of the connection — the exact
// permanent wedge patch 11 exists to close, arrived at from the waiter's side.
//
// The negative half matters as much as the positive: with didReset=true, cancelStream() releases
// nothing, so the waiter must STILL be parked when the deadline expires. Without that, a harness
// that woke the waiter for any reason at all would look like a guard.
func TestCancelStreamWakesTheSlotWaiter(t *testing.T) {
	got := countResetsAfterCancel(t, false)
	if got.waiterFreedByClose {
		t.Fatal("didReset=false: the parked goroutine was released by cc.closed, not by an empty stream " +
			"map, so the connection died during the measurement and this run proves nothing about " +
			"cancelStream(). Investigate the harness before trusting any other result in this file.")
	}
	if !got.wokeCondWaiter {
		t.Errorf("didReset=false: a goroutine parked in cc.cond.Wait() waiting for a concurrency slot "+
			"was NEVER WOKEN within %v after cancelStream() released the stream. cc.forgetStreamID ends "+
			"in cc.cond.Broadcast(), and that broadcast is the ONLY thing that wakes "+
			"awaitOpenSlotForRequest. Deleting the stream from cc.streams without it frees the slot in "+
			"the accounting and tells nobody: on a connection saturated at Chrome's 100 concurrent "+
			"streams, the request already waiting for a slot is never woken when a context-cancelled "+
			"stream frees one, and that connection is wedged for that request for the rest of its life. "+
			"That is the same permanent wedge patch 11 exists to close.", waiterWakeDeadline)
	}
	if !got.lastActiveSet {
		t.Error("didReset=false: cc.lastActive was still the zero time after cancelStream(). " +
			"cc.forgetStreamID sets it, and ClientConn's idle accounting (tooIdleLocked, " +
			"http2ClientConnPool reuse, httptrace's GotConn.IdleTime) reads it. A connection whose " +
			"last activity is recorded as the zero time is indistinguishable from one that was never " +
			"used.")
	}
	if !got.lastIdleSet {
		t.Error("didReset=false: cc.lastIdle was still the zero time after cancelStream() emptied " +
			"cc.streams. tooIdleLocked() returns false while lastIdle is zero, so a connection whose " +
			"last stream was CANCELLED is never judged too idle and is handed to new requests for " +
			"ever.")
	}
	if !got.idleTimerRearmed {
		t.Errorf("didReset=false: cc.idleTimer never fired after cancelStream() emptied cc.streams, "+
			"although the harness armed it with a %v idleTimeout and waited %v. cc.forgetStreamID "+
			"calls cc.idleTimer.Reset(cc.idleTimeout) when the last stream goes; without it the "+
			"connection that just became idle by CANCELLATION never schedules its own reaping and "+
			"sits in the pool for ever. That is a second leak alongside the slot leak, on the same "+
			"one-line path.", 25*time.Millisecond, waiterWakeDeadline)
	}

	// Negative: didReset=true means cancelStream() owes nothing — no reset, no release, no wake.
	// A harness whose waiter wakes here is not measuring the broadcast.
	quiet := countResetsAfterCancel(t, true)
	if quiet.wokeCondWaiter {
		t.Errorf("didReset=true: the goroutine parked in cc.cond.Wait() woke anyway within %v, with the "+
			"stream still in cc.streams. cancelStream() releases nothing on this branch, so the wake "+
			"came from somewhere else and the positive assertion above is measuring ambient noise "+
			"rather than cc.forgetStreamID's broadcast.", waiterWakeDeadline)
	}
	if quiet.idleTimerRearmed {
		t.Error("didReset=true: cc.idleTimer fired although cancelStream() released no stream, so the " +
			"idle-timer assertion above is not measuring cc.forgetStreamID either.")
	}
}
