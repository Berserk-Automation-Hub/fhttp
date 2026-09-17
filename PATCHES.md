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

This tree carries **82 `*_test.go` files: 73 from upstream v0.6.9, of which 28 are byte-identical to
upstream, 40 carry nothing but the module-path rewrite and 5 are edited, plus 9 added by this
fork.** `export_test.go` and `testdata/` are kept too.

**RETRACTED, FALSE** — every revision up to `v0.6.9-sightglass.18` said
"68 carry nothing but the module-path rewrite" here, and that number measures nothing.
Only 45 of the 73 upstream test files appear in `git diff --name-status` against the base at all;
the other 28 are byte-identical to upstream, because they never mention the module path.
45 minus the 5 edited leaves 40 that carry only the rewrite. The guard meant to catch this
checked `untouched + edited == upstream`, an identity satisfied by ANY wrong description of the 68;
`TestPatchesMDTestFileSplitIsDerivedFromTheTree` derives all three numbers from the tree instead.

Keeping them is load-bearing, not tidiness: restoring upstream's suite is what found patch 4b
(a leak patch 4 introduced, which hung `TestTransportProxyHTTPSConnectLeak` for the full test
timeout) and what forced the two toolchain-drift fixes below. The five edits are listed in the
manifest, each against the patch that made it necessary.

**RETRACTED CLAIMS.** Published tags cannot be edited, so the corrections live here, each beside the
sentence it corrects. `patches_doc_test.go` pins every one of them: the claim must still appear (a
retraction nobody can grep for is not a retraction), and every appearance must satisfy THREE
conditions: it is QUOTED, not stated; a RETRACTED / FALSE / HARMFUL marker sits within 150 bytes of
it; and a phrase out of THAT claim's own correction sits within 400. The second and third conditions
were added in `v0.6.9-sightglass.18` because marker-presence alone was not a guard. Those 3
markers occur 38 times in this file, so an adversarial reader re-asserted the stream-map
sentence below verbatim in a paragraph whose only nearby marker retracted the BASE VERSION, and
every doc guard this file carried at `v0.6.9-sightglass.17` — fourteen of them — stayed green.
Proximity is not association, and a retraction quotes the published sentence rather than asserting
it.

* **RETRACTED, FALSE** — "the base is v0.6.8". It is v0.6.9, commit `a8b1417`; the tag "v0.6.8" is
  `ecfe905`, one release earlier — a different commit, not a different spelling of this one.
  Verified against `git ls-remote --tags https://github.com/bogdanfinn/fhttp`.
* **RETRACTED, FALSE** — "vendored here and wired in with `replace github.com/bogdanfinn/fhttp` =>
  `./third_party/fhttp`". There is no such directive and no such directory, and the rule forbids
  both.
* **RETRACTED, FALSE** — "stripped of `*_test.go`, `export_test.go` and `testdata/` … 0 test files".
  The tree carries 82, and the retained upstream suite is what found patch 4b. (Revisions up to
  `v0.6.9-sightglass.17` said 81 HERE while the front matter above said 82, and the false string was
  additionally hard-coded in `patches_doc_test.go`: a retraction section correcting a false count
  with a false count. Every statement of the count in this file is now swept, not just the first.)
* **RETRACTED, HARMFUL** — the `## Maintenance` instruction to "re-copy upstream", and its
  instruction to "strip tests". A re-copy reverts the rewritten module path and the tree stops
  compiling; HARMFUL too, "strip tests" would delete the suite that catches the defects —
  Upstream's 73 test files are KEPT. See `## Maintenance` at the end of this file.

## Files touched

Mechanically derived from `git diff --name-status a8b1417 HEAD`, and re-derived by
`patches_doc_test.go` on every run. 98 files in total: 12 added, 86 modified, of which 70 are
modified **only** by the module-path rewrite.

### Added (12)

```
A  PATCHES.md                              this file
A  http2/chrome_concurrency.go             patch 3   product
A  http2/priority_perrequest.go            patch 1   product
A  header_http1omit_test.go                patch 8   guard
A  http2/await_request_cancel_test.go      patch 12  guard
A  http2/cancel_stream_reset_test.go       patch 11  guard
A  http2/goaway_flush_test.go              patch 9   guard
A  http2/hpack/indexing_policy_test.go     patch 6   guard
A  http2/hpack/static_name_index_test.go   patch 10  guard
A  no_auto_headers_test.go                 patch 8b  guard
A  transport_deflate_leak_test.go          patch 7   guard
A  patches_doc_test.go                     —         guard on THIS FILE
```

Two product files and nine guards. Eight are new with the patch they guard — one each for patches 7,
8, 8b, 9, 10, 11 and 12, plus `http2/hpack/indexing_policy_test.go` for patch 6's indexing half — and
the ninth, `patches_doc_test.go`, guards this document rather than a numbered patch: it re-derives
the base commit, the manifest above, the patch numbers, the guard names, the marker counts and the
test-file count from the tree, and fails naming the sentence that stopped being true. That is also
why it is the one added guard carrying no `[SIGHTGLASS PATCH n]` marker. None of them existed
upstream. (Revisions up to `v0.6.9-sightglass.17` said "ten guards. Seven are new"; the count was
correct at eight guards / seven new before `indexing_policy_test.go` landed and was incremented on
the wrong side of the sentence. `patches_doc_test.go` now derives both numbers from the ADDED
manifest and the markers.)

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

**How much of this table is machine-checked, exactly.** The sentence that used to stand here —
"a guard cannot be cited into existence" — was FALSE, and an adversarial reader proved it by adding
the row `| 3 | http2/chrome_concurrency_test.go | parity.TestParityThisGuardWasNeverWritten |`, in
which neither the file nor the test exists, and watching every doc guard of the day — ten, at
`v0.6.9-sightglass.13` — stay green. Two holes: the
fork-side FILE names were never checked at all, and any name spelled `parity.X` was whitelisted
unconditionally because the fork has no view of Sightglass. Both are closed, in the two places that
can see the two halves:

| what | checked by | where |
|---|---|---|
| every unprefixed `Test…` name this file uses as a guard exists in THIS tree | `TestPatchesMDNamesGuardsThatExist` | here |
| every backticked `…_test.go` path named in THIS table exists on disk | `TestPatchesMDGuardTableNamesFilesThatExist` | here |
| every `parity.` / `sightglass.` prefixed `Test…` name this file uses resolves to a real `func Test…` in `go/` | `parity.TestForkPATCHESNamesSightglassGuardsThatExist` | Sightglass, reading this file out of the module cache |

| patch | guard in THIS fork | guard in Sightglass (shipped path) |
|---|---|---|
| 1  | — none | `parity.TestParityHTTP2MixedUrgencyOnOneConnection`, `parity.TestBindChromeHeaderPriorityCannotDriftFromTheHeader`, `parity.TestParityHTTP2PriorityWeightPerUrgency` |
| 2  | — none | `parity.TestParityHTTP2ConnectionWindowReplenish` |
| 3  | — none | `parity.TestParityHTTP2ConcurrencyConstantsMatchChromium`, `parity.TestParityHTTP2ConcurrencyClampsPeerAdvertisedLimit`, `parity.TestParityHTTP2ConcurrencyInitialLimitBeforeSettings` |
| 4  | — none | `parity.TestParityNoAbandonedH1Dials`, `sightglass.TestStressNoLeaks` |
| 4b | `TestTransportProxyHTTPSConnectLeak` (upstream's own, restored — it is what FOUND this patch) | — |
| 5  | — none | `parity.TestParityH2DialHonoursRequestContext` |
| 6  | `http2/hpack/encode_test.go` `TestEncoderSearchTable` for the static-NAME half; `http2/hpack/indexing_policy_test.go` for the indexing-policy half (`Encoder.SetIndexingPolicy`) — the representation octet, the dynamic-table consequence on the NEXT request, the Sensitive boundary and the nil-is-upstream control | `parity.TestParityHPACKEncoderMatchesChrome153`, `parity.TestParityHPACKPseudoHeaderRepresentationMatchesChrome153`, `parity.TestHPACKIndexingPolicyIsProfileDrivenNotHardcoded` |
| 7  | `transport_deflate_leak_test.go` | — (a parked goroutine and an fd have no wire signature) |
| 8  | `header_http1omit_test.go` | `parity.TestParityHTTP1Identity` (the magic key must not reach the HTTP/1.1 wire) |
| 8b | `no_auto_headers_test.go` | `parity.TestParityCallerStatedBlockIsExactOnH2` |
| 9  | `http2/goaway_flush_test.go` | — (the frame is on a connection Sightglass tears down; no parity row observes it) |
| 10 | `http2/hpack/static_name_index_test.go` | `parity.TestParityStaticNameIndexIsProfileDrivenOnTheShippedPath` |
| 11 | `http2/cancel_stream_reset_test.go` (`TestCancelStreamResetsOnlyWhenNotAlreadyReset` for the frame, `TestCancelStreamForgetsTheStream` for the slot and `cs.done`, `TestCancelStreamWakesTheSlotWaiter` for `cc.cond.Broadcast()` and the idle-timer re-arm) | `parity.TestParityHTTP2CancelledStreamsDoNotConsumeSlots` — 150 context-cancelled requests on ONE connection through `NewSessionFactory -> Session.Do` |
| 12 | `http2/await_request_cancel_test.go` | `parity.TestParityHTTP2CancelledStreamsDoNotConsumeSlots`, `parity.TestParityHTTP2WireFrames` |
| this file | `patches_doc_test.go` | — a document has no shipped path; it is guarded where it lives |
| sibling pins | — (this fork cannot see `go/go.mod`) | `parity.TestForkGoModsDoNotPinOlderSiblingForks` |

**The five with no fork test — 1, 2, 3, 4 and 5 — are the original batch**, written before this fork
kept upstream's suite, and each is a wire or dial property that a loopback origin in the consumer
observes directly. They are guarded, and ablated, in Sightglass; they are not guarded here. Patch 6's
indexing half WAS the sixth such gap, and it is now closed:
`http2/hpack/indexing_policy_test.go` is its fork-local guard, added in `v0.6.9-sightglass.15`
because code this fork ADDS is tested in this fork and not only through a consumer.

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

### What it costs, measured — and it is not only a test artefact

Keeping a dial upstream would have dropped means holding a local socket upstream would have released,
and under a workload that CANCELS requests faster than the pool can serve them that is unbounded.

`TestCancelRequestWhenSharingConnection` is upstream's own test for exactly that workload:
`MaxConnsPerHost = 1`, `MaxIdleConns = 1`, and ten goroutines that build a request, cancel its context
immediately (`go reqcancel()`) and loop for ten seconds, while an eleventh does ordinary requests and
fails the test if one errors. It is **deterministic**, not flaky:

```
with patch 4          6 runs, 6 failures   "unexpected: … dial tcp 127.0.0.1:49839:
                                            connect: can't assign requested address"  at ~6-8 s
without patch 4       4 runs, 4 passes     (the full ten seconds, no error)
pristine v0.6.9       1 run,  1 pass       (GOTOOLCHAIN=go1.27.0, same toolchain)
```

The mechanism is in `wantConn.cancel`: when the request goes away, upstream's dial goes with it, so
the socket never reaches ESTABLISHED. Here the dial is detached, so it COMPLETES, and
`cancel` hands the finished conn to `putOrCloseIdleConn` — which, with the pool already full, CLOSES
it. Every cancelled request therefore spends one full connect and leaves one local port in
`TIME_WAIT`. Ten goroutines cancelling as fast as they can exhaust the ephemeral range in about six
seconds.

`TestOmitHTTP2`, the other test this fork fails and upstream passes, is the same pressure seen from
the side: it shells out to a second full `net/http` suite, and with patch 4 it failed in 4 of 6 runs
of this tree (601 s each) and passed in 2 (when the rest of the suite happened not to be competing);
without patch 4 and in pristine it passed every time. That one is load-dependent, so it is reported
as suggestive rather than attributed.

**What this does NOT say.** It does not say Sightglass leaks ports. Sightglass bounds the pool
(`MaxConnsPerHost`) and `sightglass.TestStressNoLeaks` returns goroutines and fds to baseline over
1.08 M requests — but that workload does not cancel a request per iteration, which is the shape that
costs. Whether the shipped path needs a bound here is **ledger T0525**, open, to be settled by
counting sockets in `TIME_WAIT` under a cancel-heavy load with and without this expression, not by
assuming either answer. The numbers and the method for all of the above are in "Regression diff" at
the foot of this file.

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

### Ablation (the guard is falsifiable, not self-fulfilling)

Restoring the conditional bound — `connectCtx := ctx; if ctx.Done() == nil { connectCtx, cancel =
testHookProxyConnectTimeout(ctx, 1*time.Minute) }`, i.e. the shape this tree carried before the
patch — and running upstream's own test with a 90 s bound:

```
$ GOTOOLCHAIN=auto go test . -run TestTransportProxyHTTPSConnectLeak -count=1 -timeout 90s
panic: test timed out after 1m30s
	running tests:
		TestTransportProxyHTTPSConnectLeak (1m30s)
...
github.com/Berserk-Automation-Hub/fhttp.ReadResponse(0x482742e181e0, 0x482742f18200)
	/tmp/fhttp-fork/response.go:162 +0x80
github.com/Berserk-Automation-Hub/fhttp.(*Transport).dialConn.func4()
	/tmp/fhttp-fork/transport.go:1733 +0x1dc
created by github.com/Berserk-Automation-Hub/fhttp.(*Transport).dialConn in goroutine 8
FAIL	github.com/Berserk-Automation-Hub/fhttp	90.389s
```

That is the defect itself, not a mere difference: the dialConn goroutine is parked in `ReadResponse`
on a proxy that accepted the TCP connect and never answered, holding the socket, with no bound on it
at all. Patch 4 made the condition `ctx.Done() == nil` false in exactly the case the safety net
existed for. Restored, the test passes in well under a second.

### Verification

Baseline matters here: **pristine upstream v0.6.9 already fails 34 tests** under `go test -short`, so
"green" is not an available bar. The bar is a regression diff — run pristine and fork identically and
count only what fails in the fork and not in pristine. The tree-wide measurement is under "Regression
diff" at the foot of this file; this entry's own, measured the day it landed:

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
**114 `:path` and 8 `:method`** literal-with-indexed-name observations, with zero exceptions
(re-derived; the scope is in the Ground truth section below). Only the STATIC table is rebuilt; the
dynamic table keeps most-recent-wins, because its indices shift on eviction.

**2. The encoder indexed everything that fit.** Chrome never incrementally-indexes `:path` — the
value changes every request, so indexing it evicts useful entries for nothing — and never indexes a
literal `:method`, while it *does* index `:authority`, which is stable for the connection. Measured:
all **114** `:path` and all **8** `:method` literals are "without Indexing" — `:path` is
incrementally indexed **zero** times — and `:authority` is emitted "with Incremental Indexing" **45**
times.
`Encoder.SetIndexingPolicy` adds a per-field hook; `nil` is upstream behaviour exactly.
`Transport.HPACKIndexingPolicy` carries it, installed once per connection because HPACK is stateful
and a mid-connection change would desynchronise our table from the peer's view of it.

`Sensitive` is deliberately NOT the mechanism: it emits "Never Indexed" (`0x1x`), which carries an
explicit do-not-proxy instruction and which Chrome uses **zero** times in the **2513** request
header fields of the census (re-derived; the scope is in the Ground truth section below). It would
fix one octet and break another. The boundary
is guarded: `TestIndexingPolicyDoesNotOverrideSensitive` fails if a policy that returns true can
turn a `Sensitive` field into an indexed one.

### Result

```
:path "/x"   upstream default            -> 0x44
             with Chrome's policy        -> 0x04     (what Chrome emits)
```

### Ablation (both halves, separately)

**The static-NAME half.** Disabling the first-match rebuild in `newStaticTable` (`if false &&
!lastMatch`), i.e. upstream's table:

```
$ GOTOOLCHAIN=auto go test ./http2/hpack/ -count=1
--- FAIL: TestEncoderSearchTable (0.00s)
    encode_test.go:130: d.search(header field ":method" = "GET" (sensitive)) = 3, false; want 2, false
--- FAIL: TestStaticNameIndexPolicyIsOnTheWire/first-match_(Chrome) (0.00s)
    static_name_index_test.go:46: :path name index = 5, want 4 (leading byte 0x15)
--- FAIL: TestStaticTablesAgreeExceptOnByName (0.00s)
    static_name_index_test.go:120: byName is identical in both tables — the last-match table is not being built
```

**The indexing-policy half.** Disabling the hook in `shouldIndex` (`if false && e.indexingPolicy !=
nil`), which is exactly upstream's "index everything that fits":

```
$ GOTOOLCHAIN=auto go test ./http2/hpack/ -run 'TestIndexingPolicy|TestNilIndexingPolicy' -count=1
--- FAIL: TestIndexingPolicyChangesTheRepresentationOctet (0.00s)
    indexing_policy_test.go:69: with Chrome's policy the first octet for :path is 0x44, want 0x04
        (literal WITHOUT indexing, name index 4). SetIndexingPolicy is not reaching the
        representation, so the encoder is emitting the indexed form no browser emits — the exact
        octet patch 6 exists to fix.
    indexing_policy_test.go:74: the policy changed nothing: both encodings are
        44 93 60 6b 77 1d 16 95 63 54 b5 47 59 09 1a 4c 45 8b 52 6c f5.
--- FAIL: TestIndexingPolicyDecidesWhatComesBackAsADynamicIndex/Chrome's:_only_:authority_comes_back_indexed
    indexing_policy_test.go:117: second request, :path: encoded as bf (indexed=true), want indexed=false.
```

Both messages name the octet and the dynamic-table consequence, not merely a mismatch, and both go
green the moment the patch is restored.

### Coverage, before and after

`Encoder.SetIndexingPolicy` is a function this fork ADDED and no fork test had ever executed.

```
before:  git worktree add /tmp/fh14 v0.6.9-sightglass.14 && cd /tmp/fh14 &&
         GOTOOLCHAIN=auto go test ./http2/hpack/ -coverprofile=/tmp/before.out -count=1
after:   GOTOOLCHAIN=auto go test ./http2/hpack/ -coverprofile=/tmp/after.out -count=1

                             before (.14)   after (.15+)
SetIndexingPolicy               0.0%          100.0%
shouldIndex                    80.0%          100.0%
http2/hpack package            89.2%           89.6%
```

The `before` column is the `.14` TAG, checked out, not this tree with the new file moved aside; the
command is written out because the two are not the same measurement and a bare percentage does not
say which one it is. Re-run at `v0.6.9-sightglass.18`: `.14` gives `89.2%`, this tree gives `89.6%`
on two consecutive runs.

0.0% is the number that matters: a consumer's parity guard exercised the seam end to end, and the
fork's own suite never touched it at all.

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

### Ablation

Restoring the eager `identifyDeflate` — the version that drained the whole body with `io.Copy` to
replay the two octets it sniffed — reproduces the deadlock on every shape the guard covers:

```
$ GOTOOLCHAIN=auto go test . -run TestDeflate -count=1 -timeout 120s
--- FAIL: TestDeflateResponseDoesNotParkReadLoop/zlib-wrapped_(RFC_1950) (10.00s)
    transport_deflate_leak_test.go:143: the request never completed (Get "http://127.0.0.1:54737/x":
    context deadline exceeded (Client.Timeout exceeded while awaiting headers)). Before the fix this
    reported "Client.Timeout exceeded while awaiting headers", because readLoop was parked inside
    its own body drain.
--- FAIL: TestDeflateResponseDoesNotParkReadLoop/raw_DEFLATE_(RFC_1951) (10.01s)
--- FAIL: TestDeflateShortBodyDoesNotParkReadLoop/one_octet (10.00s)
    transport_deflate_leak_test.go:182: the request never completed (Get "http://127.0.0.1:54737/x":
    context deadline exceeded (Client.Timeout exceeded while awaiting headers)); a body too short to
    sniff must not park readLoop either
--- FAIL: TestDeflateShortBodyDoesNotParkReadLoop/two_octets,_not_a_zlib_header (10.00s)
--- FAIL: TestDeflateBodyClosedWithoutReadingDoesNotPanic (10.01s)
--- FAIL: TestDeflateLeavesNoParkedGoroutine (5.00s)
FAIL	github.com/Berserk-Automation-Hub/fhttp	55.526s
```

Six failures, all of them the deadlock rather than a difference: the caller sees the slow-origin
symptom `Client.Timeout exceeded while awaiting headers` because the readLoop goroutine is parked on
a channel only readLoop can close.

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
request HEADERS blocks, as the last field, and on **0 of 744** captured HTTP/1.1 requests — 722 over
TLS with forced ALPN plus 22 to a flag-free `http://localhost` origin. It has no HTTP/1.1 form at
all. Without this key a request built once and sent over whichever protocol the origin offers either
loses `priority` on HTTP/2 or invents it on HTTP/1.1.

**Re-derived, not remembered.** Those two numbers were carried unchecked through five tags under a
note that no capture was reachable, which was false. They are now reproduced exactly by
`groundtruth/tools/derive_hpack_census.py`; the scope, the command and the attribution rule are in
the Ground truth section below.

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

### Ablation

Dropping the two omission loops in `Header.writeSubset` — the ordered path and the unordered one —
so that `HTTP1OmitKey` is still excluded from the wire as a key but no longer omits anything:

```
$ GOTOOLCHAIN=auto go test . -run TestHTTP1OmitKey -count=1
--- FAIL: TestHTTP1OmitKeyDropsOnlyOnHTTP1 (0.00s)
    header_http1omit_test.go:63: priority survived on the HTTP/1.1 wire despite HTTP1OmitKey:
        GET /x HTTP/1.1
        Accept: */*
        Host: example.invalid
        Priority: u=0, i
        User-Agent: probe
--- FAIL: TestHTTP1OmitKeyIsCaseInsensitive (0.00s)
    header_http1omit_test.go:91: header "Priority" was not dropped by omit name "priority":
--- FAIL: TestHTTP1OmitKeyWorksWithHeaderOrder (0.00s)
    header_http1omit_test.go:111: priority survived the ordered path:
```

The message prints the wire bytes with `Priority:` on them, which is the defect — a header with no
HTTP/1.1 form, invented on an HTTP/1.1 request — and not merely a mismatch.

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
`User-Agent`; the magic key never reaches the wire on either the ordered or the unordered path. The
HTTP/2 sites are proven end-to-end on a real h2 loopback origin by Sightglass's
`parity.TestParityCallerStatedBlockIsExactOnH2`.

### Ablation

Removing the gate from the HTTP/1.1 User-Agent site in `request.go` (`uaCap == nil && uaLow == nil`,
i.e. upstream):

```
$ GOTOOLCHAIN=auto go test . -run TestNoAutoHeadersKey -count=1
--- FAIL: TestNoAutoHeadersKeySuppressesTheInjectedUserAgent (0.00s)
    no_auto_headers_test.go:40: NoAutoHeadersKey did not suppress the injected User-Agent:
        GET /x HTTP/1.1
        Host: example.invalid
        X-One: 1
        Accept: text/plain
        User-Agent: Go-http-client/1.1
```

The failure prints the leak itself: a caller that asked for three headers got four, and the fourth
names the library. For a caller emitting a byte-exact browser ClientHello that is the loudest
possible tell.

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
| Firefox 156 | last match | 5 | 3 | 41 of 41 attributed HEADERS blocks (`:path` index 5 on all 41; `:method` index 3 on both of its 2); leading byte `0x05` where a first-match encoder emits `0x04` |

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
client RST_STREAM frames on 6 DISTINCT streams across both captures, never two on one stream,
against 109 streams the server ended with END_STREAM — and a second reset on a stream the peer has
already closed is trivially loggable by any origin.

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

**RETRACTED, FALSE — "no stream-map leak resulted from the old code, because `Close()` calls
`forgetStreamID` unconditionally".** That sentence was published in `v0.6.9-sightglass.9` and `.11`,
in this entry and in the commit message beside it, and it is FALSE. It holds for exactly one of the
two paths that can end a stream — `transportResponseBody.Close()` — and the operator who wrote it
never checked the other one.

`cc.forgetStreamID` sits inside the very same `if` as the reset. So on the CONTEXT-CANCEL path —
`awaitRequestCancel -> cancelStream()`, where the caller cancels the request with the body still
open, `transportResponseBody.Close()` never runs, and `cancelStream()` is the only code that can
release the stream — the inverted condition wrote no `RST_STREAM` **and** called no
`cc.forgetStreamID`. The `clientStream` stayed in `cc.streams` for the life of the connection,
holding its accounting and one of the connection's 100 concurrency slots. The one-character fix
closes the leak as well as the duplicate reset; both branches are restored by the same negation.

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

The `if` this patch negates contains TWO statements, and the second one — `cc.forgetStreamID(cs.ID)`
— is not one effect but four. `forgetStreamID` is `streamByID(id, true)`, which deletes the map
entry, closes `cs.done`, calls `cc.cond.Broadcast()` and updates `cc.lastActive` /
`cc.idleTimer.Reset(cc.idleTimeout)` / `cc.lastIdle`. Three guards cover them, and each of the four
ablations below breaks a different subset:

| guard | what it pins |
| --- | --- |
| `TestCancelStreamResetsOnlyWhenNotAlreadyReset` | the condition: exactly one `RST_STREAM`, and only when not already reset |
| `TestCancelStreamForgetsTheStream` | the map entry and `close(cs.done)` |
| `TestCancelStreamWakesTheSlotWaiter` | `cc.cond.Broadcast()`, `cc.lastActive`, `cc.lastIdle` and the idle-timer re-arm |

**Ablation 1 — restore the inverted condition** (`if didReset`). It fails in **both** directions,
which is the point, and it takes the bookkeeping down with it:

```
$ GOTOOLCHAIN=auto go test ./http2/ -run TestCancelStream -count=1
--- FAIL: TestCancelStreamResetsOnlyWhenNotAlreadyReset (2.03s)
    cancel_stream_reset_test.go:276: didReset=false: peer received 0 RST_STREAM, want 1 — a cancelled
    stream that has not been reset must be reset once
    cancel_stream_reset_test.go:281: didReset=true: peer received 1 RST_STREAM, want 0 — the stream
    was already reset, so this is the duplicate Chrome never sends (measured 6/6 streams, one reset each)
--- FAIL: TestCancelStreamForgetsTheStream (2.00s)
    cancel_stream_reset_test.go:306: didReset=false: the clientStream was still in cc.streams after
    cancelStream() — cc.forgetStreamID sits inside the same `if` as the reset, so an inverted
    condition leaks one clientStream and one concurrency slot per cancelled request, for the life of
    the connection
--- FAIL: TestCancelStreamWakesTheSlotWaiter (2.03s)
    cancel_stream_reset_test.go:347: didReset=false: a goroutine parked in cc.cond.Wait() waiting for a concurrency slot was NEVER WOKEN within 2s after cancelStream() released the stream.
```

**Ablation 2 — delete only `cc.forgetStreamID(cs.ID)`,** leaving `cc.writeStreamReset` in place, so
the frame half of the `if` is untouched and only the leak is reintroduced. This is the ablation the
RETRACTED "stream-map leak" sentence above never had:

```
--- FAIL: TestCancelStreamForgetsTheStream (2.00s)
    cancel_stream_reset_test.go:306: didReset=false: the clientStream was still in cc.streams after
    cancelStream() — cc.forgetStreamID sits inside the same `if` as the reset, so an inverted
    condition leaks one clientStream and one concurrency slot per cancelled request, for the life of
    the connection
    cancel_stream_reset_test.go:316: didReset=false: cs.done was still OPEN after cancelStream()
    returned. Releasing the stream means cc.forgetStreamID, which closes cs.done and broadcasts on
    cc.cond as well as deleting the map entry
--- FAIL: TestCancelStreamWakesTheSlotWaiter (4.00s)
    cancel_stream_reset_test.go:347: didReset=false: a goroutine parked in cc.cond.Wait() waiting for a concurrency slot was NEVER WOKEN within 2s after cancelStream() released the stream.
    cancel_stream_reset_test.go:357: didReset=false: cc.lastActive was still the zero time after cancelStream().
    cancel_stream_reset_test.go:364: didReset=false: cc.lastIdle was still the zero time after cancelStream() emptied cc.streams.
    cancel_stream_reset_test.go:370: didReset=false: cc.idleTimer never fired after cancelStream() emptied cc.streams, although the harness armed it with a 25ms idleTimeout and waited 2s.
```

**RETRACTED, FALSE — the ablation record this entry published for ablation 2.** `v0.6.9-sightglass.9`
through `.18` printed it as a single error at line 161 of `cancel_stream_reset_test.go`. Two things
were wrong with that: the assertion had moved, and `.18` had added a second error arm the block
never showed. Nobody could reproduce the text as published.
`TestPatchesMDCitesNoLineNumbersIntoThisTree` waved it through because 161 was still *inside* the
file — its fenced-block exemption doing exactly what its own comment warns about — so
`TestPatchesMDQuotedAblationOutputIsReproducible` now requires every quoted `file.go:NNN:` in this
document to land on a testing call that really prints the quoted text.

**Ablation 3 — replace `cc.forgetStreamID(cs.ID)` with `cc.mu.Lock(); delete(cc.streams, cs.ID);
cc.mu.Unlock()`.** An adversarial reader wrote this: the slot really is freed, so a map-only guard
passes and so does the Sightglass parity guard that counts free concurrency slots. Output is
identical to ablation 2's, because deleting the entry by hand is exactly what dropping the call did
to the map.

**Ablation 4 — replace it with a partial that does MORE**, and still not everything:

```go
cc.mu.Lock()
if cs2 := cc.streams[cs.ID]; cs2 != nil && !cc.closed {
	delete(cc.streams, cs.ID)
	close(cs2.done)
}
cc.mu.Unlock()
```

This drops exactly three things: `cc.cond.Broadcast()`, `cc.lastActive` and the idle-timer re-arm. It
passed BOTH layers at `v0.6.9-sightglass.18` — the whole fork `http2` suite and
`parity.TestParityHTTP2CancelledStreamsDoNotConsumeSlots` on Sightglass's shipped path — which is
why `TestCancelStreamWakesTheSlotWaiter` exists. `awaitOpenSlotForRequest` parks in `cc.cond.Wait()`
and NOTHING but `cc.cond.Broadcast()` wakes it, so a request already waiting for one of Chrome's 100
slots on a saturated connection is never told when a context-cancelled stream frees one: the same
permanent wedge this patch exists to close, reached from the waiter's side. The idle-timer half is a
second leak — a connection whose last stream was CANCELLED never re-arms its reaping timer.

```
--- FAIL: TestCancelStreamWakesTheSlotWaiter (4.01s)
    cancel_stream_reset_test.go:347: didReset=false: a goroutine parked in cc.cond.Wait() waiting for a concurrency slot was NEVER WOKEN within 2s after cancelStream() released the stream. cc.forgetStreamID ends in cc.cond.Broadcast(), and that broadcast is the ONLY thing that wakes awaitOpenSlotForRequest.
    cancel_stream_reset_test.go:357: didReset=false: cc.lastActive was still the zero time after cancelStream(). cc.forgetStreamID sets it, and ClientConn's idle accounting (tooIdleLocked, http2ClientConnPool reuse, httptrace's GotConn.IdleTime) reads it.
    cancel_stream_reset_test.go:364: didReset=false: cc.lastIdle was still the zero time after cancelStream() emptied cc.streams. tooIdleLocked() returns false while lastIdle is zero, so a connection whose last stream was CANCELLED is never judged too idle and is handed to new requests for ever.
    cancel_stream_reset_test.go:370: didReset=false: cc.idleTimer never fired after cancelStream() emptied cc.streams, although the harness armed it with a 25ms idleTimeout and waited 2s.
```

`TestCancelStreamWakesTheSlotWaiter` also asserts the NEGATIVE: with `didReset=true` nothing is
released, so the parked goroutine must STILL be parked when the deadline expires and the idle timer
must NOT fire. Without that half, a harness that woke its waiter for any reason at all would look
like a guard — which is how ablations 3 and 4 got through in the first place.

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

Coverage, for the guards added at `v0.6.9-sightglass.19`. The whole `http2` package is **89.8%
before and 89.8% after** — this tag adds a guard, not product code, and the fork's full suite already
reached the statements it newly asserts. The number that moves is what patch 11's OWN guards reach,
run alone (`go test ./http2/ -run TestCancelStream -count=1 -coverprofile`):

```
                       v0.6.9-sightglass.18   v0.6.9-sightglass.19
cancelStream                    100.0%                100.0%
forgetStreamID                  100.0%                100.0%
streamByID                       85.7%                100.0%
```

The two statements that went from 0 to 1 are `cc.idleTimer.Reset(cc.idleTimeout)` and
`cc.lastIdle = time.Now()` — the idle-timer re-arm, which patch 11's guards had **never once
executed**. That is not a coincidence: it is exactly the effect ablation 4 drops, and a profile
saying `count 0` is what an unguarded element looks like before anyone writes the guard.

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

Chrome 153 never resets a stream that ended: its 6 RST_STREAM frames across both ground-truth
captures are on 6 distinct streams it abandoned, and the 109 streams the server ended with
END_STREAM carry none. A reset on a completed stream is as loggable as the duplicate patch 11
removed.

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

## Ground truth

Every count in this file about what a browser puts on the wire is RE-DERIVED, from captures in this
repository's sibling `groundtruth/out`, and none of it is quoted from memory or from a vendor
constant (HR-1, HR-3).

**What was wrong here, stated precisely, because the first attempt at correcting it overshot.**
Revisions up to `v0.6.9-sightglass.14` carried these counts with a note that no capture was reachable
from this machine. That note was FALSE: `groundtruth/out` is 974 MB — nine Chrome 153.0.8010.37
captures and one Firefox 156.0, each with `capture.pcapng` + `keys.keylog` + `netlog.json` — and
`groundtruth/README.md` documents the pipeline. So the counts were UNVERIFIED, which is the defect
and is real. They were not WRONG: re-derived, every one of them reproduces exactly, and the only
figure that moves is the one that was written as an estimate in the first place ("~94 completed
streams"; measured, 109).

```
command:  python3 groundtruth/tools/derive_hpack_census.py groundtruth/out
decoder:  tshark 4.4.8, decrypting with the browser's OWN SSLKEYLOGFILE
```

**The census scope, which the earlier revisions never named — which is exactly why nobody could
check them.** The Chrome HTTP/2 figures are the two captures that carry HTTP/2 to real origins,
`chrome-153.0.8010.37/smoke-20260915T174204Z` (19 request HEADERS blocks) and
`chrome-153.0.8010.37/h3-depth-20260915T181108Z` (100); the HTTP/1.1 figures are
`h1-cookies-20260915T215150Z` (722 requests over TLS with forced ALPN) plus
`h1-localhost-20260915T221335Z` (22 to a flag-free `http://localhost` origin). Naming that capture
rather than "one `h1-localhost-*`" matters even though the answer is the same either way: of the
three, `…221335Z` and `…221411Z` each carry 22 and `…221253Z` carries 0, so an unnamed scope was
reproducible by luck and not by construction. Sightglass's own
`go/tlsemu/testdata/PARITY_MATRIX.md` rows H1-4 and H2-15 quote the same scope, so the two documents
now agree by construction rather than by coincidence. The `h1-depth-*` captures carry a little
HTTP/2 as well, and counting them is a DIFFERENT census; it is the right-hand column below, so that
a reader who re-runs the tool over the whole directory and gets larger numbers is not misled into
thinking one of us is wrong.

**Attribution (HR-3).** The keylog was written by the browser that was under capture, so a TLS
session that decrypts with it is that browser's. A session tshark cannot decrypt produces no
`http2.header` field at all, so every observation is the browser's by construction rather than by
heuristic — the capture runs on a real NIC and carries every other process on the machine.

| fact | Chrome 153, CENSUS SCOPE | Chrome 153, all 9 captures | Firefox 156 |
|---|---|---|---|
| client HTTP/2 request HEADERS blocks | **119** | 135 | 41 |
| ...carrying `priority:` | **112** | 128 | 11 |
| HTTP/1.1 requests observed | **744** | 930 | 1 |
| ...carrying `priority` | **0** | 0 | 0 |
| request header fields observed | **2513** | 2729 | 471 |
| ...emitted "Never Indexed" (`0x1x`) | **0** | 0 | 0 |
| `:path` literal-with-indexed-name | **114, all name index 4** | 130, all index 4 | 41, all index 5 |
| `:method` literal-with-indexed-name | **8, all name index 2** | 8, all index 2 | 2, all index 3 |
| `:path` emitted with incremental indexing | **0** | 0 | 0 |
| `:authority` emitted with incremental indexing | **45** | 53 | 13 |
| client `RST_STREAM` frames / distinct streams | **6 / 6** | 6 / 6 | 0 / 0 |
| streams the server ended with END_STREAM | **109** | 125 | 27 |

Patch 1's HEADERS-priority ground truth is separate and is pinned as JSON rather than prose:
`go/tlsemu/testdata/chrome152_h2_priority.json`, 12 observations, NetLog HEADERS-priority joined per
stream with that stream's own `priority:` header.


## Regression diff

Upstream v0.6.9 is **not green**, so "the suite passes" is not an available bar and this file never
claims it. The bar is a regression diff: the same command, on the same machine, against the published
tag and against this tree, reporting only failures that are NEW here.

Per-patch entries above quote the diff measured on the day that patch landed, under whatever else
this machine was running; those numbers are not comparable with one another and must not be read as a
running total. The measurement for the tree as it stands is here, and it is the one to trust.

<!-- REGRESSION-DIFF-BEGIN — patches_doc_test.go checks that this block exists, that it names a tag
     on both its `before:` and `after:` lines, that the `before:` tag is a real tag and an ancestor of
     HEAD, and that the `after:` tag either IS this tree or does not exist yet (the tree waiting to be
     tagged as it). Up to and including v0.6.9-sightglass.14 this comment claimed the tag check and
     there was none: .14 shipped numbers measured at .12 and .13 under a line reading
     "after: v0.6.9-sightglass.12, this tree". TestPatchesMDRegressionDiffIsForTHISTree exists so that
     cannot happen again. -->
```
command: GOTOOLCHAIN=auto go test ./... -count=1 -timeout 60m
         same machine, one run after the other, never concurrently
before:  v0.6.9-sightglass.18 in a clean worktree of the published tag (2d9f1b6)
after:   v0.6.9-sightglass.19, this tree

before:  36 failing tests   root package 664.423s   whole run 665.58s wall
after:   35 failing tests   root package  54.134s   whole run  99.96s wall

              Each side was measured TWICE, on different occasions in the same session, and each
              side's sorted `--- FAIL` list is IDENTICAL to its own other run — 36 names both times
              on the before side, 35 names both times on the after side. The numbers above are the
              second pair, run one after the other with a 150s gap.

NEW failures: NONE. The lists are diffed as LISTS, never as totals: `comm -13 before after` is
              EMPTY on both pairings.
              `comm -23` is ONE name, TestOmitHTTP2, and it is not this fork's to fail. Two
              independent facts say so, and neither is an opinion about load:
                (1) `git diff v0.6.9-sightglass.18 HEAD -- '*.go' ':!*_test.go'` is EMPTY. This tag
                    changes no product code at all; it adds one test and rewrites prose.
                (2) TestOmitHTTP2's body is
                    `exec.Command(goTool, "test", "-short", "-tags=nethttpomithttp2", "net/http")`.
                    It runs the STANDARD LIBRARY's net/http suite in a subprocess. Nothing in this
                    tree is an input to it.
              Run alone rather than after the rest of the root package, it PASSES on BOTH sides:
              `go test . -run '^TestOmitHTTP2$' -count=1` is `ok 1.765s` at
              v0.6.9-sightglass.18 and `ok 1.689s` here. It fails only as the last act of a full
              root-package run, when that run has exhausted the ephemeral port range — the
              subprocess reports `dial tcp 127.0.0.1:59583: connect: can't assign requested
              address` and then sits until its own 10-minute timeout, which is where the 664s root
              package comes from. That is an INTERACTION with the suite around it, the same class
              as the TestMissingStatusNoPanic outcome recorded below, and it is why this block
              diffs names instead of counting them.
FIXED by these tags: none. .12 through .19 change no product code — every change to a non-test .go
              file since .11 is a comment or a [SIGHTGLASS PATCH n] marker, which `git diff
              v0.6.9-sightglass.11 HEAD -- '*.go' ':!*_test.go'` shows directly, and for .19 alone
              that diff is empty.

Packages: fhttp, fhttp/http2 and fhttp/httputil FAIL on both sides; cgi, cookiejar, fcgi,
http2/h2c, http2/hpack, httptest, httptrace, internal, internal/profile and pprof are ok on
both. Neither run contains one `--- SKIP` line or one [build failed]: `grep -c '^--- SKIP'` is 0
and `grep -c 'build failed'` is 0 on both transcripts. A package whose tests are skipped wholesale
is not a passing package, so that is checked rather than assumed.

HOW THE `after` COUNT WAS TAKEN, because the guard above is part of the suite it measures. This
tree was run with this block already re-tagged to `before: v0.6.9-sightglass.18` /
`after: v0.6.9-sightglass.19` and the numeric fields still holding placeholders, and the doc guards
did not object: TestPatchesMDRegressionDiffIsForTHISTree checks the TAGS, not the numbers, and the
`after:` line named a tag that did not exist yet, which is the one state in which the file may name
its own tag before it is cut. So the 35 above is the whole suite with no known-red guard in it, and
filling in these numbers afterwards changes no test's outcome. (Up to .14 that was not true: the
block claimed a tag check that did not exist and shipped numbers measured at .12 and .13.)

RETRACTED, FALSE: "not a hang, and not a larger failure set. With 25m the package completes."
Revisions of this file up to and including v0.6.9-sightglass.14 said that about the root package,
and it is FALSE. Running the command that sentence prescribes, at .14, in a clean tree, an
adversarial reader got:

    panic: test timed out after 25m0s
            running tests:
                    TestMissingStatusNoPanic (24m8s)
    FAIL    github.com/Berserk-Automation-Hub/fhttp  1500.328s

It IS a hang, it is NOT TestOmitHTTP2 (which never ran in that run at all), and the "36/35" counts
that revision published were partial totals truncated by that panic. What actually happens is
LOAD-DEPENDENT and has three observed outcomes on this machine:

  root package 655-660s, 36 failing — TestOmitHTTP2 runs its subprocess suite into that
                                     subprocess's own 10-minute timeout and FAILS. Observed twice
                                     in one session: 655.265s at .11 and 659.742s at .18. Those
                                     are the two sides of the pair above, and they are what makes
                                     that pair comparable.
  root package ~51-55s, 35 failing — TestOmitHTTP2 PASSES and the package finishes in under a
                                     minute. THE PUBLISHED NUMBER FOR THIS OUTCOME IS RETRACTED,
                                     not the outcome: revisions .15 .. .17 rendered ONE run three
                                     ways — "51.329s" on the `after:` line, "55.0s" ten lines
                                     below it, "51.3 s" in PROGRAMME F67 — and attributed it to
                                     three different tags (.15, .16, .16). One measurement cannot
                                     have three values, so none of the three is quoted as a
                                     measurement any more. That the outcome exists is not in
                                     doubt and does not rest on that run: the root package run
                                     ALONE passed TestOmitHTTP2 twice, at 50.7s and 60.8s, which
                                     is what patch 4's entry cites.
  root package 1500.3s, PANIC      — TestCancelRequestWhenSharingConnection exhausts the ephemeral
                                     range, and TestMissingStatusNoPanic, which runs next, blocks
                                     24m8s in `<-done` while its own listener sits in Accept: the
                                     client's proxy dial never reaches it. Run alone that test
                                     passes in 0.00s, so it is an INTERACTION, not a broken test.
                                     Observed at .14.

All three are the same root cause and it is OURS, not the environment: patch 4's detached H1 dial
spends a full connect and leaves a TIME_WAIT port per cancelled request (see "What it costs,
measured" under patch 4). It is ledger T0525, open and raised to high, and the 24-minute wedge is
recorded there — patch 4's socket cost can stall the whole root package, not merely fail two tests.
Use `-timeout 60m` so that when the wedge happens the package still finishes rather than being
killed mid-run and reporting a partial count as a total.

AND THE DIFFERENCE AGAINST UPSTREAM IS OURS. Against pristine v0.6.9 run on the same
toolchain (`GOTOOLCHAIN=go1.27.0`, so the comparison is source and not language version)
this fork fails exactly two tests upstream does not, and one of them is attributed and one
is not:

    fork v0.6.9-sightglass.12                    36 failing   root package 654.3 s
    the same tree minus patch 4's WithoutCancel  34 failing   root package  56.8 s
    pristine v0.6.9 @ GOTOOLCHAIN=go1.27.0       36 failing   root package  60.1 s

  TestCancelRequestWhenSharingConnection — ATTRIBUTED to patch 4 and deterministic:
      6 runs of this tree, 6 failures; 4 runs with patch 4's one expression reverted,
      4 passes; pristine, pass. It failed in both runs above (7.03 s at .15).

  TestOmitHTTP2 — SUGGESTIVE, not attributed. It shells out to a second full net/http
      suite and fails on the same ephemeral-port exhaustion. It failed in the .11 run
      above and PASSED in the .15 one, on a tree whose product code is identical, which
      is what "load-dependent" means and why this file attributes it to nothing.

  It is NOT our added tests. Skipping every test in the four files this fork adds to
      the root package — header_http1omit_test.go, no_auto_headers_test.go,
      patches_doc_test.go, transport_deflate_leak_test.go — leaves TestOmitHTTP2 at
      601.55 s and still failing. (Revisions up to v0.6.9-sightglass.17 said "the nine
      files": nine is every test file this fork adds, but five of them are in http2/ and
      http2/hpack/ and cannot affect this package's run at all.)

THE COUNT IS NOT STABLE ACROSS RUNS and this file does not claim it is. Observed on this line of
history: 36, 36, 35, 35, 36 — the 35s are the runs where TestOmitHTTP2 passed, and 34 with patch 4
reverted. The invariant a regression diff needs does hold, and at .18 it holds in its strongest
form: no run of .12 .. .18 has produced a failure that .11 did not, and the .11/.18 pair above is
identical failure for failure. (Revisions up to .17 wrote this sentence as ".12 .. .15" while the
tree was at .17, so the span excluded the two most recent tags it was meant to cover.)

Two upstream failures this fork FIXES, for the same reason the suite is kept:
`TestCompressionDeflate` (patch 7 — upstream's own test for raw DEFLATE, which it fails) and
`TestDescriptions` (patch 4b's `goroutineleak` profile entry — pristine's `pprof` package
fails; ours is ok).
```
<!-- REGRESSION-DIFF-END -->

---

## Maintenance

**RETRACTED, HARMFUL: the instruction that used to sit here.** Up to and including
`v0.6.9-sightglass.11` this section read "re-copy upstream, strip tests, re-apply the hunks above
(four for patch 1, one for patch 2, three for patch 3, three for patch 4, two for patch 5)". Three
separate things about it were FALSE or HARMFUL: a re-copy reverts the rewritten module path,
Upstream's 73 test files are KEPT, and the section sat in the MIDDLE of the file so "above" excluded
six of the fourteen entries. Each is RETRACTED in full here:

* RETRACTED, HARMFUL — "strip tests". Upstream's 73 test files are KEPT and are the reason patch 4b
  exists. Stripping them is how the CONNECT leak survived patch 4 in the first place.
* RETRACTED, HARMFUL — "re-copy upstream". This is a fork with a rewritten module path across 86
  files. A copy reverts the rewrite, and the tree then fails to compile against `go/go.mod`.
* RETRACTED, HARMFUL — "re-apply the hunks above". The section sat in the MIDDLE of the file, so
  "above" excluded patches 4b, 6, 7, 8, 8b and 10: six of the fourteen entries, verified by listing
  the `## ` headings of the published `v0.6.9-sightglass.11` file (`git show 895d5a8:PATCHES.md`),
  where `## Maintenance` is at line 609 and those six are the only sections below it. It now sits at
  the end, and there is no "above" left to get wrong.

### On an upstream bump

1. `git fetch https://github.com/bogdanfinn/fhttp --tags` and **merge** the new tag into the
   `sightglass` branch. Do not copy a tree over this one.
2. Rewrite the module path in any file upstream ADDED:
   `github.com/bogdanfinn/fhttp` → `github.com/Berserk-Automation-Hub/fhttp`, and
   `github.com/bogdanfinn/utls` → `github.com/Berserk-Automation-Hub/utls`.
3. **Keep every `*_test.go`, `export_test.go` and `testdata/`.** If a merge conflict tempts you to
   drop one, resolve it instead.
4. Resolve conflicts against the markers, not against this file's prose. Every site of every patch
   carries a numbered `[SIGHTGLASS PATCH n]` comment — all 24 files that carry a numbered marker,
   including the six upstream files patch 4b edits and eight of the nine guards this fork adds
   (`patches_doc_test.go` guards this document rather than a numbered patch, so its marker carries no
   number) — so `grep -rln 'SIGHTGLASS PATCH' --include='*.go' .` lists all 25 marked files: those 24
   plus `patches_doc_test.go`. `patches_doc_test.go` derives both counts from the tree, so neither
   can drift; up to `v0.6.9-sightglass.17` this step said "all 24 marked files … and the nine guards"
   and sent the maintainer to a grep that returns 25.
   `patches_doc_test.go` fails if a patch documented here has no marker in the tree, if a marker in
   the tree has no entry here, **or if a marker names a patch the "Files touched" manifest does not
   list beside that same file**. That last one is new in `v0.6.9-sightglass.15` and it is the check
   this step depends on: before it, the patch-5 marker in `http2/client_conn_pool.go` and the patch-9
   marker in `http2/transport.go` could be swapped and every guard stayed green, which would send you
   to the wrong section of this file from the right line of code.
5. Update the base commit, the file manifest, the per-file patch attribution in it, and the counts.
   `patches_doc_test.go` re-derives all of them from `git diff` and from the markers, and fails if
   you do not.
6. Re-run, in this order:
   * `GOTOOLCHAIN=auto go test ./... -count=1 -timeout 60m` here, and diff the failure set against the
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
