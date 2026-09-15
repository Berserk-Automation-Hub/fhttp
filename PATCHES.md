# fhttp vendor + Chrome-parity patches (per-request HTTP/2 priority, connection-window replenish)

`github.com/bogdanfinn/fhttp` v0.6.8, vendored here and wired in with
`replace github.com/bogdanfinn/fhttp => ./third_party/fhttp` (go/go.mod), exactly as
`third_party/quic-go-utls` already is. **One HTTP stack, patched in one place**: tls-client, tlsemu,
sightglass, quich3 and quic-go-utls all resolve to this copy, so there is no second fhttp to drift
against.

Upstream tree copied verbatim, then stripped of `*_test.go`, `export_test.go` and `testdata/`
(same treatment as the quic-go-utls vendor: 0 test files). Nothing else was removed.

## Why the patch exists

Upstream carries the HTTP/2 HEADERS-embedded PRIORITY on the **transport**:

```go
// http2/transport.go:94
HeaderPriority  *PriorityParam
```

and dereferences it inside `ClientConn.writeHeaders`, which runs on the connection's write path under
`cc.wmu`. Two consequences:

1. one `*Transport` emits **one** weight for **every** stream it opens;
2. mutating the shared `*PriorityParam` between requests is a **data race**, so there is no
   "set it before each `Do()`" workaround.

Genuine Chrome 152 does the opposite — it derives each stream's weight from **that request's own**
RFC-9218 urgency and multiplexes mixed urgencies on **one** connection. Ground truth
(`go/tlsemu/testdata/chrome152_h2_priority.json`, NetLog HEADERS-priority joined per stream with that
stream's `priority:` header; 12/12 observations match):

```
accounts.google.com   stream 1   weight 110   u=4      POST /ListAccounts       (fetch)
accounts.google.com   stream 3   weight 256   u=0      GET  /v3/signin/identifier (navigate)
accounts.google.com   stream 5   weight 256   u=0      GET  /_/bscframe
                      ^^^ three streams, ONE connection, TWO different weights
```

So before this patch a Sightglass caller that mixed urgencies had to open one client — hence one
connection — per urgency class, which made the **connection count itself** diverge from Chrome
(matrix rows B-1 and B-2).

## Patch 1 — per-request HEADERS priority

### The diff (upstream files touched: `http2/transport.go`, 2 call sites + 1 signature)

```
http2/transport.go
  - func (cc *ClientConn) writeHeaders(streamID uint32, endStream bool, maxFrameSize int, hdrs []byte) error {
  + func (cc *ClientConn) writeHeaders(streamID uint32, endStream bool, maxFrameSize int, hdrs []byte, prio *PriorityParam) error {

    …inside, after the existing `if cc.t.HeaderPriority != nil { … }`:
  +         if prio != nil {
  +             defaultHeaderPriorityParam = *prio
  +         }

  - werr := cc.writeHeaders(cs.ID, endStream, int(cc.maxFrameSize), hdrs)                 // roundTrip
  + werr := cc.writeHeaders(cs.ID, endStream, int(cc.maxFrameSize), hdrs, requestHeaderPriority(req))

  - err = cc.writeHeaders(cs.ID, true, maxFrameSize, trls)                                // trailers
  + err = cc.writeHeaders(cs.ID, true, maxFrameSize, trls, nil)
```

Everything else is the **new** file `http2/priority_perrequest.go` (`WithRequestPriority`,
`RequestPriority`, `requestHeaderPriority`).

**Backwards compatible by construction.** `prio == nil` reproduces upstream byte-for-byte:
`Transport.HeaderPriority` if set, else fhttp's `{Exclusive:true, Weight:255, StreamDep:0}` default. A
caller that sets no per-request priority sees no change, which is why every pre-existing parity guard
(`TestParityHTTP2`, `TestParityHTTP2PriorityWeightPerUrgency`, `TestParityHTTP2WireFrames`,
`TestParityHTTP2CookieCrumbling`, …) still passes untouched.

## Why a context value and not a header sentinel

fhttp's own ordering hooks (`HeaderOrderKey`, `PHeaderOrderKey`) are magic header keys that must be
filtered out again in `encodeHeaders`. A priority sentinel of that shape that ever failed to filter
would put a **novel header on the wire** — strictly worse than the bug being fixed. A
`context.Context` value cannot reach the wire at all.

## How it is driven (and why it cannot drift)

`tlsemu.BindChromeHeaderPriority(req)` derives the frame **from the request's own `priority:`
header**:

```go
req = …with `priority: u=0` in its ordered headers…
req, err = tlsemu.BindChromeHeaderPriority(req)   // -> HEADERS weight 256, exclusive, dep 0
```

- no `priority:` header ⇒ the request is returned unchanged and the transport-level priority still
  governs (we must not invent an urgency for a request that signals none);
- a malformed/out-of-range header **loud-fails** (HR-6) instead of defaulting, because a silently
  defaulted priority is exactly the header/frame mismatch this machinery exists to prevent.

## Proof (HR-7)

```
$ cd go && /Users/rentamac/go-sdk/go/bin/go test ./parity/ -run 'TestParityHTTP2MixedUrgency|TestBindChromeHeaderPriority' -v
=== RUN   TestParityHTTP2MixedUrgencyOnOneConnection
    mixed urgency on ONE connection (127.0.0.1:62816): stream 1 `u=4, i` -> weight 110,
    stream 3 `u=0, i` -> weight 256 (Chrome 152 accounts.google.com: 110 then 256)
--- PASS: TestParityHTTP2MixedUrgencyOnOneConnection (0.80s)
--- PASS: TestBindChromeHeaderPriorityCannotDriftFromTheHeader (0.00s)
```

**Ablation (the guard is falsifiable, not self-fulfilling).** With the three added lines disabled
(`if false && prio != nil`), i.e. exactly upstream behaviour, the same guard fails:

```
--- FAIL: TestParityHTTP2MixedUrgencyOnOneConnection (0.80s)
    stream 1 carried `priority: u=4, i` but HEADERS weight 256; genuine Chrome 152 stamps 110
    (this is the per-transport-priority divergence)
```

and passes again the moment the patch is restored.

---

## Patch 2 — connection-level WINDOW_UPDATE replenishes to the FULL window (matrix A-15)

`cc.inflow` is seeded at `http2/transport.go:879` with `cc.connFlow + initialWindowSize` — for a
Chrome-152 profile `15663105 + 65535 = 15728640`, which is exactly Chrome's
`session_max_recv_window_size_`. Upstream then **topped it back up to `cc.connFlow`** on every refill:

```
  - if v := cc.inflow.available(); v < int32(cc.connFlow/2) {
  -     connAdd = int32(cc.connFlow) - v
  + connWindow := int32(cc.connFlow) + int32(initialWindowSize)
  + if v := cc.inflow.available(); v < connWindow/2 {
  +     connAdd = connWindow - v
        cc.inflow.add(connAdd)
    }
```

Chrome (`net/spdy/spdy_session.cc SpdySession::IncreaseRecvWindowSize`):

```cpp
session_unacked_recv_window_bytes_ += delta_window_size;
if (session_unacked_recv_window_bytes_ > session_max_recv_window_size_ / 2 || elapsed >= …) {
  SendWindowUpdateFrame(kSessionFlowControlStreamId, session_unacked_recv_window_bytes_, HIGHEST);
  session_unacked_recv_window_bytes_ = 0;   // window is back at session_max_recv_window_size_
}
```

so both the **trigger** (half the FULL window) and the **amount** (the whole accumulated unacked
count) were off: the client's connection window sat permanently 65535 bytes below Chrome's, and at
the exact point Chrome emits its first refill the upstream client emitted **none at all**.

### Proof (HR-7)

```
$ go test ./parity/ -run TestParityHTTP2ConnectionWindowReplenish -v
    connection window: preface +15663105 (=> 15728640), refill +7864321 after 7864321 bytes
    => back to 15728640 (Chrome's session_max_recv_window_size_)
--- PASS (0.01s)
```

**Ablation** (`connWindow := int32(cc.connFlow)`, i.e. upstream):

```
--- FAIL: TestParityHTTP2ConnectionWindowReplenish (45.01s)
    server: timeout: sent 7864321/7864321 body bytes, 0 conn WINDOW_UPDATEs
```

---

## Patch 3 — Chrome's HTTP/2 stream-concurrency limits (matrix A-9)

The parity matrix listed A-9 as `unverifiable` ("needs the Chrome constant from source/NetLog"). It
is settled from the oracle's own Chromium 152 tree, and once settled it is a **three-way**
divergence, all of it server-visible (the origin simply counts the streams the client opens):

| | upstream fhttp v0.6.8 | Chrome 152 | source |
|---|---|---|---|
| initial (pre-SETTINGS) limit | `1000  // "infinite", per spec` | **100** | `net/spdy/spdy_session.h:84` `kInitialMaxConcurrentStreams`, applied `spdy_session.cc:837` |
| ceiling on the peer's advertised value | none — `cc.maxConcurrentStreams = s.Val` | **256** | `net/spdy/spdy_session.cc:383` `kMaxConcurrentStreamLimit`, applied `:2356-2357` |
| streams actually opened at the limit | `max - 1` (`len+1 < max`) | exactly `max` | `net/spdy/spdy_session.cc:1697-1699` `if (active_streams_.size() + created_streams_.size() < max_concurrent_streams_) return CreateStream(...)` |

### The diff (upstream file touched: `http2/transport.go`, 3 lines)

```
http2/transport.go:766
  -   maxConcurrentStreams:  1000,               // "infinite", per spec. 1000 seems good enough.
  +   maxConcurrentStreams:  ChromeInitialMaxConcurrentStreams,

http2/transport.go:967   (idleStateLocked, the non-StrictMaxConcurrentStreams branch)
  -   maxConcurrentOkay = int64(len(cc.streams)+1) <  int64(cc.maxConcurrentStreams)
  +   maxConcurrentOkay = int64(len(cc.streams)+1) <= int64(cc.maxConcurrentStreams)

http2/transport.go:2743  (SETTINGS handler)
  -   cc.maxConcurrentStreams = s.Val
  +   cc.maxConcurrentStreams = chromeClampMaxConcurrentStreams(s.Val)
```

Everything else is the **new** file `http2/chrome_concurrency.go` (the two constants + the clamp).

### Proof (HR-7)

```
$ cd go && /Users/rentamac/go-sdk/go/bin/go test ./parity/ -run TestParityHTTP2Concurrency -v
--- PASS: TestParityHTTP2ConcurrencyConstantsMatchChromium (0.00s)
--- PASS: TestParityHTTP2ConcurrencyClampsPeerAdvertisedLimit (6.70s)
    peer advertised 1000, client opened 256 concurrent streams (Chrome's kMaxConcurrentStreamLimit)
--- PASS: TestParityHTTP2ConcurrencyInitialLimitBeforeSettings (6.71s)
    peer advertised nothing, client opened 100 concurrent streams (Chrome's kInitialMaxConcurrentStreams)
```

**Ablation (the guard is falsifiable, not self-fulfilling).** With all three hunks reverted to
upstream, the same loopback servers see the client open **300** and **150** streams:

```
--- FAIL: TestParityHTTP2ConcurrencyClampsPeerAdvertisedLimit
    client opened 300 concurrent streams against a peer advertising 1000; Chrome clamps at
    kMaxConcurrentStreamLimit = 256 (net/spdy/spdy_session.cc:2356)
--- FAIL: TestParityHTTP2ConcurrencyInitialLimitBeforeSettings
    with no peer MAX_CONCURRENT_STREAMS the client opened 150 concurrent streams; Chrome's
    kInitialMaxConcurrentStreams is 100 (net/spdy/spdy_session.h:84)
```

and both pass again the moment the patch is restored.

---

## Patch 4 — detach the H1 dial from the request's cancellation (abandoned-dial churn, golang.org/issue/32406)

This fhttp fork predates Go's upstream `getCtxForDial`/`WithoutCancel` fix. A single `getConn` races an
idle-conn lookup against a fresh dial, and the losing dial's context **was the request's context**
(`wantConn.ctx = req.Context()`). When a pooled/idle conn won the race and the request then completed
(or was cancelled), the request context was cancelled and the losing dial — already mid-TCP-connect or
mid-TLS-handshake in `utls.(*UConn).handshakeContext` — was torn down and the socket abandoned. The
client's own error rate stayed **0.00%**, so the waste was invisible from the client side, but every
abandoned dial cost a local `socket()+connect()+(partial)ClientHello` and a wasted
`accept()+goroutine+failed-handshake` on the origin.

### The diff (upstream file touched: `transport.go`, `wantConn` + 2 call sites)

```
transport.go  type wantConn struct {
  +   cancelCtx context.CancelFunc   // releases ctx once the dial goroutine is done

transport.go  getConn(...)
  -   ctx:        ctx,                                                     // == req.Context()
  +   dialCtx, dialCancel := context.WithCancel(context.WithoutCancel(ctx))
  +   ctx:        dialCtx,
  +   cancelCtx:  dialCancel,

transport.go  dialConnFor(w *wantConn)
  +   defer w.cancelCtx()            // release the detached context when the dial goroutine ends
```

`context.WithoutCancel(ctx)` keeps the request's **values** (httptrace, etc.) but drops its
cancellation + deadline, so a dial that loses the idle-vs-dial race **runs to completion and joins the
idle pool** for a future request instead of being abandoned. Each request remains bounded by
`Client.Timeout` (getConn still selects on `req.Context().Done()`) and each handshake by the
ctx-independent 10s `TLSHandshakeTimeout` timer — so no user-visible timeout changes, and
`sightglass.TestStressNoLeaks` confirms goroutines/conns return to baseline.

### Proof (HR-7)

```
$ cd go && go build -tags loadlab -o /tmp/dialprobe ./loadlab/cmd/zzh1dialprobe && /tmp/dialprobe -conc 6 -secs 3
PRE-FIX   shipped-ipv4  sent=50778   srv_hs_ok=1  srv_hs_ERR=11214   (11034 zero-byte abandoned + mid-handshake)
POST-FIX  shipped-ipv4  sent=164310  srv_hs_ok=6  srv_hs_ERR=0       (pool warms to MaxConnsPerHost=6; 3x throughput)

$ go test ./parity/ -run TestParityNoAbandonedH1Dials -v
    sent=24000 cli_err=0 | srv_hs_ok=6 srv_hs_ERR=0
--- PASS: TestParityNoAbandonedH1Dials (0.86s)
```

**Ablation (the guard is falsifiable, not self-fulfilling).** With the detach reverted to upstream
(`context.WithCancel(ctx)`), the same committed guard fails:

```
--- FAIL: TestParityNoAbandonedH1Dials
    sent=24000 cli_err=0 | srv_hs_ok=1 srv_hs_ERR=5331
      aborted handshake 5236x  EOF | bytes_read=0              (dial abandoned before ClientHello)
      aborted handshake   30x  EOF | bytes_read=1951           (Chrome-152 ClientHello written, then torn down)
    origin saw 5331 ABANDONED/aborted handshakes — the H1 dial is being torn down by the request context
```

and passes again the moment the patch is restored.

## Patch 5 — the H2 connect honours the request's context (uncancellable-dial hang)

Same defect class as patch 4, on the **HTTP/2** side, and worse: this fork carries the pre-2021
x/net/http2 pool, where the dial takes **no context at all** —

```
fork          func (t *Transport) dialClientConn(addr string, singleUse bool) (*ClientConn, error)
x/net@v0.48.0 func (t *Transport) dialClientConn(ctx context.Context, addr string, singleUse bool) (*ClientConn, error)
```

— and `clientConnPool.getClientConn` then blocked on `<-call.done` with no select on the request's
context. The shipped stack makes that unbounded in practice: tls-client v1.15.1 hands
`http2.Transport` a **legacy, context-free `DialTLS` hook** that dials with `context.Background()`
(`roundtripper.go:483 dialTLSHTTP2 -> rt.dialTLS(context.Background(), …)`), so the TLS handshake of an
H2 re-dial can never be cancelled. Once a pooled H2 connection dies and the origin stops completing
handshakes (blackhole, half-open middlebox, overloaded LB), **every** request to that authority parks
in `GetClientConn` forever — past its own deadline, past `Client.Timeout` — each holding a goroutine
and a socket. That is the "hung goroutine / fd growth under load" shape the scale phase hunts.

### The diff (upstream file touched: `http2/client_conn_pool.go`, 2 call sites)

```
http2/client_conn_pool.go  getClientConn(...)  shared-dial wait
  -   <-call.done
  -   return call.res, call.err
  +   select {
  +   case <-call.done:
  +       return call.res, call.err
  +   case <-req.Context().Done():
  +       return nil, req.Context().Err()
  +   }

http2/client_conn_pool.go  getClientConn(...)  isConnectionCloseRequest single-use branch
  -   cc, err := p.t.dialClientConn(addr, singleUse)     // synchronous, uncancellable
  +   … the same dial off the caller's goroutine, selected against req.Context(); an ABANDONED
  +   single-use conn is CLOSED (it joins no pool, so nothing else would ever reap it)
```

The dial goroutine is deliberately **not** abandoned (it has no context to cancel, and patch 4
established the rule): it completes and its connection joins the pool for a later request, so no socket
is wasted. Nothing on the wire changes — pure control flow, every byte-parity guard stays green.

### Proof (HR-7)

`parity.TestParityH2DialHonoursRequestContext` drives the SHIPPED client (`tlsemu.BuildClient`,
`Timeout = 2s`) against a loopback origin that serves exactly one h2 connection and then STALLS every
later one before the TLS handshake:

```
PRE-FIX   --- FAIL: TestParityH2DialHonoursRequestContext (20.02s)
              HUNG: the second request never returned — the H2 pool is waiting on an uncancellable dial

POST-FIX  --- PASS: TestParityH2DialHonoursRequestContext (2.03s)
              second request honoured its 2s deadline in 2.001019833s: context deadline exceeded
```

## Maintenance

On an fhttp upgrade: re-copy upstream, strip tests, re-apply the hunks above (four for patch 1, one for
patch 2, three for patch 3, three for patch 4, two for patch 5), keep `http2/priority_perrequest.go` and
`http2/chrome_concurrency.go`, and re-run
`go test ./parity/ -run 'TestParityHTTP2|TestParityNoAbandonedH1Dials|TestParityH2DialHonoursRequestContext'`
(all of it, not just the new guards).


---

## Patch 4b — bound the proxy CONNECT unconditionally (and the tests patch 4 invalidated)

**Found by restoring upstream's test suite**, which the vendored copy had stripped.
`TestTransportProxyHTTPSConnectLeak` hung for the full 5-minute test timeout.

**The bug patch 4 introduced.** This tree carried Go's OLD CONNECT logic:

```go
connectCtx := ctx
if ctx.Done() == nil {            // only when the caller's ctx can NEVER be cancelled
    newCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
    connectCtx = newCtx
}
```

i.e. the 1-minute leak bound applied *only* when the context had no `Done` channel, and request
cancellation was relied on in every other case. Patch 4 detaches the dial from request cancellation,
so the context arriving here always HAS a `Done` channel (it comes from `context.WithCancel`) but is
never cancelled by the request finishing. **The condition was therefore false in exactly the case the
safety net existed for**: a proxy that accepts the TCP connect and then never answers leaked the
goroutine and the socket indefinitely.

**The fix** is upstream Go's, verbatim in shape — make the bound unconditional:

```go
connectCtx, cancel := testHookProxyConnectTimeout(ctx, 1*time.Minute)
defer cancel()
```

`net/http/transport.go` does exactly this, with the comment *"Set a (long) timeout here to make sure
we don't block forever and leak a goroutine if the connection stops replying after the TCP connect."*
Upstream's leak protection for CONNECT deliberately does not depend on request cancellation — which is
what makes detaching the dial safe there, and was the missing half here.

`SetTestHookProxyConnectTimeout` is added to `export_test.go` (upstream shape) so the test can drive
that bound directly instead of waiting a real minute.

### Tests patch 4 invalidated, refreshed to their current upstream versions

This fork's tests predate the upstream change that introduced `context.WithoutCancel` for the dial,
so three of them still asserted the pre-change contract. Upstream rewrote all three at the time;
these now match:

| test | asserted before | asserts now (upstream) |
|---|---|---|
| `TestTransportDialContext` | `receivedContext != ctx` — context **identity** | `ctx.Value(ctxKey)` — the context's **value**, which `WithoutCancel` preserves |
| `TestTransportDialTLSContext` | same | same |
| `TestClientPropagatesTimeoutToContext` | a deadline inside `DialContext` | a deadline on `req.Context()`, via a `testRoundTripper` |
| `TestTransportProxyHTTPSConnectLeak` | cancelling the request aborts the CONNECT | the CONNECT's own timeout aborts it |

No assertion was weakened: each still proves the property upstream intends, against the behaviour
upstream now specifies.

### Two unrelated drift fixes, needed to run the suite at all on a current toolchain

* `http2/server_test.go` formatted an `int64` with `%q`. Harmless at runtime, but `go test` runs vet
  and the newer vet rejects it, so the whole `http2` package reported `[build failed]` before running
  a single test. Now `%d`.
* `pprof/pprof.go` did not know Go 1.27's `goroutineleak` profile, so `TestDescriptions` failed. Added
  to both `profileSupportsDelta` and `profileDescriptions`, matching upstream.

### Verification

Baseline matters here: **pristine upstream v0.6.9 already fails 34 tests** under `go test -short`, so
"green" is not an available bar. The bar is a regression diff — run pristine and fork identically and
count only what fails in the fork and not in pristine:

```
fork: 34 failing   pristine: 34 failing
REGRESSIONS: none
root package: 7.993s   (was a 300s timeout before patch 4b)
gofmt: identical to upstream's baseline
```

---

## Patch: HPACK pseudo-header indexing, so the encoder emits what a browser emits

Found by a Sightglass parity guard built from decrypted Chrome 153 traffic. Two facets land on the
same octet, and both are wire-visible to anything that decodes HPACK.

**1. The static table's name index resolved to the LAST matching entry.** `addEntry` writes
`byName[name]` unconditionally, so a name appearing twice keeps the higher index: `:path` → 5
(`/index.html`) instead of 4 (`/`), `:method` → 3 (POST) instead of 2 (GET). Invisible on a
name+value hit, but emitted verbatim in every literal-with-indexed-name representation.

RFC 7541 §6.2.1 permits matching *any* entry with that name, so this was always an implementation
choice — upstream's own test comment says exactly that ("This is allowed to match any `:method`
entry. The current implementation uses the last entry added"). Real Chrome 153 uses the first, on
114 `:path` and 8 `:method` observations across two captures with zero exceptions. Only the STATIC
table is rebuilt; the dynamic table keeps most-recent-wins, because its indices shift on eviction.

**2. The encoder indexed everything that fit.** Chrome never incrementally-indexes `:path` — the
value changes every request, so indexing it evicts useful entries for nothing — and never indexes a
literal `:method`, while it *does* index `:authority`, which is stable for the connection.
`Encoder.SetIndexingPolicy` adds a per-field hook; `nil` is upstream behaviour exactly.
`Transport.HPACKIndexingPolicy` carries it, installed once per connection because HPACK is stateful
and a mid-connection change would desynchronise our table from the peer's view of it.

`Sensitive` is deliberately NOT the mechanism: it emits "Never Indexed" (`0x1x`), which carries an
explicit do-not-proxy instruction and which Chrome uses **zero** times in 2513 observed fields. It
would fix one octet and break another.

### Result

```
:path "/x"   upstream default            -> 0x44
             with Chrome's policy        -> 0x04     (what Chrome emits)
```

### Verification

Regression diff against pristine v0.6.9, run identically: **34 failing / 34 failing, no
regressions.** One upstream test was updated rather than deleted — `TestEncoderSearchTable` pinned
the last-match choice, and its own comment documented it as free. The new expectation records why it
changed.

## Patch 7 — `Content-Encoding: deflate` deadlocks readLoop against itself (leak), and never decodes raw DEFLATE (corruption)

### The deadlock

`DecompressBody` runs **inside** `persistConn.readLoop` (`transport.go`, `if rc.addedGzip`). The body
it is handed is a `*bodyEOFSignal` whose EOF path does `<-eofc`, and `eofc` is closed only when
readLoop **returns**. So any read that reaches the end of the body from inside readLoop blocks
readLoop on a channel that only readLoop can close.

`identifyDeflate` did exactly that. To replay the two octets it sniffed, it drained the **entire**
body with `io.Copy`:

```
readLoop -> DecompressBody -> identifyDeflate -> prependBytesToReadCloser -> io.Copy
         -> bodyEOFSignal.Read -> condfn -> readLoop.func4 -> <-eofc      [parked forever]
```

Reproduced, with that exact stack, against a loopback origin answering
`Content-Encoding: deflate` with a 25-byte zlib body.

**It did not look like a deadlock.** `Client.Timeout` rescues the caller, so the symptom is
`context deadline exceeded (Client.Timeout exceeded while awaiting headers)` — a slow origin. What
actually happened is that the readLoop goroutine and its socket were pinned for the life of the
process; `Transport.CloseIdleConnections` cannot free a conn whose readLoop never returns. Every
deflate response leaked one goroutine and one fd.

A **two-octet** body was enough on its own: `io.ReadFull` hitting EOF took the same `<-eofc` path
before any copy started.

### The corruption

The same function decided "is this zlib or raw DEFLATE?" by comparing the first two octets against a
list of four common CMF/FLG pairs, and **passed anything else through untouched** — while
`DecompressBody` had already deleted `Content-Encoding` and set `res.Uncompressed = true`. `deflate`
is famously two formats (RFC 1950 zlib-wrapped and RFC 1951 raw, both of which browsers accept), and
a raw stream begins with the first block's BFINAL/BTYPE bits, which are not `0x78` in general. So a
raw-DEFLATE body came back **compressed, presented as decoded**.

The fork's own `TestCompressionDeflate` — which is `testCompressionDeflate(t, /*zlibWrapped=*/false)`
— has been failing for exactly this reason, sitting unnoticed among the pre-existing upstream
failures.

### The fix

```
identifyDeflate     returns a deflateSniffer that picks the flavour on its FIRST READ; nothing is
                    read inside readLoop, which is how the gzip/br/zstd paths already behave
isZlibHeader        RFC 1950 §2.2's own test — CM == 8, CINFO <= 7, and (CMF<<8|FLG) % 31 == 0 —
                    instead of a list of four pairs. Exact, not a heuristic
                    (a raw stream passes all three only by coincidence, ~1 in 31 for the checksum)
no pass-through     a body under `Content-Encoding: deflate` is one of the two forms or it is
                    broken; a broken one now surfaces as flate's error instead of silent corruption
prependBytesTo…     replays the sniffed octets with io.MultiReader instead of draining. The draining
                    version also swallowed io.Copy's error and closed the body while returning a
                    buffer, so a truncated body read as a complete one
zlibDeflateReader.Close / deflateReader.Close
                    tolerate never having been read. Both construct their decoder on first Read and
                    Close reached straight through to it, so closing an untouched body was a
                    nil-receiver panic
```

### Tests

`transport_deflate_leak_test.go`: zlib-wrapped and raw DEFLATE both round-trip; the three
sniff-boundary shapes (empty, one octet, two non-header octets) all return; an unread body closes
without panicking; and after three requests that are abandoned without reading, **no goroutine is
parked in the decode path** — with the origin still holding its sockets, so only the fix can free
them.

Ablated by restoring the eager `identifyDeflate`: all three go red with the original symptom,
`the request never completed (… Client.Timeout exceeded while awaiting headers)`.

### Verification

```
before (v0.6.9-sightglass.2): 51 failing
after:                        50 failing
NEW failures: none
FIXED:        TestCompressionDeflate   <- the raw-DEFLATE corruption, upstream's own test
```
