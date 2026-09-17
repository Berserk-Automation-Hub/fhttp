package http_test

// [SIGHTGLASS PATCH 7] this file is patch 7's fork-local guard.

// A `Content-Encoding: deflate` response must not park persistConn.readLoop forever.
//
// DecompressBody runs INSIDE readLoop. The body it is handed is a *bodyEOFSignal whose EOF path does
// `<-eofc`, and eofc is closed only when readLoop RETURNS. So any read that reaches the end of the
// body from inside readLoop blocks readLoop on a channel that only readLoop can close:
//
//	readLoop -> DecompressBody -> identifyDeflate -> io.Copy -> bodyEOFSignal.Read
//	         -> condfn -> readLoop.func4 -> <-eofc      [parked forever]
//
// identifyDeflate used to drain the entire body with io.Copy in order to replay two sniffed octets.
// The CALLER was rescued by Client.Timeout, so the bug presented as a slow origin — but the readLoop
// goroutine and its socket were never released, and CloseIdleConnections could not free them.
//
// These tests assert the whole of that: the request completes, the body decodes, the goroutine is
// gone afterwards, and the four shapes that used to take the same path (2-octet body, empty body,
// raw DEFLATE, and a body closed without ever being read) are each exercised.

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"fmt"
	"io"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	http "github.com/Berserk-Automation-Hub/fhttp"
)

// deflateOrigin answers every request with one canned deflate body and then HOLDS the socket open,
// so only our own side can end the readLoop goroutine — which is what makes the leak assertion mean
// something.
func deflateOrigin(t *testing.T, body []byte) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	// The handlers hold their sockets until the test ends rather than sleeping: only OUR side may
	// end the readLoop goroutine, or the leak assertion proves nothing — but a sleeping handler
	// would outlive the test and trip this package's own goroutine-leak check.
	done := make(chan struct{})
	t.Cleanup(func() { close(done); _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = c.Close() }()
				_ = c.SetDeadline(time.Now().Add(60 * time.Second))
				_, _ = c.Read(make([]byte, 4096))
				_, _ = fmt.Fprintf(c, "HTTP/1.1 200 OK\r\nContent-Encoding: deflate\r\nContent-Length: %d\r\n\r\n", len(body))
				_, _ = c.Write(body)
				// Hold: a server that hangs up would end readLoop for reasons unrelated to the fix.
				<-done
			}()
		}
	}()
	return ln.Addr().String()
}

func zlibBody(t *testing.T, payload []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("zlib write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zlib close: %v", err)
	}
	return b.Bytes()
}

func rawDeflateBody(t *testing.T, payload []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w, err := flate.NewWriter(&b, flate.DefaultCompression)
	if err != nil {
		t.Fatalf("flate writer: %v", err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("flate write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("flate close: %v", err)
	}
	return b.Bytes()
}

// goroutinesParkedInDeflate counts goroutines sitting in the decode path.
func goroutinesParkedInDeflate() (int, string) {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	all := string(buf[:n])
	count := 0
	var sample string
	for _, g := range strings.Split(all, "\n\n") {
		if strings.Contains(g, "identifyDeflate") || strings.Contains(g, "prependBytesToReadCloser") ||
			strings.Contains(g, "deflateSniffer") {
			count++
			if sample == "" {
				sample = g
			}
		}
	}
	return count, sample
}

func TestDeflateResponseDoesNotParkReadLoop(t *testing.T) {
	payload := bytes.Repeat([]byte("sightglass deflate parity "), 64)

	for _, tc := range []struct {
		name string
		body func(*testing.T, []byte) []byte
	}{
		{"zlib-wrapped (RFC 1950)", zlibBody},
		{"raw DEFLATE (RFC 1951)", rawDeflateBody},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr := deflateOrigin(t, tc.body(t, payload))
			tr := &http.Transport{}
			defer tr.CloseIdleConnections()
			cl := &http.Client{Transport: tr, Timeout: 10 * time.Second}

			// No Accept-Encoding from the caller, so the TRANSPORT adds it and sets addedGzip —
			// which is the only condition under which DecompressBody runs at all.
			req, err := http.NewRequest(http.MethodGet, "http://"+addr+"/x", nil)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			resp, err := cl.Do(req)
			if err != nil {
				t.Fatalf("the request never completed (%v). Before the fix this reported "+
					"\"Client.Timeout exceeded while awaiting headers\", because readLoop was parked "+
					"inside its own body drain.", err)
			}
			defer func() { _ = resp.Body.Close() }()
			got, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if !bytes.Equal(got, payload) {
				t.Fatalf("decoded %d bytes, want %d; the body did not round-trip", len(got), len(payload))
			}
			if ce := resp.Header.Get("Content-Encoding"); ce != "" {
				t.Fatalf("Content-Encoding %q survived decoding", ce)
			}
		})
	}
}

// TestDeflateShortBodyDoesNotParkReadLoop covers the shapes that hit EOF DURING the two-octet sniff.
// Before the fix io.ReadFull took the same <-eofc path as the drain did, so a 0- or 1-octet body
// deadlocked without any copy being involved at all.
func TestDeflateShortBodyDoesNotParkReadLoop(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []byte
	}{
		{"empty body", nil},
		{"one octet", []byte{0x78}},
		{"two octets, not a zlib header", []byte{'h', 'i'}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr := deflateOrigin(t, tc.body)
			tr := &http.Transport{}
			defer tr.CloseIdleConnections()
			cl := &http.Client{Transport: tr, Timeout: 10 * time.Second}
			req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/x", nil)
			resp, err := cl.Do(req)
			if err != nil {
				t.Fatalf("the request never completed (%v); a body too short to sniff must not park "+
					"readLoop either", err)
			}
			defer func() { _ = resp.Body.Close() }()
			// The content is undecodable by definition; what matters is that reading it RETURNS.
			_, _ = io.ReadAll(resp.Body)
		})
	}
}

// TestDeflateBodyClosedWithoutReadingDoesNotPanic covers the second defect the same code carried:
// zlibDeflateReader and deflateReader construct their decoder on the FIRST READ, and their Close
// reached straight through to it — so closing a body nobody read was a nil-receiver panic.
func TestDeflateBodyClosedWithoutReadingDoesNotPanic(t *testing.T) {
	addr := deflateOrigin(t, zlibBody(t, []byte("never read")))
	tr := &http.Transport{}
	defer tr.CloseIdleConnections()
	cl := &http.Client{Transport: tr, Timeout: 10 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/x", nil)
	resp, err := cl.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("closing an unread deflate body: %v", err)
	}
}

// TestDeflateLeavesNoParkedGoroutine is the leak assertion proper. It runs several deflate requests,
// gives up on each, and then requires the decode path to hold no goroutine — with the origin still
// holding its sockets open, so nothing but the fix can free them.
func TestDeflateLeavesNoParkedGoroutine(t *testing.T) {
	addr := deflateOrigin(t, zlibBody(t, bytes.Repeat([]byte("leak"), 256)))
	before := runtime.NumGoroutine()

	for i := 0; i < 3; i++ {
		tr := &http.Transport{}
		cl := &http.Client{Transport: tr, Timeout: 5 * time.Second}
		req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/x", nil)
		resp, err := cl.Do(req)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		_ = resp.Body.Close() // give up WITHOUT reading: the harshest case for the decode path
		tr.CloseIdleConnections()
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		parked, sample := goroutinesParkedInDeflate()
		if parked == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d goroutine(s) parked in the deflate decode after every caller gave up and "+
				"CloseIdleConnections ran. The socket is held with them, for the life of the "+
				"process.\n\n%s", parked, sample)
		}
		time.Sleep(200 * time.Millisecond)
	}
	runtime.GC()
	t.Logf("goroutines %d -> %d, none parked in the deflate decode", before, runtime.NumGoroutine())
}
