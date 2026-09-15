// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Transport code's client connection pooling.

package http2

import (
	"sync"

	tls "github.com/bogdanfinn/utls"

	http "github.com/bogdanfinn/fhttp"
)

// ClientConnPool manages a pool of HTTP/2 client connections.
type ClientConnPool interface {
	GetClientConn(req *http.Request, addr string) (*ClientConn, error)
	MarkDead(*ClientConn)
}

// clientConnPoolIdleCloser is the interface implemented by ClientConnPool
// implementations which can close their idle connections.
type clientConnPoolIdleCloser interface {
	ClientConnPool
	closeIdleConnections()
}

var (
	_ clientConnPoolIdleCloser = (*clientConnPool)(nil)
	_ clientConnPoolIdleCloser = noDialClientConnPool{}
)

// TODO: use singleflight for dialing and addConnCalls?
type clientConnPool struct {
	t *Transport

	mu sync.Mutex // TODO: maybe switch to RWMutex
	// TODO: add support for sharing conns based on cert names
	// (e.g. share conn for googleapis.com and appspot.com)
	conns        map[string][]*ClientConn // key is host:port
	dialing      map[string]*dialCall     // currently in-flight dials
	keys         map[*ClientConn][]string
	addConnCalls map[string]*addConnCall // in-flight addConnIfNeede calls
}

func (p *clientConnPool) GetClientConn(req *http.Request, addr string) (*ClientConn, error) {
	return p.getClientConn(req, addr, dialOnMiss)
}

const (
	dialOnMiss   = true
	noDialOnMiss = false
)

// shouldTraceGetConn reports whether getClientConn should call any
// ClientTrace.GetConn hook associated with the http.Request.
//
// This complexity is needed to avoid double calls of the GetConn hook
// during the back-and-forth between net/http and x/net/http2 (when the
// net/http.Transport is upgraded to also speak http2), as well as support
// the case where x/net/http2 is being used directly.
func (p *clientConnPool) shouldTraceGetConn(st clientConnIdleState) bool {
	// If our Transport wasn't made via ConfigureTransport, always
	// trace the GetConn hook if provided, because that means the
	// http2 package is being used directly and it's the one
	// dialing, as opposed to net/http.
	if _, ok := p.t.ConnPool.(noDialClientConnPool); !ok {
		return true
	}
	// Otherwise, only use the GetConn hook if this connection has
	// been used previously for other requests. For fresh
	// connections, the net/http package does the dialing.
	return !st.freshConn
}

// PATCH 3 (Sightglass) — the H2 connect must honour the REQUEST's context.
//
// Upstream x/net/http2 threads a context all the way into the dial
// (dialClientConn(ctx, addr, singleUse), client_conn_pool.go@v0.48.0), so a cancelled request always
// unblocks: its dial dies and `call.done` closes. This fork carries the pre-2021 pool — the dial
// takes no context — and the SHIPPED stack makes that unbounded, because tls-client v1.15.1 hands
// http2.Transport a legacy, context-free DialTLS hook that dials with context.Background()
// (roundtripper.go:483 dialTLSHTTP2). So a waiter blocked on `<-call.done` waited FOREVER whenever an
// origin stopped completing TLS handshakes: past the request's own deadline, past Client.Timeout,
// holding a goroutine and a socket each time. PROVEN by parity.TestParityH2DialHonoursRequestContext
// (loopback origin that serves one h2 conn then stalls every later one): PRE this patch the second
// request never returned inside 20 s under a 2 s client timeout; POST it returns in ~2 s.
//
// The dial itself is deliberately NOT abandoned — it has no context to cancel, and cancelling it is
// also the wrong answer (see the round-1 H1 fix in ../transport.go): it runs to completion and its
// connection joins the pool for a later request, so no socket is wasted. Nothing on the wire changes.
func (p *clientConnPool) getClientConn(req *http.Request, addr string, dialOnMiss bool) (*ClientConn, error) {
	if isConnectionCloseRequest(req) && dialOnMiss {
		// It gets its own connection.
		traceGetConn(req, addr)
		const singleUse = true
		// Same defect, same fix: this dial is uncancellable too, so run it off the caller's
		// goroutine and let the caller leave on its own context. A single-use conn joins no pool, so
		// an abandoned one is CLOSED rather than leaked.
		type singleUseDial struct {
			cc  *ClientConn
			err error
		}
		ch := make(chan singleUseDial, 1)
		go func() {
			cc, err := p.t.dialClientConn(addr, singleUse)
			ch <- singleUseDial{cc, err}
		}()
		select {
		case r := <-ch:
			if r.err != nil {
				return nil, r.err
			}
			return r.cc, nil
		case <-req.Context().Done():
			go func() {
				if r := <-ch; r.cc != nil {
					r.cc.Close()
				}
			}()
			return nil, req.Context().Err()
		}
	}
	p.mu.Lock()
	for _, cc := range p.conns[addr] {
		if st := cc.idleState(); st.canTakeNewRequest {
			if p.shouldTraceGetConn(st) {
				traceGetConn(req, addr)
			}
			p.mu.Unlock()
			return cc, nil
		}
	}
	if !dialOnMiss {
		p.mu.Unlock()
		return nil, ErrNoCachedConn
	}
	traceGetConn(req, addr)
	call := p.getStartDialLocked(addr)
	p.mu.Unlock()
	select {
	case <-call.done:
		return call.res, call.err
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}
}

// dialCall is an in-flight Transport dial call to a host.
type dialCall struct {
	_    incomparable
	p    *clientConnPool
	done chan struct{} // closed when done
	res  *ClientConn   // valid after done is closed
	err  error         // valid after done is closed
}

// requires p.mu is held.
func (p *clientConnPool) getStartDialLocked(addr string) *dialCall {
	if call, ok := p.dialing[addr]; ok {
		// A dial is already in-flight. Don't start another.
		return call
	}
	call := &dialCall{p: p, done: make(chan struct{})}
	if p.dialing == nil {
		p.dialing = make(map[string]*dialCall)
	}
	p.dialing[addr] = call
	go call.dial(addr)
	return call
}

// run in its own goroutine.
func (c *dialCall) dial(addr string) {
	const singleUse = false // shared conn
	c.res, c.err = c.p.t.dialClientConn(addr, singleUse)
	close(c.done)

	c.p.mu.Lock()
	delete(c.p.dialing, addr)
	if c.err == nil {
		c.p.addConnLocked(addr, c.res)
	}
	c.p.mu.Unlock()
}

// addConnIfNeeded makes a NewClientConn out of c if a connection for key doesn't
// already exist. It coalesces concurrent calls with the same key.
// This is used by the http1 Transport code when it creates a new connection. Because
// the http1 Transport doesn't de-dup TCP dials to outbound hosts (because it doesn't know
// the protocol), it can get into a situation where it has multiple TLS connections.
// This code decides which ones live or die.
// The return value used is whether c was used.
// c is never closed.
func (p *clientConnPool) addConnIfNeeded(key string, t *Transport, c *tls.Conn) (used bool, err error) {
	p.mu.Lock()
	for _, cc := range p.conns[key] {
		if cc.CanTakeNewRequest() {
			p.mu.Unlock()
			return false, nil
		}
	}
	call, dup := p.addConnCalls[key]
	if !dup {
		if p.addConnCalls == nil {
			p.addConnCalls = make(map[string]*addConnCall)
		}
		call = &addConnCall{
			p:    p,
			done: make(chan struct{}),
		}
		p.addConnCalls[key] = call
		go call.run(t, key, c)
	}
	p.mu.Unlock()

	<-call.done
	if call.err != nil {
		return false, call.err
	}
	return !dup, nil
}

type addConnCall struct {
	_    incomparable
	p    *clientConnPool
	done chan struct{} // closed when done
	err  error
}

func (c *addConnCall) run(t *Transport, key string, tc *tls.Conn) {
	cc, err := t.NewClientConn(tc)

	p := c.p
	p.mu.Lock()
	if err != nil {
		c.err = err
	} else {
		p.addConnLocked(key, cc)
	}
	delete(p.addConnCalls, key)
	p.mu.Unlock()
	close(c.done)
}

// p.mu must be held
func (p *clientConnPool) addConnLocked(key string, cc *ClientConn) {
	for _, v := range p.conns[key] {
		if v == cc {
			return
		}
	}
	if p.conns == nil {
		p.conns = make(map[string][]*ClientConn)
	}
	if p.keys == nil {
		p.keys = make(map[*ClientConn][]string)
	}
	p.conns[key] = append(p.conns[key], cc)
	p.keys[cc] = append(p.keys[cc], key)
}

func (p *clientConnPool) MarkDead(cc *ClientConn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, key := range p.keys[cc] {
		vv, ok := p.conns[key]
		if !ok {
			continue
		}
		newList := filterOutClientConn(vv, cc)
		if len(newList) > 0 {
			p.conns[key] = newList
		} else {
			delete(p.conns, key)
		}
	}
	delete(p.keys, cc)
}

func (p *clientConnPool) closeIdleConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()
	// TODO: don't close a cc if it was just added to the pool
	// milliseconds ago and has never been used. There's currently
	// a small race window with the HTTP/1 Transport's integration
	// where it can add an idle conn just before using it, and
	// somebody else can concurrently call CloseIdleConns and
	// break some caller's RoundTrip.
	for _, vv := range p.conns {
		for _, cc := range vv {
			cc.closeIfIdle()
		}
	}
}

func filterOutClientConn(in []*ClientConn, exclude *ClientConn) []*ClientConn {
	out := in[:0]
	for _, v := range in {
		if v != exclude {
			out = append(out, v)
		}
	}
	// If we filtered it out, zero out the last item to prevent
	// the GC from seeing it.
	if len(in) != len(out) {
		in[len(in)-1] = nil
	}
	return out
}

// noDialClientConnPool is an implementation of http2.ClientConnPool
// which never dials. We let the HTTP/1.1 client dial and use its TLS
// connection instead.
type noDialClientConnPool struct{ *clientConnPool }

func (p noDialClientConnPool) GetClientConn(req *http.Request, addr string) (*ClientConn, error) {
	return p.getClientConn(req, addr, noDialOnMiss)
}
