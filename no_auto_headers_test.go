package http

import (
	"bytes"
	"strings"
	"testing"
)

// The caller states an exact request block. Without NoAutoHeadersKey this package adds a
// User-Agent the caller never asked for; with it, the block is exactly what was stated.
func TestNoAutoHeadersKeySuppressesTheInjectedUserAgent(t *testing.T) {
	build := func(noAuto bool) string {
		req, err := NewRequest("GET", "http://example.invalid/x", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header = Header{
			"X-One":        {"1"},
			"Accept":       {"text/plain"},
			HeaderOrderKey: {"host", "x-one", "accept"},
		}
		if noAuto {
			req.Header[NoAutoHeadersKey] = []string{"1"}
		}
		var buf bytes.Buffer
		if err := req.Write(&buf); err != nil {
			t.Fatalf("write: %v", err)
		}
		return buf.String()
	}

	with, without := build(true), build(false)

	if !strings.Contains(without, "User-Agent: Go-http-client/1.1\r\n") {
		t.Fatalf("ablation failed: without the key the injected User-Agent should be present:\n%s", without)
	}
	if strings.Contains(with, "User-Agent") {
		t.Fatalf("NoAutoHeadersKey did not suppress the injected User-Agent:\n%s", with)
	}
	// And the magic key itself is never on the wire.
	if strings.Contains(with, NoAutoHeadersKey) || strings.Contains(with, "No-Auto-Headers") {
		t.Fatalf("the magic key reached the wire:\n%s", with)
	}
	want := "GET /x HTTP/1.1\r\nHost: example.invalid\r\nX-One: 1\r\nAccept: text/plain\r\n\r\n"
	if with != want {
		t.Fatalf("request block is not exactly what the caller stated.\n got: %q\nwant: %q", with, want)
	}
}

// The key must not disturb the ordering machinery, and must stay off the wire when no order is
// stated either (the lexicographic branch of writeSubset).
func TestNoAutoHeadersKeyStaysOffTheWireWithoutAnOrder(t *testing.T) {
	h := Header{
		"B-Header":       {"b"},
		"A-Header":       {"a"},
		NoAutoHeadersKey: {"1"},
	}
	var buf bytes.Buffer
	if err := h.Write(&buf); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "A-Header: a\r\nB-Header: b\r\n"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
