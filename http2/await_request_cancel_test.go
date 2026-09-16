package http2

// awaitRequestCancel MUST NOT REPORT A CANCELLATION FOR A STREAM THAT HAS ALREADY FINISHED.
//
// The whole reason this matters is one line in net/http's own Client: setRequestCancel builds
// stopTimer as `close(stopTimerCh); cancelCtx()`, and stopTimer runs when the response body reaches
// EOF or is Closed. A request with a Client.Timeout therefore cancels its own context as a NORMAL
// part of completing, so by the time this goroutine is scheduled, ctx.Done() and done are both
// closed. A Go select picks uniformly at random among ready cases, so roughly half of those returned
// ctx.Err() — and the only caller turns any non-nil error into cancelStream(), which since
// v0.6.9-sightglass.9 correctly writes a RST_STREAM. The premise was wrong, not the reset: the
// stream had ended. Measured through Sightglass at 4 of 12 three-leg runs, always on the COMPLETED
// GET (stream 1) or the COMPLETED POST (stream 3).
//
// This test does NOT drive a request, for the same reason cancel_stream_reset_test.go does not: the
// defect is a 50/50 schedule race, and a 50% assertion is a coin flip, not a guard. Both channels
// are closed BEFORE the call, which is the state the race produces, and the answer must be the same
// on every run — including the 200-iteration loop below, which is what actually exercises the
// randomness in select.

import (
	"context"
	"testing"
	"time"

	http "github.com/Berserk-Automation-Hub/fhttp"
)

// awaitCancelReq builds a request carrying ctx, the way clientStream.awaitRequestCancel sees one.
func awaitCancelReq(t *testing.T, ctx context.Context) *http.Request {
	t.Helper()
	req, err := http.NewRequest("GET", "https://example.invalid/", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	return req.WithContext(ctx)
}

func TestAwaitRequestCancelPrefersAFinishedStream(t *testing.T) {
	// BOTH READY. This is the state a completed, timed request leaves behind: the read loop closed
	// done when END_STREAM arrived, and the Client cancelled the context when the body drained.
	// 200 iterations because one is meaningless against a uniform-random select — an unpatched
	// awaitRequestCancel returns an error on ~100 of these.
	const iters = 200
	cancelled := 0
	for i := 0; i < iters; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		done := make(chan struct{})
		close(done)
		if err := awaitRequestCancel(awaitCancelReq(t, ctx), done); err != nil {
			cancelled++
		}
	}
	if cancelled != 0 {
		t.Errorf("awaitRequestCancel reported a cancellation on %d of %d calls where the stream was "+
			"ALREADY DONE; it must report none. Every one of those becomes a cancelStream() and an "+
			"RST_STREAM(CANCEL) on a stream that completed normally — a frame no browser sends, and "+
			"the reason Chrome 153's 6 resets across both captures are all on abandoned streams",
			cancelled, iters)
	}

	// A REAL CANCELLATION, done still open: the error must still come back, or the fix has simply
	// disabled cancellation and nothing would ever be reset again.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := awaitRequestCancel(awaitCancelReq(t, ctx), make(chan struct{})); err == nil {
		t.Error("awaitRequestCancel returned nil for a cancelled context on a stream that is NOT " +
			"done; the caller then never resets the stream and the peer is left holding it")
	}

	// A DEADLINE that expires while the stream is still open must also come back as an error.
	dctx, dcancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer dcancel()
	if err := awaitRequestCancel(awaitCancelReq(t, dctx), make(chan struct{})); err == nil {
		t.Error("awaitRequestCancel returned nil for an EXPIRED deadline on a stream that is not done")
	}

	// A stream that finishes with a live context: nil, as it always was.
	lctx, lcancel := context.WithCancel(context.Background())
	defer lcancel()
	ldone := make(chan struct{})
	close(ldone)
	if err := awaitRequestCancel(awaitCancelReq(t, lctx), ldone); err != nil {
		t.Errorf("awaitRequestCancel returned %v for a finished stream with a live context, want nil", err)
	}
}
