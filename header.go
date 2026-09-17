// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http

import (
	"io"
	"maps"
	"net/textproto"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Berserk-Automation-Hub/fhttp/httptrace"
)

// A Header represents the Key-value pairs in an HTTP header.
//
// The keys should be in canonical form, as returned by
// CanonicalHeaderKey.
type Header map[string][]string

// HeaderOrderKey is a magic Key for ResponseWriter.Header map keys
// that, if present, defines a header order that will be used to
// write the headers onto wire. The order of the slice defined how the headers
// will be sorted. A defined Key goes before an undefined Key.
//
// This is the only way to specify some order, because maps don't
// have a a stable iteration order. If no order is given, headers will
// be sorted lexicographically.
//
// According to RFC2616 it is good practice to send general-header fields
// first, followed by request-header or response-header fields and ending
// with entity-header fields.
const HeaderOrderKey = "Header-Order:"

// PHeaderOrderKey is a magic Key for setting http2 pseudo header order.
// If the header is nil it will use regular GoLang header order.
// Valid fields are :authority, :method, :path, :scheme
const PHeaderOrderKey = "PHeader-Order:"

// HTTP1OmitKey is a magic Key whose values name headers to omit on HTTP/1.1 ONLY. They are written
// normally on HTTP/2 and HTTP/3.
//
// It exists because some headers are protocol-scoped in a direction the existing machinery cannot
// express. Connection-specific fields go the easy way — HTTP/2 already drops `Connection`,
// `Keep-Alive`, `Proxy-Connection`, `Transfer-Encoding` and `Upgrade` itself (RFC 9113 §8.2.2), so a
// caller can set them unconditionally and only HTTP/1.1 writes them. The other direction has no
// equivalent: a header that belongs on HTTP/2 and HTTP/3 but NOT on HTTP/1.1 will be written by the
// HTTP/1.1 serializer, and the caller cannot know which protocol the transport will negotiate,
// because ALPN is decided at dial time and the request is built before that.
//
// The concrete case is RFC 9218's `priority`. Chrome sends it on 112 of 119 captured HTTP/2
// requests, as the LAST field, and on 0 of 744 captured HTTP/1.1 requests — it has no HTTP/1.1 form
// at all. Without this key, a request built once and sent over whichever protocol the origin offers
// either loses it on HTTP/2 or invents it on HTTP/1.1.
//
// Values are matched case-insensitively. Like the other magic keys, this one is never written to the
// wire and is skipped by the HTTP/2 and HTTP/3 encoders.
// [SIGHTGLASS PATCH 8]
const HTTP1OmitKey = "HTTP1-Omit:"

// NoAutoHeadersKey is a magic Key that, when present, forbids this package from adding ANY header
// the caller did not ask for. Its values are ignored; presence is the whole signal.
//
// Without it, a caller who states an exact request block does not get one. Three separate sites add
// headers behind the caller's back, on every protocol:
//
//	Request.write          User-Agent: Go-http-client/1.1   when no user-agent key exists
//	Transport.roundTrip    Accept-Encoding: gzip, deflate, br
//	http2 encodeHeaders    user-agent: Go-http-client/2.0, accept-encoding: gzip, deflate, br
//
// None of them is reachable through the header map, because each triggers on ABSENCE — writing an
// empty value suppresses the User-Agent ones but not the Accept-Encoding ones, which test
// Header.Get() and therefore cannot tell "no value" from "not set". Disabling compression at the
// Transport is the only existing lever and it is both connection-wide and entangled with response
// decoding, so it cannot express "this request carries exactly these fields".
//
// The two injected values are also the loudest possible identity leak for a caller emulating a
// browser: a byte-exact browser TLS ClientHello followed by `User-Agent: Go-http-client/2.0`.
//
// Setting this key costs the automatic gzip request AND the automatic gunzip of the response, which
// is the honest pairing: this package only decodes what it asked for.
//
// Like the other magic keys, it is never written to the wire.
// [SIGHTGLASS PATCH 8b]
const NoAutoHeadersKey = "No-Auto-Headers:"

// NoAutoHeaders reports whether the caller has forbidden library-added headers on this request.
// Exported because the http2 package, which has its own copy of both injection sites, must ask too.
func (h Header) NoAutoHeaders() bool {
	if h == nil {
		return false
	}
	_, ok := h[NoAutoHeadersKey]
	return ok
}

// Add adds the Key, value pair to the header.
// It appends to any existing Values associated with Key.
// The Key is case insensitive; it is canonicalized by
// CanonicalHeaderKey.
func (h Header) Add(key, value string) {
	textproto.MIMEHeader(h).Add(key, value)
}

// Append adds the Key, value pair to the header by also appending the key to the header order.
// It appends to any existing Values associated with Key.
// The Key is case insensitive; it is canonicalized by
func (h Header) Append(key, value string) {
	textproto.MIMEHeader(h).Add(key, value)
	textproto.MIMEHeader(h).Add(HeaderOrderKey, strings.ToLower(key))
}

// Set sets the header entries associated with Key to the
// single element value. It replaces any existing Values
// associated with Key. The Key is case insensitive; it is
// canonicalized by textproto.CanonicalMIMEHeaderKey.
// To use non-canonical keys, assign to the map directly.
func (h Header) Set(key, value string) {
	textproto.MIMEHeader(h).Set(key, value)
}

// Get gets the first value associated with the given Key. If
// there are no Values associated with the Key, Get returns "".
// It is case insensitive; textproto.CanonicalMIMEHeaderKey is
// used to canonicalize the provided Key. To use non-canonical keys,
// access the map directly.
func (h Header) Get(key string) string {
	return textproto.MIMEHeader(h).Get(key)
}

// Values returns all Values associated with the given Key.
// It is case insensitive; textproto.CanonicalMIMEHeaderKey is
// used to canonicalize the provided Key. To use non-canonical
// keys, access the map directly.
// The returned slice is not a copy.
func (h Header) Values(key string) []string {
	return textproto.MIMEHeader(h).Values(key)
}

// get is like Get, but Key must already be in CanonicalHeaderKey form.
func (h Header) get(key string) string {
	if v := h[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// has reports whether h has the provided Key defined, even if it's
// set to 0-length slice.
func (h Header) has(key string) bool {
	_, ok := h[key]
	return ok
}

// Del deletes the Values associated with Key.
// The Key is case insensitive; it is canonicalized by
// CanonicalHeaderKey.
func (h Header) Del(key string) {
	textproto.MIMEHeader(h).Del(key)
}

// Write writes a header in wire format.
func (h Header) Write(w io.Writer) error {
	return h.write(w, nil)
}

func (h Header) write(w io.Writer, trace *httptrace.ClientTrace) error {
	return h.writeSubset(w, nil, trace)
}

// Clone returns a copy of h or nil if h is nil.
func (h Header) Clone() Header {
	if h == nil {
		return nil
	}

	// Find total number of Values.
	nv := 0
	for _, vv := range h {
		nv += len(vv)
	}
	sv := make([]string, nv) // shared backing array for headers' Values
	h2 := make(Header, len(h))
	for k, vv := range h {
		n := copy(sv, vv)
		h2[k] = sv[:n:n]
		sv = sv[n:]
	}
	return h2
}

var timeFormats = []string{
	TimeFormat,
	time.RFC850,
	time.ANSIC,
}

// ParseTime parses a time header (such as the Date: header),
// trying each of the three formats allowed by HTTP/1.1:
// TimeFormat, time.RFC850, and time.ANSIC.
func ParseTime(text string) (t time.Time, err error) {
	for _, layout := range timeFormats {
		t, err = time.Parse(layout, text)
		if err == nil {
			return
		}
	}
	return
}

var headerNewlineToSpace = strings.NewReplacer("\n", " ", "\r", " ")

// stringWriter implements WriteString on a Writer.
type stringWriter struct {
	w io.Writer
}

func (w stringWriter) WriteString(s string) (n int, err error) {
	return w.w.Write([]byte(s))
}

type HeaderKeyValues struct {
	Key    string
	Values []string
}

// A headerSorter implements sort.Interface by sorting a []keyValues
// by the given order, if not nil, or by Key otherwise.
// It's used as a pointer, so it can fit in a sort.Interface
// interface value without allocation.
type headerSorter struct {
	kvs   []HeaderKeyValues
	order map[string]int
	// orderIdx[i], orderOK[i] cache order[strings.ToLower(kvs[i].Key)],
	// resolved once per sort by SortedKeyValuesBy so that Less does no
	// map lookups or lowercasing per comparison. Populated only when
	// order is non-nil.
	orderIdx []int
	orderOK  []bool
}

func (s *headerSorter) Len() int { return len(s.kvs) }
func (s *headerSorter) Swap(i, j int) {
	s.kvs[i], s.kvs[j] = s.kvs[j], s.kvs[i]
	// orderIdx/orderOK are only populated by SortedKeyValuesBy;
	// SortedKeyValues sorts without them.
	if s.order != nil {
		s.orderIdx[i], s.orderIdx[j] = s.orderIdx[j], s.orderIdx[i]
		s.orderOK[i], s.orderOK[j] = s.orderOK[j], s.orderOK[i]
	}
}
func (s *headerSorter) Less(i, j int) bool {
	// If the order isn't defined, sort lexicographically.
	if s.order == nil {
		return s.kvs[i].Key < s.kvs[j].Key
	}
	idxi, iok := s.orderIdx[i], s.orderOK[i]
	idxj, jok := s.orderIdx[j], s.orderOK[j]
	if !iok && !jok {
		return s.kvs[i].Key < s.kvs[j].Key
	} else if !iok && jok {
		return false
	} else if iok && !jok {
		return true
	}
	return idxi < idxj
}

var headerSorterPool = sync.Pool{
	New: func() interface{} { return new(headerSorter) },
}

// SortedKeyValues returns h's keys sorted in the returned kvs
// slice. The headerSorter used to sort is also returned, for possible
// return to headerSorterCache.
func (h Header) SortedKeyValues(exclude map[string]bool) (kvs []HeaderKeyValues, hs *headerSorter) {
	hs = headerSorterPool.Get().(*headerSorter)
	if cap(hs.kvs) < len(h) {
		hs.kvs = make([]HeaderKeyValues, 0, len(h))
	}
	kvs = hs.kvs[:0]
	for k, vv := range h {
		if !exclude[k] {
			kvs = append(kvs, HeaderKeyValues{k, vv})
		}
	}
	hs.kvs = kvs
	// Reset any order left on the sorter by a previous SortedKeyValuesBy
	// call, otherwise a pooled sorter sorts by the stale order instead of
	// lexicographically.
	hs.order = nil
	sort.Sort(hs)
	return kvs, hs
}

func (h Header) SortedKeyValuesBy(order map[string]int, exclude map[string]bool) (kvs []HeaderKeyValues, hs *headerSorter) {
	hs = headerSorterPool.Get().(*headerSorter)
	if cap(hs.kvs) < len(h) {
		hs.kvs = make([]HeaderKeyValues, 0, len(h))
	}
	kvs = hs.kvs[:0]
	for k, vv := range h {
		if !exclude[k] {
			kvs = append(kvs, HeaderKeyValues{k, vv})
		}
	}
	hs.kvs = kvs
	hs.order = order

	// Decorate-sort-undecorate: resolve each key's order lookup once, so
	// Less compares the cached results instead of doing two map lookups
	// (with key lowercasing) per comparison.
	if cap(hs.orderIdx) < len(kvs) {
		hs.orderIdx = make([]int, len(kvs))
		hs.orderOK = make([]bool, len(kvs))
	}
	hs.orderIdx = hs.orderIdx[:len(kvs)]
	hs.orderOK = hs.orderOK[:len(kvs)]
	for i, kv := range kvs {
		hs.orderIdx[i], hs.orderOK[i] = order[strings.ToLower(kv.Key)]
	}

	sort.Sort(hs)

	return kvs, hs
}

// WriteSubset writes a header in wire format.
// If exclude is not nil, keys where exclude[Key] == true are not written.
// Keys are not canonicalized before checking the exclude map.
func (h Header) WriteSubset(w io.Writer, exclude map[string]bool) error {
	return h.writeSubset(w, exclude, nil)
}

func (h Header) writeSubset(w io.Writer, exclude map[string]bool, trace *httptrace.ClientTrace) error {
	ws, ok := w.(io.StringWriter)
	if !ok {
		ws = stringWriter{w}
	}

	var kvs []HeaderKeyValues
	var sorter *headerSorter

	// Check if the HeaderOrder is defined.
	if headerOrder, ok := h[HeaderOrderKey]; ok {
		order := make(map[string]int)
		for i, v := range headerOrder {
			order[v] = i
		}
		// Add the magic keys to a copy of exclude instead of mutating the
		// caller's map: callers pass shared package-level maps (e.g.
		// respExcludeHeader), so writing to exclude both raced with other
		// writers and readers and leaked the exclusions into every later
		// write that used the same map.
		excl := make(map[string]bool, len(exclude)+4)
		maps.Copy(excl, exclude)
		excl[HeaderOrderKey] = true
		excl[PHeaderOrderKey] = true
		excl[HTTP1OmitKey] = true
		excl[NoAutoHeadersKey] = true
		// HTTP/1.1-only omissions. This is the HTTP/1.1 serializer, so anything named here is
		// dropped; the HTTP/2 and HTTP/3 encoders never reach this function and write it normally.
		for _, name := range h[HTTP1OmitKey] {
			for k := range h {
				if strings.EqualFold(k, name) {
					excl[k] = true
				}
			}
		}
		kvs, sorter = h.SortedKeyValuesBy(order, excl)
	} else {
		excl := exclude
		omit, hasOmit := h[HTTP1OmitKey]
		if hasOmit || h.NoAutoHeaders() {
			excl = make(map[string]bool, len(exclude)+2)
			maps.Copy(excl, exclude)
			excl[HTTP1OmitKey] = true
			excl[NoAutoHeadersKey] = true
			for _, name := range omit {
				for k := range h {
					if strings.EqualFold(k, name) {
						excl[k] = true
					}
				}
			}
		}
		kvs, sorter = h.SortedKeyValues(excl)
	}

	var formattedVals []string
	for _, kv := range kvs {
		for _, v := range kv.Values {
			v = headerNewlineToSpace.Replace(v)
			v = textproto.TrimString(v)
			for _, s := range []string{kv.Key, ": ", v, "\r\n"} {
				if _, err := ws.WriteString(s); err != nil {
					headerSorterPool.Put(sorter)
					return err
				}
			}
			if trace != nil && trace.WroteHeaderField != nil {
				formattedVals = append(formattedVals, v)
			}
		}
		if trace != nil && trace.WroteHeaderField != nil {
			trace.WroteHeaderField(kv.Key, formattedVals)
			formattedVals = nil
		}
	}
	headerSorterPool.Put(sorter)
	return nil
}

// CanonicalHeaderKey returns the canonical format of the
// header Key s. The canonicalization converts the first
// letter and any letter following a hyphen to upper case;
// the rest are converted to lowercase. For example, the
// canonical Key for "accept-encoding" is "Accept-Encoding".
// If s contains a space or invalid header field bytes, it is
// returned without modifications.
func CanonicalHeaderKey(s string) string { return textproto.CanonicalMIMEHeaderKey(s) }

// hasToken reports whether token appears with v, ASCII
// case-insensitive, with space or comma boundaries.
// token must be all lowercase.
// v may contain mixed cased.
func hasToken(v, token string) bool {
	if len(token) > len(v) || token == "" {
		return false
	}
	if v == token {
		return true
	}
	for sp := 0; sp <= len(v)-len(token); sp++ {
		// Check that first character is good.
		// The token is ASCII, so checking only a single byte
		// is sufficient. We skip this potential starting
		// position if both the first byte and its potential
		// ASCII uppercase equivalent (b|0x20) don't match.
		// False positives ('^' => '~') are caught by EqualFold.
		if b := v[sp]; b != token[0] && b|0x20 != token[0] {
			continue
		}
		// Check that start pos is on a valid token boundary.
		if sp > 0 && !isTokenBoundary(v[sp-1]) {
			continue
		}
		// Check that end pos is on a valid token boundary.
		if endPos := sp + len(token); endPos != len(v) && !isTokenBoundary(v[endPos]) {
			continue
		}
		if strings.EqualFold(v[sp:sp+len(token)], token) {
			return true
		}
	}
	return false
}

func isTokenBoundary(b byte) bool {
	return b == ' ' || b == ',' || b == '\t'
}
