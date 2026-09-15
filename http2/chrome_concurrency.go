package http2

// [SIGHTGLASS PATCH 3] Chrome's HTTP/2 stream-concurrency limits (parity matrix A-9).
//
// Upstream fhttp seeds a new ClientConn with `maxConcurrentStreams: 1000  // "infinite", per spec`
// and then adopts the peer's SETTINGS_MAX_CONCURRENT_STREAMS verbatim. Chrome does neither:
//
//	kInitialMaxConcurrentStreams = 100    net/spdy/spdy_session.h:84
//	                                      (applied in the SpdySession ctor, spdy_session.cc:837)
//	kMaxConcurrentStreamLimit    = 256    net/spdy/spdy_session.cc:383, applied at :2356-2357
//	                                      max_concurrent_streams_ =
//	                                          std::min(static_cast<size_t>(value), kMaxConcurrentStreamLimit);
//
// Both differences are visible AT THE SERVER, which is why this is a parity row and not tuning:
//
//   - before the peer's SETTINGS arrive, a client with several requests queued opens as many streams
//     as its initial limit allows. Chrome opens at most 100; upstream fhttp would open up to 1000.
//   - after SETTINGS, a server advertising MAX_CONCURRENT_STREAMS above 256 (GFE advertises 100, but
//     plenty of origins advertise more) sees Chrome cap itself at 256 and sees upstream fhttp take
//     the advertised number.
//
// The two constants live here, in the vendored dependency, because the value has to be applied where
// the ClientConn is constructed and where SETTINGS is handled — see FHTTP_LAYER_PATCH.md, patch 3.
// Upstream lines touched: exactly two (transport.go:766 and :2737).
const (
	// ChromeInitialMaxConcurrentStreams is net::kInitialMaxConcurrentStreams.
	ChromeInitialMaxConcurrentStreams uint32 = 100
	// ChromeMaxConcurrentStreamLimit is net::kMaxConcurrentStreamLimit, the ceiling Chrome clamps the
	// peer's advertised value to.
	ChromeMaxConcurrentStreamLimit uint32 = 256
)

// chromeClampMaxConcurrentStreams reproduces spdy_session.cc:2356-2357.
func chromeClampMaxConcurrentStreams(advertised uint32) uint32 {
	if advertised > ChromeMaxConcurrentStreamLimit {
		return ChromeMaxConcurrentStreamLimit
	}
	return advertised
}
