package http2

// PER-REQUEST HEADERS-EMBEDDED PRIORITY (Sightglass additive patch, not upstream fhttp).
//
// WHY THIS FILE EXISTS. Upstream fhttp v0.6.8 carries the HTTP/2 HEADERS-embedded PRIORITY on the
// TRANSPORT: Transport.HeaderPriority (transport.go:94) is dereferenced inside
// ClientConn.writeHeaders, which runs on the connection's write path under cc.wmu. Consequences:
//
//	(1) one *Transport emits ONE weight for every stream it ever opens, and
//	(2) mutating the shared *PriorityParam between requests is a DATA RACE, so there is no
//	    "just change it before each Do()" workaround.
//
// Genuine Chrome does the opposite: it derives each stream's weight from THAT request's own RFC-9218
// urgency and multiplexes mixed urgencies on ONE connection. Ground truth (go/tlsemu/testdata/
// chrome152_h2_priority.json, joined NetLog HEADERS-priority x `priority:` header, 12/12 match):
//
//	accounts.google.com   stream 1  weight 110  u=4     <- POST /ListAccounts (fetch)
//	accounts.google.com   stream 3  weight 256  u=0     <- GET /v3/signin/identifier (navigate)
//	accounts.google.com   stream 5  weight 256  u=0     <- GET /_/bscframe
//	                      ^^^ three different streams, ONE connection, TWO different weights
//
// So a Sightglass client that mixes urgencies had to open one connection per urgency class, which
// makes the CONNECTION COUNT itself diverge from Chrome — the divergence this patch removes.
//
// THE MECHANISM: a request carries its priority in its own context.Context. The context is chosen
// over an http.Header sentinel (fhttp's HeaderOrderKey style) deliberately: a header sentinel has to
// be filtered out again in encodeHeaders, and anything that fails to filter it emits a NOVEL header
// on the wire — strictly worse than the bug being fixed. A context value cannot reach the wire.
//
// Nothing changes for a caller that does not set one: RequestPriority returns ok=false and
// writeHeaders falls back to Transport.HeaderPriority exactly as before, so every existing
// single-urgency client is byte-identical to pre-patch behaviour.

import (
	"context"

	http "github.com/Berserk-Automation-Hub/fhttp"
)

// perRequestPriorityKey is the unexported context key. Unexported so nothing outside this package
// can collide with it or forge a value of a different type.
type perRequestPriorityKey struct{}

// WithRequestPriority returns a context that makes the HTTP/2 client stamp exactly this
// HEADERS-embedded PRIORITY on the request sent with it, overriding Transport.HeaderPriority for
// that one stream. Attach it with req.WithContext(...).
//
// The caller is responsible for keeping the value consistent with the request's own `priority:`
// header — in Sightglass that is enforced by deriving BOTH from one urgency
// (tlsemu.ApplyChromePriority), so the frame and the header cannot drift.
func WithRequestPriority(ctx context.Context, p PriorityParam) context.Context {
	return context.WithValue(ctx, perRequestPriorityKey{}, p)
}

// RequestPriority reports the per-request HEADERS priority carried by ctx, if any.
func RequestPriority(ctx context.Context) (PriorityParam, bool) {
	p, ok := ctx.Value(perRequestPriorityKey{}).(PriorityParam)
	return p, ok
}

// requestHeaderPriority resolves the priority for one outgoing request: the per-request value if the
// caller set one, else nil (meaning "fall back to Transport.HeaderPriority", i.e. upstream
// behaviour). It never returns a zero-value PriorityParam by accident — a zero PriorityParam is a
// LEGAL frame (weight 1, non-exclusive, dep 0) that Chrome never sends, so distinguishing "unset"
// from "set to zero" matters.
func requestHeaderPriority(req *http.Request) *PriorityParam {
	if req == nil {
		return nil
	}
	ctx := req.Context()
	if ctx == nil {
		return nil
	}
	if p, ok := RequestPriority(ctx); ok {
		return &p
	}
	return nil
}
