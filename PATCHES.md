# fhttp — the Sightglass fork: what each patch changes, why, and what guards it

<!-- SIGHTGLASS-DOC-CONTRACT: the machine-checked facts below are pinned by patches_doc_test.go in
     this tree. If you change the base commit, the file manifest, a patch number or a guard name,
     that test goes red and tells you which sentence stopped being true. Do not edit around it. -->

## Base

**Upstream base: `github.com/bogdanfinn/fhttp` v0.6.9**, tag `v0.6.9` =
commit `a8b14170ef54dc2355c68ec20951a86750fb7732` ("updated tests and utls dependency"). Everything
in this file is stated relative to that commit, and `git diff a8b1417 HEAD` is the entire fork.

## It is a fork, not a vendored tree, and there is no `replace` anywhere

The module path is rewritten to `github.com/Berserk-Automation-Hub/fhttp` and Sightglass consumes it
with a plain `require` at a tag. That is binding, not incidental: `go/go.mod`'s own header and
`go/AGENTS.md` both forbid `replace`, because a directory `replace` does not propagate to dependents
— a consumer of the Sightglass module would silently get upstream fhttp instead of the build
Sightglass tests. `go/go.mod` has **zero** `replace` directives, and there is no `third_party/` tree
in this repository or in Sightglass.

The rewritten path is also what makes ONE HTTP stack possible. tls-client, websocket, quic-go-utls
and Sightglass itself all import fhttp and utls; if any of them stayed on the upstream path the
binary would contain two fhttp packages with two incompatible sets of types, and it would not
compile. HR-2 enforced by the compiler rather than by convention.

## Upstream's tests are KEPT, not stripped

This tree carries **81 `*_test.go` files: 73 from upstream v0.6.9, of which 68 carry nothing but the
module-path rewrite and 5 are edited, plus 8 added by this fork.** `export_test.go` and `testdata/`
are kept too. That is load-bearing, not tidiness: restoring upstream's suite is what found patch 4b
(a leak patch 4 introduced, which hung `TestTransportProxyHTTPSConnectLeak` for the full test
timeout) and what forced the two toolchain-drift fixes below. The five edits are listed in the
manifest, each against the patch that made it necessary.

**A correction, recorded because the tags that carried it are published.** Revisions of this file up
to and including `v0.6.9-sightglass.11` opened with three statements that were all false: that the
base was v0.6.8 (it is v0.6.9 — v0.6.8 is commit `ecfe905`, one release earlier); that the tree was
"vendored here and wired in with `replace github.com/bogdanfinn/fhttp => ./third_party/fhttp`" (there
is no such `replace` and no such directory, and the rule forbids both); and that it had been
"stripped of `*_test.go`, `export_test.go` and `testdata/` … 0 test files" (it has 81). The
`## Maintenance` section then repeated the last one as an INSTRUCTION — "re-copy upstream, strip
tests" — which is a fourth error and not a restatement of the third: one is a false description, the
other tells the next maintainer to delete the suite that catches the defects. All four are corrected
here and pinned by `patches_doc_test.go`.

## Files touched

Mechanically derived from `git diff --name-status a8b1417 HEAD`, and re-derived by
`patches_doc_test.go` on every run. 97 files in total: 11 added, 86 modified, of which 70 are
modified **only** by the module-path rewrite.

### Added (11)

```
A  PATCHES.md                              this file
A  http2/chrome_concurrency.go             patch 3   product
A  http2/priority_perrequest.go            patch 1   product
A  header_http1omit_test.go                patch 8   guard
A  http2/await_request_cancel_test.go      patch 12  guard
A  http2/cancel_stream_reset_test.go       patch 11  guard
A  http2/goaway_flush_test.go              patch 9   guard
A  http2/hpack/static_name_index_test.go   patch 10  guard
A  no_auto_headers_test.go                 patch 8b  guard
A  transport_deflate_leak_test.go          patch 7   guard
A  patches_doc_test.go                     —         guard on THIS FILE
```

Two product files and eight guards. Seven are new with the patch they guard; the eighth,
`patches_doc_test.go`, guards this document — it re-derives the base commit, the manifest above,
the patch numbers, the guard names and the test-file count from the tree, and fails naming the
sentence that stopped being true. None of them existed upstream.

### Modified, with changes of their own (16)

```
M  http2/transport.go            patches 1, 2, 3, 6, 8, 8b, 9, 10, 11, 12   (18 hunks)
M  transport.go                  patches 4, 4b, 7, 8, 8b
M  header.go                     patches 8, 8b
M  h2_bundle.go                  patches 8, 8b
M  http2/client_conn_pool.go     patch 5
M  http2/hpack/encode.go         patches 6, 10
M  http2/hpack/tables.go         patches 6, 10
M  request.go                    patch 8b
M  pprof/pprof.go                patch 4b   (Go 1.27 `goroutineleak` profile — toolchain drift)
M  client_test.go                patch 4b   (upstream test patch 4 invalidated)
M  transport_test.go             patch 4b   (upstream tests patch 4 invalidated)
M  export_test.go                patch 4b   (SetTestHookProxyConnectTimeout, upstream shape)
M  http2/server_test.go          patch 4b   (`%q` on an int64 — current vet rejects it)
M  http2/hpack/encode_test.go    patch 6    (TestEncoderSearchTable pinned the last-match choice)
M  go.mod                        module path, Go 1.27, current deps
M  go.sum                        ditto
```

### Modified by the module-path rewrite ONLY (70)

70 files whose entire diff against v0.6.9 is `bogdanfinn/fhttp` → `Berserk-Automation-Hub/fhttp` and
`bogdanfinn/utls` → `Berserk-Automation-Hub/utls`. They carry no behaviour change and are not
discussed again in this file. `patches_doc_test.go` re-derives the split, so the three counts above
cannot drift from the tree.

## Where each patch is guarded

Two layers, and the split is not uniform — saying so is the point of this section. A fork test proves
the library behaves; only a Sightglass test proves the behaviour reaches the wire the product puts on
it, through `sightglass.NewSessionFactory -> Session.Do` (or `chromeagent.Agent` above it). Five
patches have no fork-local test of their own and are guarded ONLY in the consumer; that is recorded
here rather than glossed, because a reader who assumes otherwise will delete the wrong thing on the
next upstream merge.

`patches_doc_test.go` requires every test this file names to exist either in this tree or in this
table, so a guard cannot be cited into existence.

| patch | guard in THIS fork | guard in Sightglass (shipped path) |
|---|---|---|
| 1  | — none | `parity.TestParityHTTP2MixedUrgencyOnOneConnection`, `parity.TestBindChromeHeaderPriorityCannotDriftFromTheHeader`, `parity.TestParityHTTP2PriorityWeightPerUrgency` |
| 2  | — none | `parity.TestParityHTTP2ConnectionWindowReplenish` |
| 3  | — none | `parity.TestParityHTTP2ConcurrencyConstantsMatchChromium`, `parity.TestParityHTTP2ConcurrencyClampsPeerAdvertisedLimit`, `parity.TestParityHTTP2ConcurrencyInitialLimitBeforeSettings` |
| 4  | — none | `parity.TestParityNoAbandonedH1Dials`, `sightglass.TestStressNoLeaks` |
| 4b | `TestTransportProxyHTTPSConnectLeak` (upstream's own, restored — it is what FOUND this patch) | — |
| 5  | — none | `parity.TestParityH2DialHonoursRequestContext` |
| 6  | `http2/hpack/encode_test.go` `TestEncoderSearchTable` covers the static-NAME half; the **indexing-policy** half (`Encoder.SetIndexingPolicy`, `Transport.HPACKIndexingPolicy`) has **no fork test** | `parity.TestParityHPACKEncoderMatchesChrome153`, `parity.TestParityHPACKPseudoHeaderRepresentationMatchesChrome153`, `parity.TestHPACKIndexingPolicyIsProfileDrivenNotHardcoded` |
| 7  | `transport_deflate_leak_test.go` | — (a parked goroutine and an fd have no wire signature) |
| 8  | `header_http1omit_test.go` | `parity.TestParityHTTP1Identity` (the magic key must not reach the HTTP/1.1 wire) |
| 8b | `no_auto_headers_test.go` | `parity.TestParityCallerStatedBlockIsExactOnH2` |
| 9  | `http2/goaway_flush_test.go` | — (the frame is on a connection Sightglass tears down; no parity row observes it) |
| 10 | `http2/hpack/static_name_index_test.go` | `parity.TestParityStaticNameIndexIsProfileDrivenOnTheShippedPath` |
| 11 | `http2/cancel_stream_reset_test.go` (`TestCancelStreamResetsOnlyWhenNotAlreadyReset` for the frame, `TestCancelStreamForgetsTheStream` for the slot) | `parity.TestParityHTTP2CancelledStreamsDoNotConsumeSlots` — 150 context-cancelled requests on ONE connection through `NewSessionFactory -> Session.Do` |
| 12 | `http2/await_request_cancel_test.go` | `parity.TestParityHTTP2CancelledStreamsDoNotConsumeSlots`, `parity.TestParityHTTP2WireFrames` |
| this file | `patches_doc_test.go` | — a document has no shipped path; it is guarded where it lives |
| sibling pins | — (this fork cannot see `go/go.mod`) | `parity.TestForkGoModsDoNotPinOlderSiblingForks` |

**The five with no fork test — 1, 2, 3, 4 and 5 — are the original batch**, written before this fork
kept upstream's suite, and each is a wire or dial property that a loopback origin in the consumer
observes directly. They are guarded, and ablated, in Sightglass; they are not guarded here. Patch 6's
indexing half is a genuine gap on both counts and is filed as such.

Run prefixes quoted in the `Proof (HR-7)` blocks below — `parity.TestParityHTTP2`,
`parity.TestParityHTTP2MixedUrgency`, `parity.TestParityHTTP2Concurrency` and
`parity.TestBindChromeHeaderPriority` — are `go test -run` patterns, not test names; each matches the
tests listed in the rows above.

## Patch numbering

Numbers are historical — they are the order the patches were written, and they appear in
`[SIGHTGLASS PATCH n]` markers in the source, so they are not renumbered when a patch lands out of
order. Two are lettered for that reason: **4b** is the leak patch 4 introduced, and **8b** is
`NoAutoHeadersKey`, which extends patch 8's magic-key machinery and shipped between patch 8 and
patch 9. Every number from 1 to 12 is used exactly once, in tag order:

| patch | what | first tag |
|---|---|---|
| 1  | per-request HEADERS priority | v0.6.9-sightglass.1 |
| 2  | connection WINDOW_UPDATE replenishes to the full window | v0.6.9-sightglass.1 |
| 3  | Chrome's HTTP/2 stream-concurrency limits | v0.6.9-sightglass.1 |
| 4  | detach the H1 dial from the request's cancellation | v0.6.9-sightglass.1 |
| 4b | bound the proxy CONNECT unconditionally | v0.6.9-sightglass.1 |
| 5  | the H2 connect honours the request's context | v0.6.9-sightglass.1 |
| 6  | HPACK pseudo-header indexing | v0.6.9-sightglass.2 |
| 7  | `Content-Encoding: deflate` deadlock and raw-DEFLATE corruption | v0.6.9-sightglass.3 |
| 8  | `HTTP1OmitKey` | v0.6.9-sightglass.4, .5 |
| 8b | `NoAutoHeadersKey` | v0.6.9-sightglass.6 |
| 9  | a connection-level GOAWAY never reached the wire | v0.6.9-sightglass.7 |
| 10 | the HPACK static NAME index is a per-engine choice | v0.6.9-sightglass.8 |
| 11 | `cancelStream()` reset precisely when already reset, and forgot nothing | v0.6.9-sightglass.9, .11 |
| 12 | a FINISHED stream reported as a CANCELLED one | v0.6.9-sightglass.10 |

Line numbers are deliberately NOT cited anywhere below. They drifted by up to 120 lines across the
twelve patches and every citation in earlier revisions of this file was stale; functions and
`[SIGHTGLASS PATCH]` markers are cited instead, and the markers are what `patches_doc_test.go`
checks.

---

## Patch 1 — per-request HEADERS priority

### The divergence it fixes

Upstream carries the HTTP/2 HEADERS-embedded PRIORITY on the **transport**:

```go
// http2/transport.go, type Transport
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

### The diff

One upstream file, `http2/transport.go`, in **four** hunks: the `writeHeaders` signature, the
per-request branch inside it, and the two call sites (`roundTrip` and the trailer write). They carry
the markers `SIGHTGLASS PATCH 1 (1/4)` .. `(4/4)`. Everything else is the new file
`http2/priority_perrequest.go`. (Revisions up to `v0.6.9-sightglass.11` described this as "2 call
sites + 1 signature" and marked only two of the four sites `(1/2)` and `(2/2)`.)

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
(`parity.TestParityHTTP2`, `parity.TestParityHTTP2PriorityWeightPerUrgency`,
`parity.TestParityHTTP2WireFrames`, `parity.TestParityHTTP2CookieCrumbling`, …) still passes
untouched.

### Why a context value and not a header sentinel

fhttp's own ordering hooks (`HeaderOrderKey`, `PHeaderOrderKey`) are magic header keys that must be
filtered out again in `encodeHeaders`. A priority sentinel of that shape that ever failed to filter
would put a **novel header on the wire** — strictly worse than the bug being fixed. A
`context.Context` value cannot reach the wire at all.

### How it is driven (and why it cannot drift)

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

### Proof (HR-7)

```
$ cd go && GOTOOLCHAIN=auto go test ./parity/ -run 'TestParityHTTP2MixedUrgency|TestBindChromeHeaderPriority' -v
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

---

## Patch 2 — connection-level WINDOW_UPDATE replenishes to the FULL window (matrix A-15)

`cc.inflow` is seeded in `(*Transport).newClientConn` with `cc.connFlow + initialWindowSize` — for a
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

---

## Patch 3 — Chrome's HTTP/2 stream-concurrency limits (matrix A-9)

The parity matrix listed A-9 as `unverifiable` ("needs the Chrome constant from source/NetLog"). It
is settled from the oracle's own Chromium 152 tree, and once settled it is a **three-way**
divergence, all of it server-visible (the origin simply counts the streams the client opens):

| | upstream fhttp v0.6.9 | Chrome 152 | source |
|---|---|---|---|
| initial (pre-SETTINGS) limit | `1000  // "infinite", per spec` | **100** | `net/spdy/spdy_session.h:84` `kInitialMaxConcurrentStreams`, applied `spdy_session.cc:837` |
| ceiling on the peer's advertised value | none — `cc.maxConcurrentStreams = s.Val` | **256** | `net/spdy/spdy_session.cc:383` `kMaxConcurrentStreamLimit`, applied `:2356-2357` |
| streams actually opened at the limit | `max - 1` (`len+1 < max`) | exactly `max` | `net/spdy/spdy_session.cc:1697-1699` `if (active_streams_.size() + created_streams_.size() < max_concurrent_streams_) return CreateStream(...)` |

### The diff (upstream file touched: `http2/transport.go`, 3 lines, all marked `[SIGHTGLASS PATCH 3]`)

```
http2/transport.go   (newClientConn, the ClientConn literal)
  -   maxConcurrentStreams:  1000,               // "infinite", per spec. 1000 seems good enough.
  +   maxConcurrentStreams:  ChromeInitialMaxConcurrentStreams,

http2/transport.go   (idleStateLocked, the non-StrictMaxConcurrentStreams branch)
  -   maxConcurrentOkay = int64(len(cc.streams)+1) <  int64(cc.maxConcurrentStreams)
  +   maxConcurrentOkay = int64(len(cc.streams)+1) <= int64(cc.maxConcurrentStreams)

http2/transport.go   (processSettingsNoWrite, the SETTINGS handler)
  -   cc.maxConcurrentStreams = s.Val
  +   cc.maxConcurrentStreams = chromeClampMaxConcurrentStreams(s.Val)
```

Everything else is the **new** file `http2/chrome_concurrency.go` (the two constants + the clamp).

### Proof (HR-7)

```
$ cd go && GOTOOLCHAIN=auto go test ./parity/ -run TestParityHTTP2Concurrency -v
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

### What it costs, measured

Keeping a dial upstream would have dropped means holding a local socket upstream would have released.
That is not free, and this fork's own suite is where it shows: `TestOmitHTTP2` and
`TestCancelRequestWhenSharingConnection` are the ONLY two tests this fork fails that pristine v0.6.9
passes on the same toolchain, and reverting this one expression makes both pass and takes the root
package from 654 s to 57 s. Both fail with `connect: can't assign requested address` — ephemeral-port
exhaustion — while a second full `net/http` suite runs in a subprocess beside them. The numbers and
the method are in "Regression diff" at the foot of this file; the open question of whether a bound
belongs here is ledger **T0525**. The product side is bounded by `MaxConnsPerHost` and is covered by
`sightglass.TestStressNoLeaks`, which returns goroutines and fds to baseline; the suite is the
extreme case, not the shipped one.

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
so four of them still asserted the pre-change contract — `git diff a8b1417 HEAD -- client_test.go
transport_test.go` touches exactly these four and nothing else. Upstream rewrote all four at the
time; these now match:

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

---

## Patch 5 — the H2 connect honours the request's context (uncancellable-dial hang)

Same defect class as patch 4, on the **HTTP/2** side, and worse: this fork carries the pre-2021
x/net/http2 pool, where the dial takes **no context at all** —

```
fork          func (t *Transport) dialClientConn(addr string, singleUse bool) (*ClientConn, error)
x/net@v0.59.0 func (t *Transport) dialClientConn(ctx context.Context, addr string, singleUse bool) (*ClientConn, error)
```

— and `clientConnPool.getClientConn` then blocked on `<-call.done` with no select on the request's
context. The shipped stack makes that unbounded in practice: tls-client hands
`http2.Transport` a **legacy, context-free `DialTLS` hook** that dials with `context.Background()`
(`roundtripper.go`, `dialTLSHTTP2 -> rt.dialTLS(context.Background(), …)` — still true at
`v1.16.0-sightglass.6`, the version Sightglass ships beside this one), so the TLS handshake of an
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

---

## Patch 6 — HPACK pseudo-header indexing, so the encoder emits what a browser emits

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

---

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

---

## Patch 8 — `HTTP1OmitKey`, for headers that belong on HTTP/2 but not on HTTP/1.1

Some headers are protocol-scoped in a direction the existing machinery could not express.

**Connection-specific fields go the easy way.** HTTP/2 already drops `Connection`, `Keep-Alive`,
`Proxy-Connection`, `Transfer-Encoding` and `Upgrade` itself (RFC 9113 §8.2.2, `transport.go` in the
`enumerateHeaders` skip list) and `checkConnHeaders` explicitly permits `Connection: keep-alive`. So
a caller sets them unconditionally and only the HTTP/1.1 serializer writes them.

**The other direction had no equivalent.** A header that belongs on HTTP/2 and HTTP/3 but *not* on
HTTP/1.1 is written by the HTTP/1.1 serializer, and the caller cannot know which protocol will be
negotiated: ALPN is decided at dial time and the request is built before that.

The concrete case is RFC 9218's `priority`. Chrome sends it on **112 of 119** captured HTTP/2
requests, as the last field, and on **0 of 744** captured HTTP/1.1 requests — 722 over TLS with
forced ALPN plus 22 to a flag-free `http://localhost` origin. It has no HTTP/1.1 form at all.
Without this key a request built once and sent over whichever protocol the origin offers either
loses `priority` on HTTP/2 or invents it on HTTP/1.1.

```
header.go        HTTP1OmitKey = "HTTP1-Omit:"; writeSubset drops the named headers, both on the
                 ordered path (SortedKeyValuesBy) and the unordered one (SortedKeyValues)
http2/transport.go, h2_bundle.go, transport.go
                 the key is skipped by the HTTP/2 encoder and ALLOWED by both header-name
                 validators, exactly as HeaderOrderKey and PHeaderOrderKey already are. The
                 HTTP/1.1 one (transport.go) matters as much as the HTTP/2 one: a magic key ends
                 in ':', which httpguts.ValidHeaderFieldName rejects, so without the allowance
                 every HTTP/1.1 request carrying it fails before it is written
```

Names match case-insensitively — HTTP/1.1 names are case-insensitive, and a library whose point is
that the case on the wire is chosen deliberately must not make the caller guess which spelling to
name here.

### Tests

`header_http1omit_test.go`: the header is written without the key and gone with it, on the wire
bytes of `Request.Write`; the magic key itself never reaches the wire; nothing else is dropped;
case-insensitive in both directions; and it works on the ordered path, which is the one a
browser-emulating caller actually takes.

### Verification

```
before (v0.6.9-sightglass.3): 50 failing
after:                        49 failing
NEW failures: none
```

---

---

## Patch 8b — `NoAutoHeadersKey`: the caller states the block, and the library adds nothing to it

`v0.6.9-sightglass.6`

A caller who states an exact request block did not get one. Three sites add headers behind the
caller's back, and all three trigger on **absence**, so none of them is reachable through the header
map:

```
request.go          Request.write        User-Agent: Go-http-client/1.1      when neither the
                                                                            "User-Agent" nor the
                                                                            "user-agent" key exists
transport.go        Transport.roundTrip  Accept-Encoding: gzip, deflate, br  when Header.Get is ""
http2/transport.go  encodeHeaders        user-agent: Go-http-client/2.0      when no UA field was
                                         accept-encoding: gzip, deflate, br  emitted / Get is ""
h2_bundle.go        the same two, in package http's own copy of the encoder
```

Measured on a loopback origin, with a caller asking for exactly three headers:

```
caller asked for:      X-One, Accept, X-Two

HTTP/1.1 wire:         X-One / Accept / X-Two / User-Agent: Go-http-client/1.1
                                              / Accept-Encoding: gzip, deflate, br
HTTP/2  wire:          x-one / accept / x-two / accept-encoding: gzip, deflate, br
                                              / user-agent: Go-http-client/2.0
```

Writing an empty value suppresses the two User-Agent sites (they test the map key) but **not** the
two Accept-Encoding ones: those call `Header.Get`, which cannot distinguish "no value" from "not
set". `Transport.DisableCompression` is the only existing lever, and it is connection-wide and
entangled with response decoding, so it cannot express *this request carries exactly these fields*.

For a caller emulating a browser the injected values are the loudest possible leak: a byte-exact
browser TLS ClientHello followed by `user-agent: Go-http-client/2.0`.

```
header.go            NoAutoHeadersKey = "No-Auto-Headers:" and Header.NoAutoHeaders(); writeSubset
                     excludes it on both the ordered and the unordered path
request.go           the HTTP/1.1 User-Agent injection is gated on it
transport.go         the HTTP/1.1 Accept-Encoding injection is gated on it; the header-name
                     validator allows the key (it ends in ':', which httpguts rejects)
http2/transport.go   requestGzip and the didUA fallback are gated on it; the key is skipped by the
h2_bundle.go         encoder and allowed by the validator, as the other magic keys already are
```

Values are ignored — presence is the whole signal. The cost is the automatic gzip request **and**
the automatic gunzip of the response, which is the honest pairing: the package only decodes what it
asked for.

### Also fixed here

`h2_bundle.go`'s encoder skipped `PHeaderOrderKey` and `HeaderOrderKey` when writing fields but not
`HTTP1OmitKey`, so package `http`'s own HTTP/2 client emitted `http1-omit:: priority` as a header
field — not a legal HPACK name. `fhttp/http2`, the package `tls-client` drives, already skipped it;
this copy did not, so the defect was one `Transport` choice away from the wire.

### Tests

`no_auto_headers_test.go`: with the key, `Request.Write` produces exactly the stated block and no
`User-Agent`; **without** it the same request grows the injected one (the ablation), so the test
fails if the gate is removed. The magic key never reaches the wire on either the ordered or the
unordered path. The HTTP/2 sites are proven end-to-end on a real h2 loopback origin by Sightglass's
`parity.TestParityCallerStatedBlockIsExactOnH2`.

### Verification

```
before (v0.6.9-sightglass.5): 33 failing
after:                        33 failing
NEW failures: none
```

---

## Patch 9 — a connection-level GOAWAY never reached the wire

`http2/transport.go`, `readLoop`.

`readLoop` answers a `ConnectionError` by writing a GOAWAY and then returning, at which point the
deferred `rl.cleanup()` closes the connection:

```go
cc.readerErr = rl.run()
if ce, ok := cc.readerErr.(ConnectionError); ok {
    cc.wmu.Lock()
    cc.fr.WriteGoAway(0, ErrCode(ce), nil)
    cc.wmu.Unlock()
}
```

`Framer.endWrite` writes into `cc.bw`, a `*bufio.Writer`, and does **not** flush. Every other write
path in this file calls `cc.bw.Flush()` explicitly; this one did not. So the GOAWAY sat in the
buffer and the close discarded it, and the frame log said `wrote GOAWAY ErrCode=PROTOCOL_ERROR`
while the peer received nothing.

**Two costs.** The peer loses the error code it needs to understand what it did wrong, and a browser
becomes distinguishable from this client in a single frame: Chrome answers a connection-level
protocol error with GOAWAY and *then* closes, which is exactly what this code was already trying to
do. A peer that provokes a protocol error — an unsolicited PUSH_PROMISE will do it — sees an abrupt
close from us and a diagnosed one from a browser.

**Inherited from upstream `golang.org/x/net/http2`**, which has the same omission — and still does:
`golang.org/x/net@v0.59.0/http2/transport.go` writes the GOAWAY under `cc.wmu` and returns without a
`cc.bw.Flush()`, exactly as this file did. Recorded because it means the fix is not a divergence from
upstream's intent but a completion of it.

Guard: `http2/goaway_flush_test.go`. A real TCP pair, not `net.Pipe` — `NewClientConn` writes the
preface and SETTINGS synchronously and an unbuffered pipe deadlocks before a reader can start. Two
things the test has to get right, and both were wrong in earlier drafts, so they are commented in
place: the server must consume the 24-byte client preface before reading frames (otherwise
`PRI * HTT` parses as a frame header with a 5 MB length), and the PUSH_PROMISE must carry a
well-formed header block (`readMetaFrame` runs `checkPseudos()` first, and a malformed block is a
STREAM error, which produces no GOAWAY at all).

Ablation:

```
--- FAIL: TestGoAwayIsFlushedOnAConnectionError
        the client hit a connection-level PROTOCOL_ERROR and put NO GOAWAY on the wire
```

### It also fixes a hanging upstream test, and the mechanism is NOT fully traced

`TestTransportReturnsUnusedFlowControlSingleWrite` — an upstream Go test (golang.org/issue/20469) —
**hangs to the test timeout without this patch and passes in ~0.3 s with it**, reproducibly:

```
WITH patch 9      ok 0.787s   ok 0.368s   ok 0.274s
WITHOUT patch 9   FAIL        FAIL        FAIL       (each a 90 s timeout panic)
```

Under `http2debug=2` the client's framer logs `wrote RST_STREAM` and `wrote WINDOW_UPDATE`, and the
test's server framer never reads either — it blocks in `ReadFrame`. So those frames do not reach the
wire without this flush.

**What is NOT established:** why. `writeStreamReset` and the body-close window-update path both call
`cc.bw.Flush()` themselves, so on the face of it those frames should already have been flushed. A
plausible chain is `stickyErrWriter` — `cc.bw` wraps one, so once `cc.werr` is set every later write
is silently dropped — but that has not been traced, and this file does not claim it.

What IS established is that the patch is a strict improvement: a previously hanging test now passes,
and the failure set is otherwise byte-identical (below).

---

## Patch 10 — the HPACK static NAME index was one browser's identity, hardcoded

### The defect

RFC 7541 Appendix A gives four header names more than one static entry: `:method` (2 GET, 3 POST),
`:path` (4 `/`, 5 `/index.html`), `:scheme` (6 http, 7 https) and `:status`. When an encoder spells a
field out but cites its NAME by index — a literal with an indexed name — it emits whichever entry it
resolved to. That choice is invisible for a name+value hit and **on the wire** for every other
occurrence, one byte per field.

Upstream resolves a duplicated name to the **highest** index, because `addEntry` writes
`byName[name]` unconditionally. An earlier Sightglass patch rebuilt the static table to resolve to
the **lowest** instead, because that is what Chrome does. It was measured and it was right about
Chrome — and it was still wrong, because it put **one browser's identity into the stack as a
constant**. A Firefox profile driving this fork emitted Chrome's index and could not be corrected
from the profile, which is the failure the profile-driven design exists to prevent.

Both behaviours are real, and both are measured:

| engine | policy | `:path` | `:method` | evidence |
|---|---|---|---|---|
| Chrome 153 | first match | 4 | 2 | 114 `:path` + 8 `:method` observations, two captures, zero exceptions |
| Firefox 156 | last match | 5 | 3 | 41 of 41 attributed HEADERS blocks; leading byte `0x05` where a first-match encoder emits `0x04`; confirmed independently by the tshark HPACK dissector |

### The fix

`newStaticTable(lastMatch bool)` builds both variants; `staticTable` keeps first-match and
`staticTableLastMatch` is upstream's. `Encoder.SetStaticNameIndexPolicy(bool)` selects per encoder,
mirroring the existing `SetIndexingPolicy` seam, and `Transport.HPACKStaticNameLastMatch` threads it
from the caller's profile. Only the NAME-only lookup is affected: `ents` and `byNameValue` are
identical in both tables, so name+value hits and **all decoding** are unchanged.

One trap worth recording. `idToIndex` distinguished static from dynamic by comparing against the
single global `staticTable` **pointer**, so a second static table would have been classified dynamic
and indexed as `len()-k` instead of `k+1` — a wrong index on every field. Tables now carry
`static bool`. The pointer check is deliberately KEPT alongside it, because upstream's own
`TestHeaderFieldTable` simulates a static table by temporarily reassigning that global
(`http2/hpack/tables_test.go`; up to v0.6.9-sightglass.11 this entry cited that test under a name
that does not exist in this tree), and
dropping it would have silently changed what upstream's test measures.

### Tests

`http2/hpack/static_name_index_test.go`, three tests:

- **the wire byte, both ways** — `:path` with a value in neither static entry encodes with name index
  4 under first-match and 5 under last-match, and round-trips to the same field under either, since
  this is an encoder signature and not a protocol change.
- **the control** — a name+value hit (`:method GET`, `:path /`) and a unique name (`:authority`,
  `user-agent`) must encode **byte-identically** under both policies, or the patch changes more than
  it claims.
- **the table invariant** — the two static tables must agree on `ents` and `byNameValue`, both be
  marked static, and differ on `byName` for exactly the duplicated names (4).

Ablation: making `searchTable` ignore the policy fails the last-match case with
`:path name index = 4, want 5`.

### Verification

```
before (v0.6.9-sightglass.7): 1 failing  (TestTransportRejectsConnHeaders, pre-existing upstream)
after:                       1 failing  (identical set)
NEW failures: none
```

`go test ./http2/hpack/` is fully green both before and after.

---

## Patch 11 — `cancelStream()` reset a stream precisely when it had already been reset

### The defect

`clientStream.didReset` means "we have already sent a RST_STREAM for this stream". The reset
therefore belongs on the `!didReset` branch, which is what the upstream `golang.org/x/net/http2`
this package's `http2` was vendored from has — verified, not remembered:
`golang.org/x/net@v0.0.0-20190620200207-3b0461eec859/http2/transport.go`, `cancelStream`, reads
`if !didReset { cc.writeStreamReset(...); cc.forgetStreamID(...) }`, both statements inside the one
`if`. (Current x/net no longer has `cancelStream` at all — it was refactored away — so "upstream has
`!didReset`" has to be pinned to that generation or it cannot be checked.) This fork read `didReset` — the same function, byte-identical but for
the negation — so it did the opposite of its purpose in both directions:

- a stream that had **already** been reset got a **second** `RST_STREAM`;
- a stream that had **not** been reset got **none**, so the reset the function owed the peer was
  never sent.

The first is a wire divergence. Chrome 153 sends exactly one `RST_STREAM` per reset stream — 6
resets on 6 distinct streams across both ground-truth captures, never two on one stream — and a
second reset on a stream the peer has already closed is trivially loggable by any origin.

The race that exposes it: `transportResponseBody.Close()` writes `RST_STREAM(CANCEL)`, sets
`didReset = true`, writes `WINDOW_UPDATE(0, unread)` to return connection flow control, then forgets
the stream. When the request context is cancelled by that same `Close()`, `cancelStream()` runs just
behind it, observes `didReset == true` and writes the second reset. The measured sequence is exactly:

```
RST_STREAM(N, CANCEL)   WINDOW_UPDATE(0, unread)   RST_STREAM(N, CANCEL)
```

Reproduced in Sightglass at **12 of 200 runs (6.0%)** — five independent attempts of 40, scored
4/40, 3/40, 3/40, 1/40, 1/40 — and more under CPU load. (An earlier revision quoted only the first
three attempts and called it "~8-10%"; the full five-attempt figure is 6.0% and is the one below.)

The second is a **leak**, and this entry originally got that wrong. It claimed "no stream-map leak
resulted from the old code — `Close()` calls `forgetStreamID` unconditionally — so the only lost
behaviour on the other branch was the reset itself." That is true only of the path where the caller
**closes the response body**. `cc.forgetStreamID` sits inside the very same `if` as the reset, so on
the path where the caller cancels the request **context** with the body still open — where
`transportResponseBody.Close()` never runs and `cancelStream()` is the only code that can release
the stream — the old condition wrote no reset **and** forgot nothing. The `clientStream` stayed in
`cc.streams` for the life of the connection, holding its accounting and one of the connection's
concurrency slots.

Measured through Sightglass's shipped entry point (`parity.TestParityHTTP2CancelledStreamsDoNotConsumeSlots`,
`NewSessionFactory -> Session.Do`): with the condition inverted, 100 context-cancelled requests on
one connection filled `ChromeInitialMaxConcurrentStreams = 100`, and request 101 could not be sent
on that connection at all —

```
/slot/101: Session.Do: Get "https://127.0.0.1:64343/slot/101": EOF
Request 101 of 150 on ONE connection, after 100 requests that were cancelled by their context
with the body still open. ... Extra connections dialled so far: 1.
```

So the old code was a wire divergence **and** an unbounded per-connection leak, and the fix closes
both.

### The fix

Negate the condition. One character, and it restores both branches at once.

### Tests

`http2/cancel_stream_reset_test.go` deliberately does **not** drive a request. The defect surfaces
through a race that fires ~10% of the time, and a 10% assertion is not a guard — it is a coin flip
that is green most of the time. The test calls `cancelStream()` directly with each value of
`didReset` over a real TCP pair and counts the `RST_STREAM` frames the peer actually received, so it
pins the condition and gives the same answer on every run.

Ablation, restoring the inverted condition — it fails in **both** directions, which is the point:

```
didReset=false: peer received 0 RST_STREAM, want 1
didReset=true:  peer received 1 RST_STREAM, want 0
```

`TestCancelStreamForgetsTheStream`, added when the "no stream-map leak" claim above was corrected,
pins the OTHER statement inside the same `if`: it reads `cc.streams` after `cancelStream()` returns
and before `cc.Close()` (which tears every stream down and would erase the difference). Ablation,
restoring the inverted condition:

```
cancel_stream_reset_test.go:159: didReset=false: the clientStream was still in cc.streams after
cancelStream() — cc.forgetStreamID sits inside the same `if` as the reset, so an inverted condition
leaks one clientStream and one concurrency slot per cancelled request, for the life of the connection
--- FAIL: TestCancelStreamForgetsTheStream (0.00s)
```

### Verification

Regression diff for the correction above (the comment, the `PATCHES.md` text and
`TestCancelStreamForgetsTheStream`; no product code changed): `GOTOOLCHAIN=auto go test ./... -count=1`,
**41 failing before, 41 after, identical set, no new failures**. The root `fhttp` package hit the
default 10-minute per-package timeout in both passes of that run (601.3s before, 600.6s after), which
is where most of that 41 came from; it is unaffected by this change. Whole-tree failure counts quoted
in individual patch entries were each measured on the day of that patch, under whatever else this
machine was running, and they are NOT comparable with each other — the single authoritative
measurement of this tree is under "Regression diff" at the foot of this file.

Baseline before the fix, measured through Sightglass's own H2 census over five independent
attempts of 40 runs each:

```
4/40  3/40  3/40  1/40  1/40   =  12 of 200 runs (6.0%) sent two RST_STREAM on one stream
```

Regression diff for the product change itself, `go test ./http2/ -count=1` (this patch's own
package — NOT the whole tree; the whole-tree number is in "Regression diff" at the foot of this
file, and the two are not comparable):

```
before: 2 failing  (TestTransportRejectsConnHeaders, pre-existing upstream;
                    TestCancelStreamResetsOnlyWhenNotAlreadyReset, this patch's own guard)
after:  1 failing  (TestTransportRejectsConnHeaders only)
NEW failures: none
```

---

## Patch 12 — a stream that FINISHED was reported as a stream that was CANCELLED, and patch 11 turned that into an RST_STREAM

### The defect

Patch 11 restored `cancelStream()`'s `!didReset` condition, so a cancellation that arrives before any
reset now correctly writes one. That was right, and it exposed a second defect underneath it: the
caller was asking the wrong question.

`awaitRequestCancel(req, done)` selects over three channels — `req.Cancel`, `ctx.Done()` and `done`
(the stream's completion). Its one caller, `clientStream.awaitRequestCancel`, treats any non-nil
return as "the user cancelled" and calls `cancelStream()`.

The problem is that **`net/http`'s own Client cancels a timed request's context as part of FINISHING
it.** `setRequestCancel` (client.go) builds

```go
stopTimer = func() { once.Do(func() { close(stopTimerCh); if cancelCtx != nil { cancelCtx() } }) }
```

and `stopTimer` runs when the response body reaches EOF or is closed. So on an entirely ordinary
request with a `Client.Timeout`, both `ctx.Done()` and `done` end up closed — `done` first, closed by
the read loop the moment `END_STREAM` arrives; `ctx.Done()` a few microseconds later, when the caller
drains the body.

**A Go select chooses uniformly at random among the cases that are ready.** So this function returned
`ctx.Err()` for a completed stream roughly half the times its goroutine was scheduled after both had
fired, and with patch 11 in place that wrong premise became a real `RST_STREAM(CANCEL)` on a stream
the server had already ended.

Chrome 153 never resets a stream that ended: all 6 of its RST_STREAMs across both Sightglass
ground-truth captures are on streams it abandoned, and ~94 completed streams carry none. A reset on a
completed stream is as loggable as the duplicate patch 11 removed.

Measured through Sightglass against a loopback h2 listener, 12 runs of a three-leg scenario
(bodyless GET, POST with a body, one abandoned body):

```
v0.6.9-sightglass.8   0 of 12 runs   (the inverted condition happened to swallow it)
v0.6.9-sightglass.9   4 of 12 runs   RST_STREAM(CANCEL) on stream 1 or 3 — the COMPLETED GET/POST
```

and on the 240-leg guard (120 abandoned + 120 completed on one connection), `.9` reset **48 of 120
completed streams**.

### The fix

Re-read `done` after the select and prefer it:

```go
var err error
select {
case <-req.Cancel:
	err = errRequestCanceled
case <-ctx.Done():
	err = ctx.Err()
case <-done:
	return nil
}
select {
case <-done:
	return nil
default:
	return err
}
```

Losing the race the other way is harmless: if the context really was cancelled first and the stream
then completed, there is nothing left to cancel and no reset is owed.

### Tests

`http2/await_request_cancel_test.go`. Like patch 11's test it deliberately does NOT drive a request —
the defect is a 50/50 schedule race and a 50% assertion is a coin flip, not a guard. It calls
`awaitRequestCancel` directly with both channels already closed, which is exactly the state a
completed timed request leaves behind, **200 times**, because one call says nothing against a
uniform-random select. It also pins the three cases the fix must not break: a cancelled context on an
open stream, an expired deadline on an open stream, and a finished stream under a live context.

Ablation: deleting the second select (returning `err` directly) fails with

```
awaitRequestCancel reported a cancellation on 102 of 200 calls where the stream was ALREADY DONE
```

### Verification

```
before (v0.6.9-sightglass.9): 34 failing  (pre-existing upstream; see the set in fork.before)
after:                        34 failing  (identical set)
NEW failures: none
```

Both runs with `go test ./... -count=1 -timeout 40m`. The shorter default timeout is not enough when
two full suites run concurrently on this machine and produces two spurious package-level timeouts.

---

## Regression diff

Upstream v0.6.9 is **not green**, so "the suite passes" is not an available bar and this file never
claims it. The bar is a regression diff: the same command, on the same machine, against the published
tag and against this tree, reporting only failures that are NEW here.

Per-patch entries above quote the diff measured on the day that patch landed, under whatever else
this machine was running; those numbers are not comparable with one another and must not be read as a
running total. The measurement for the tree as it stands is here, and it is the one to trust.

<!-- REGRESSION-DIFF-BEGIN (regenerated whenever this tree changes; patches_doc_test.go checks the
     marker is present and that the tag named below is the tag this tree is at) -->
```
command: GOTOOLCHAIN=auto go test ./... -count=1 -timeout 25m
         run on the same machine, one after the other, never concurrently
before:  v0.6.9-sightglass.11 in a clean worktree of the published tag (895d5a8)
after:   v0.6.9-sightglass.12, this tree

before:  36 failing tests   root package 654.713s
after:   36 failing tests   root package 654.070s
NEW failures: none
FIXED:        none (this tag changes no product code)

The two failing sets are IDENTICAL, test for test. Packages: fhttp, fhttp/http2 and
fhttp/httputil FAIL on both sides; cgi, cookiejar, fcgi, http2/h2c, http2/hpack, httptest,
httptrace, internal, internal/profile and pprof are ok on both sides.

Why -timeout 25m and not the default: 601 s of the root package's 654 s is `TestOmitHTTP2`,
an UPSTREAM test inherited verbatim, which shells out to
`go test -short -tags=nethttpomithttp2 net/http` and therefore runs the entire standard
library net/http suite in a subprocess. It fails there on ephemeral-port exhaustion
(`dial tcp 127.0.0.1:57639: connect: can't assign requested address`). Earlier entries in
this file that reported "41 failing" with the root package at "601.3s" were reporting this
test hitting the old 10-minute DEFAULT per-package timeout and killing the package — not a
hang, and not a larger failure set. With 25m the package completes and the count is stable
across runs.

AND IT IS OURS. Against pristine v0.6.9 run on the same toolchain
(`GOTOOLCHAIN=go1.27.0`, so the comparison is source and not language version) this fork
fails exactly two tests upstream does not — `TestOmitHTTP2` and
`TestCancelRequestWhenSharingConnection` — and both are **patch 4**. Measured by reverting
patch 4's one expression, `context.WithCancel(context.WithoutCancel(ctx))` back to
`context.WithCancel(ctx)`, and running the same command:

    fork v0.6.9-sightglass.12                 36 failing   root package 654.3 s
    the same tree minus patch 4's WithoutCancel  34 failing   root package  56.8 s
    pristine v0.6.9 @ GOTOOLCHAIN=go1.27.0    36 failing   root package  60.1 s

and the two that disappear are precisely those two. It is not our added tests: skipping
every test in the eight files this fork adds to the root package leaves `TestOmitHTTP2` at
601.55 s and still failing. Run on its own in this tree it passes in 2.9 s, so the trigger
is the rest of the package running beside it.

The mechanism follows from what patch 4 is FOR. Upstream tears down the dial that loses the
idle-vs-dial race; this fork lets it run to completion and join the pool, which is the whole
point — 0 abandoned handshakes at the origin instead of 11 214, and 3x throughput. The cost
is that we hold local sockets upstream would have dropped, and a package run that opens tens
of thousands of connections while a second full net/http suite runs in a subprocess beside it
exhausts the ephemeral port range. That is a real consequence of a shipped patch, recorded
here rather than filed under "environment", and tracked as ledger T0525. What it is NOT is a
new failure in this tag: `.11` and `.12` fail the same 36, identically.

Two upstream failures this fork FIXES, for the same reason the suite is kept:
`TestCompressionDeflate` (patch 7 — upstream's own test for raw DEFLATE, which it fails) and
`TestDescriptions` (patch 4b's `goroutineleak` profile entry — pristine's `pprof` package
fails; ours is ok).

Measured once with this file's regression block still unfilled, which produced exactly one
extra failure — `TestPatchesMDCarriesARegressionDiff`, the guard that refuses an unfilled block
here — and once again with it filled, which is the 36 above.
```
<!-- REGRESSION-DIFF-END -->

---

## Maintenance

**The instruction that used to sit here was wrong, and following it would have deleted the suite that
catches the defects.** Up to and including `v0.6.9-sightglass.11` this section read "re-copy upstream,
strip tests, re-apply the hunks above (four for patch 1, one for patch 2, three for patch 3, three for
patch 4, two for patch 5)". Three things about that were false or harmful:

* **"strip tests"** — upstream's 73 test files are KEPT and are the reason patch 4b exists. Stripping
  them is how the CONNECT leak survived patch 4 in the first place.
* **"re-copy"** — this is a fork with a rewritten module path across 86 files. A copy reverts the
  rewrite, and the tree then fails to compile against `go/go.mod`.
* **"the hunks above"** — the section sat in the MIDDLE of the file, so "above" excluded patches 4b,
  6, 7, 8, 8b and 10: six of the fourteen entries, verified by listing the `## ` headings of the
  published `v0.6.9-sightglass.11` file (`git show 895d5a8:PATCHES.md`), where `## Maintenance` is
  at line 609 and those six are the only sections below it. It now sits at the end, and there is no
  "above" left to get wrong.

### On an upstream bump

1. `git fetch https://github.com/bogdanfinn/fhttp --tags` and **merge** the new tag into the
   `sightglass` branch. Do not copy a tree over this one.
2. Rewrite the module path in any file upstream ADDED:
   `github.com/bogdanfinn/fhttp` → `github.com/Berserk-Automation-Hub/fhttp`, and
   `github.com/bogdanfinn/utls` → `github.com/Berserk-Automation-Hub/utls`.
3. **Keep every `*_test.go`, `export_test.go` and `testdata/`.** If a merge conflict tempts you to
   drop one, resolve it instead.
4. Resolve conflicts against the markers, not against this file's prose. Every site of every patch
   carries a `[SIGHTGLASS PATCH n]` comment; `grep -rn 'SIGHTGLASS PATCH' --include='*.go' .` lists
   them all, and `patches_doc_test.go` fails if a patch documented here has no marker in the tree, or
   a marker in the tree has no entry here.
5. Update the base commit, the file manifest and the counts in this file. `patches_doc_test.go`
   re-derives all three from `git diff` and fails if you do not.
6. Re-run, in this order:
   * `GOTOOLCHAIN=auto go test ./... -count=1 -timeout 20m` here, and diff the failure set against the
     same command on the previous tag. NO NEW failures is the bar; green is not.
   * `gofmt -l` over **only the files you touched**. This fork promises a gofmt baseline identical to
     upstream's, and upstream's is not clean — `gofmt -w .` across the tree would produce a diff of
     thousands of lines that is not ours.
   * Update the "Regression diff" block above with the numbers you just measured.
7. Tag the **next** version in sequence (`v0.6.9-sightglass.N+1`), push the tag, and bump
   `go/go.mod` to it. Never re-point a published tag: the prose in this file has been wrong in
   published tags before, and the correction is a new tag, not an edit to an old one.
8. In Sightglass, with the new tag consumed:
   `gofmt -l . && go vet ./... && go test ./... -count=2`, and in particular the guards that drive
   `sightglass.NewSessionFactory -> Session.Do` through this fork:

   ```
   go test ./parity/ -count=1 -run 'TestParityHTTP2|TestParityNoAbandonedH1Dials|TestParityH2DialHonoursRequestContext|TestParityHTTP2CancelledStreamsDoNotConsumeSlots|TestParityCallerStatedBlockIsExactOnH2'
   ```

   A fork change that is not tagged, pushed and consumed with that suite green is not shipped.

### Sibling-fork pins

This fork's `go.mod` requires `github.com/Berserk-Automation-Hub/utls`, currently
**`v1.7.8-sightglass.6`**. It must never pin an OLDER utls than `go/go.mod` ships: Go's minimal
version selection resolves the Sightglass build to the newer one anyway, so the fork's own suite
would be testing a utls the product does not use, and nothing would say so. That is exactly what
happened — `v0.6.9-sightglass.11` pinned `v1.7.8-sightglass.1` while `go/go.mod` shipped
`v1.7.8-sightglass.6`; `.12` bumps it.

`patches_doc_test.go` fails if this paragraph and `go.mod` name different versions. It cannot check
the other half — that this pin is not older than `go/go.mod`'s — because the fork has no view of
Sightglass; that half is checked on the Sightglass side by
`parity.TestForkGoModsDoNotPinOlderSiblingForks`. Check both files when either moves.
