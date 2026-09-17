package http_test

// [SIGHTGLASS PATCH 8] this file is patch 8's fork-local guard.

// HTTP1OmitKey drops a header on HTTP/1.1 and leaves it alone on HTTP/2.
//
// Some headers are protocol-scoped in a direction the existing machinery cannot express.
// Connection-specific fields go the easy way: HTTP/2 drops Connection, Keep-Alive,
// Proxy-Connection, Transfer-Encoding and Upgrade itself (RFC 9113 §8.2.2), so a caller sets them
// unconditionally and only HTTP/1.1 writes them. The other direction had no equivalent — a header
// that belongs on HTTP/2 but not on HTTP/1.1 is written by the HTTP/1.1 serializer, and the caller
// cannot know which protocol will be negotiated because ALPN is decided at dial time and the
// request is built before that.
//
// The concrete case is RFC 9218's `priority`: Chrome sends it on 112 of 119 captured HTTP/2
// requests as the LAST field, and on 0 of 744 captured HTTP/1.1 requests.

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	http "github.com/Berserk-Automation-Hub/fhttp"
)

func writeH1(t *testing.T, req *http.Request) string {
	t.Helper()
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	if err := req.Write(w); err != nil {
		t.Fatalf("Request.Write: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	return b.String()
}

func TestHTTP1OmitKeyDropsOnlyOnHTTP1(t *testing.T) {
	newReq := func() *http.Request {
		req, err := http.NewRequest(http.MethodGet, "http://example.invalid/x", nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		req.Header["User-Agent"] = []string{"probe"}
		req.Header["Priority"] = []string{"u=0, i"}
		req.Header["Accept"] = []string{"*/*"}
		return req
	}

	// Without the key, `priority` is written — which is the defect.
	raw := writeH1(t, newReq())
	if !strings.Contains(raw, "Priority: u=0, i") {
		t.Fatalf("baseline: priority is not written at all, so the test below proves nothing:\n%s", raw)
	}

	// With it, it is gone, and nothing else is.
	req := newReq()
	req.Header[http.HTTP1OmitKey] = []string{"priority"}
	raw = writeH1(t, req)
	if strings.Contains(strings.ToLower(raw), "priority:") {
		t.Fatalf("priority survived on the HTTP/1.1 wire despite HTTP1OmitKey:\n%s", raw)
	}
	if strings.Contains(raw, http.HTTP1OmitKey) {
		t.Fatalf("the magic key itself was written to the wire:\n%s", raw)
	}
	for _, want := range []string{"User-Agent: probe", "Accept: */*", "Host: example.invalid"} {
		if !strings.Contains(raw, want) {
			t.Fatalf("HTTP1OmitKey dropped more than it was asked to — %q is missing:\n%s", want, raw)
		}
	}
}

// TestHTTP1OmitKeyIsCaseInsensitive: a caller naming "Priority" must drop a header keyed "priority"
// and vice versa, because HTTP/1.1 names are case-insensitive and this library's whole point is that
// the CASE on the wire is chosen deliberately.
func TestHTTP1OmitKeyIsCaseInsensitive(t *testing.T) {
	for _, tc := range []struct{ headerKey, omitName string }{
		{"Priority", "priority"},
		{"priority", "Priority"},
		{"PRIORITY", "priority"},
	} {
		req, err := http.NewRequest(http.MethodGet, "http://example.invalid/x", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header[tc.headerKey] = []string{"u=0, i"}
		req.Header[http.HTTP1OmitKey] = []string{tc.omitName}
		if raw := writeH1(t, req); strings.Contains(strings.ToLower(raw), "priority:") {
			t.Fatalf("header %q was not dropped by omit name %q:\n%s", tc.headerKey, tc.omitName, raw)
		}
	}
}

// TestHTTP1OmitKeyWorksWithHeaderOrder covers the path a browser-emulating caller actually takes:
// HeaderOrderKey present, so writeSubset goes through SortedKeyValuesBy rather than SortedKeyValues.
func TestHTTP1OmitKeyWorksWithHeaderOrder(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.invalid/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header["User-Agent"] = []string{"probe"}
	req.Header["Accept"] = []string{"*/*"}
	req.Header["Priority"] = []string{"u=0, i"}
	req.Header[http.HeaderOrderKey] = []string{"host", "user-agent", "accept", "priority"}
	req.Header[http.HTTP1OmitKey] = []string{"priority"}

	raw := writeH1(t, req)
	if strings.Contains(strings.ToLower(raw), "priority:") {
		t.Fatalf("priority survived the ordered path:\n%s", raw)
	}
	ua, acc := strings.Index(raw, "User-Agent:"), strings.Index(raw, "Accept:")
	if ua < 0 || acc < 0 || ua > acc {
		t.Fatalf("the stated order was not honoured once a name was omitted:\n%s", raw)
	}
}
