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
	"testing"
	"time"
)

// countResetsAfterCancel sets cs.didReset to the given value, calls cancelStream(), and reports how
// many RST_STREAM frames the peer actually received and whether the clientStream was still in
// cc.streams when cancelStream() returned.
//
// The second return value is not decoration. cc.forgetStreamID sits INSIDE the same `if` as the
// reset, so the inverted condition dropped the bookkeeping along with the frame — see
// TestCancelStreamForgetsTheStream.
func countResetsAfterCancel(t *testing.T, didReset bool) (int, bool) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	resets := make(chan int, 1)
	go func() {
		srv, aerr := ln.Accept()
		if aerr != nil {
			resets <- -1
			return
		}
		defer func() { _ = srv.Close() }()
		_ = srv.SetDeadline(time.Now().Add(5 * time.Second))

		// The client preface is 24 bytes that are not a frame; reading it as one desynchronises the
		// framer for everything after it.
		pre := make([]byte, len(clientPreface))
		if _, rerr := readFull(srv, pre); rerr != nil {
			resets <- -1
			return
		}
		fr := NewFramer(srv, srv)
		n := 0
		for {
			f, ferr := fr.ReadFrame()
			if ferr != nil {
				break // deadline or EOF: we have seen everything the client sent
			}
			if _, ok := f.(*RSTStreamFrame); ok {
				n++
			}
		}
		resets <- n
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

	cs := &clientStream{cc: cc, ID: 1, done: make(chan struct{})}
	cc.mu.Lock()
	if cc.streams == nil {
		cc.streams = make(map[uint32]*clientStream)
	}
	cc.streams[cs.ID] = cs
	cs.didReset = didReset
	cc.mu.Unlock()

	cs.cancelStream()

	// Read the map BEFORE Close(), which tears down every stream on the connection and would erase
	// the difference this is measuring.
	cc.mu.Lock()
	_, stillInMap := cc.streams[cs.ID]
	cc.mu.Unlock()

	// Close the connection so the server's ReadFrame loop ends and reports what it saw.
	_ = cc.Close()
	select {
	case n := <-resets:
		if n < 0 {
			t.Fatal("server side failed")
		}
		return n, stillInMap
	case <-time.After(10 * time.Second):
		t.Fatal("server never reported")
	}
	return 0, stillInMap
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
	if n, _ := countResetsAfterCancel(t, false); n != 1 {
		t.Errorf("didReset=false: peer received %d RST_STREAM, want 1 — a cancelled stream that has "+
			"not been reset must be reset once", n)
	}
	// Already reset: cancelStream must send NOTHING. Chrome never sends two resets on one stream.
	if n, _ := countResetsAfterCancel(t, true); n != 0 {
		t.Errorf("didReset=true: peer received %d RST_STREAM, want 0 — the stream was already reset, "+
			"so this is the duplicate Chrome never sends (measured 6/6 streams, one reset each)", n)
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
	if _, stillInMap := countResetsAfterCancel(t, false); stillInMap {
		t.Errorf("didReset=false: the clientStream was still in cc.streams after cancelStream() — " +
			"cc.forgetStreamID sits inside the same `if` as the reset, so an inverted condition " +
			"leaks one clientStream and one concurrency slot per cancelled request, for the life " +
			"of the connection")
	}
}
