package hpack

// [SIGHTGLASS PATCH 6] this file is the fork-local guard for patch 6's INDEXING-POLICY half.
//
// Up to and including v0.6.9-sightglass.14 this half had no fork test at all, and PATCHES.md said so
// ("the indexing-policy half has no fork test"). Saying so is better than pretending otherwise, but
// it is not finished: Encoder.SetIndexingPolicy is code this fork ADDED, and code this fork adds is
// tested in this fork, not only through a consumer's parity guard. The wire effect is two-part and
// both parts are checked here:
//
//   - the REPRESENTATION octet. An indexed field is "Literal Header Field with Incremental Indexing"
//     (0x40 | name index); an un-indexed one is "Literal Header Field without Indexing" (0x00 | name
//     index). One byte, on the wire, in every HEADERS block that spells the name out.
//   - the DYNAMIC TABLE. An indexed field reappears on the NEXT request as a single dynamic index,
//     and an un-indexed one does not. That is the part a representation check alone cannot see, and
//     it is the part that makes the difference observable across requests rather than within one.
//
// Ground truth for the policy itself (Chrome 153, re-derived from the captures under
// groundtruth/out — see PATCHES.md, "Ground truth"): :path is spelled out as
// literal-WITHOUT-indexing on all 114 occurrences that cite an indexed name, :method on all 8, while
// :authority is emitted with incremental indexing 45 times. Those numbers belong to the profile that drives the policy; what
// this file pins is that the SEAM works, in both directions, and that nil is upstream exactly.

import (
	"bytes"
	"testing"
)

// chromeLikePolicy is the shape a profile installs: never index :path or :method, index :authority.
func chromeLikePolicy(f HeaderField) bool {
	switch f.Name {
	case ":path", ":method":
		return false
	}
	return true
}

func encodeOne(t *testing.T, policy func(HeaderField) bool, fields ...HeaderField) []byte {
	t.Helper()
	var buf bytes.Buffer
	e := NewEncoder(&buf)
	e.SetIndexingPolicy(policy)
	for _, f := range fields {
		if err := e.WriteField(f); err != nil {
			t.Fatalf("WriteField(%v): %v", f, err)
		}
	}
	return buf.Bytes()
}

// TestIndexingPolicyChangesTheRepresentationOctet pins the wire half.
func TestIndexingPolicyChangesTheRepresentationOctet(t *testing.T) {
	// A value in neither static entry, so the encoder must spell it out and cite the NAME index.
	f := HeaderField{Name: ":path", Value: "/a-value-in-no-static-entry"}

	up := encodeOne(t, nil, f)
	ch := encodeOne(t, chromeLikePolicy, f)

	if len(up) == 0 || len(ch) == 0 {
		t.Fatal("the encoder produced no bytes at all, so nothing below is measuring a representation")
	}
	// :path is static entry 4 under this fork's first-match table.
	if up[0] != 0x44 {
		t.Errorf("with policy nil the first octet is 0x%02x, want 0x44 (literal WITH incremental "+
			"indexing, name index 4). policy == nil must reproduce upstream byte for byte, or installing a "+
			"profile that declines to set one would silently change the wire.", up[0])
	}
	if ch[0] != 0x04 {
		t.Errorf("with Chrome's policy the first octet for :path is 0x%02x, want 0x04 (literal WITHOUT "+
			"indexing, name index 4). SetIndexingPolicy is not reaching the representation, so the encoder "+
			"is emitting the indexed form no browser emits — the exact octet patch 6 exists to fix.", ch[0])
	}
	if bytes.Equal(up, ch) {
		t.Errorf("the policy changed nothing: both encodings are % x. A per-field indexing hook that the "+
			"encoder ignores is a seam that reports success and ships upstream's bytes.", up)
	}
}

// TestIndexingPolicyDecidesWhatComesBackAsADynamicIndex pins the half a single-request
// representation check cannot see: whether the field entered the dynamic table.
func TestIndexingPolicyDecidesWhatComesBackAsADynamicIndex(t *testing.T) {
	path := HeaderField{Name: ":path", Value: "/repeated"}
	auth := HeaderField{Name: ":authority", Value: "example.invalid"}

	for _, tc := range []struct {
		name                     string
		policy                   func(HeaderField) bool
		wantPathIdx, wantAuthIdx bool
	}{
		{"nil (upstream): both come back indexed", nil, true, true},
		{"Chrome's: only :authority comes back indexed", chromeLikePolicy, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			e := NewEncoder(&buf)
			e.SetIndexingPolicy(tc.policy)
			for _, f := range []HeaderField{path, auth} {
				if err := e.WriteField(f); err != nil {
					t.Fatalf("first pass WriteField(%v): %v", f, err)
				}
			}
			buf.Reset()
			// Second request on the same connection: anything indexed the first time is now a single
			// "Indexed Header Field" octet (high bit set).
			for _, f := range []HeaderField{path, auth} {
				buf.Reset()
				if err := e.WriteField(f); err != nil {
					t.Fatalf("second pass WriteField(%v): %v", f, err)
				}
				got := buf.Bytes()
				indexed := len(got) == 1 && got[0]&0x80 != 0
				want := tc.wantPathIdx
				if f.Name == ":authority" {
					want = tc.wantAuthIdx
				}
				if indexed != want {
					t.Errorf("second request, %s: encoded as % x (indexed=%v), want indexed=%v.\n"+
						"An un-indexed field must NOT reappear as a dynamic index, and an indexed one must. "+
						"This is the cross-request half of patch 6: Chrome never lets :path into the dynamic "+
						"table, because the value changes every request and indexing it evicts entries that do "+
						"not.", f.Name, got, indexed, want)
				}
			}
		})
	}
}

// TestIndexingPolicyDoesNotOverrideSensitive pins the boundary the patch deliberately does not
// cross: Sensitive means "Never Indexed" (0x1x), which is a do-not-proxy instruction on the wire and
// which Chrome emits zero times in the 2513 request header fields of the census.
// A policy that returned true must not turn a Sensitive field into an indexed one.
func TestIndexingPolicyDoesNotOverrideSensitive(t *testing.T) {
	f := HeaderField{Name: "cookie", Value: "a=b", Sensitive: true}
	got := encodeOne(t, func(HeaderField) bool { return true }, f)
	if len(got) == 0 {
		t.Fatal("no bytes encoded")
	}
	if got[0]&0xf0 != 0x10 {
		t.Errorf("a Sensitive field encoded with an always-index policy starts 0x%02x, want the 0x1x "+
			"\"Never Indexed\" form. SetIndexingPolicy is a hook on a decision Sensitive has already made; "+
			"if the policy can override it, a profile can put a do-not-proxy field into the dynamic table.",
			got[0])
	}
}

// TestNilIndexingPolicyIsUpstreamExactly is the control. Every claim above is a claim about a
// DIFFERENCE, and a difference is only meaningful if the nil side is upstream untouched.
func TestNilIndexingPolicyIsUpstreamExactly(t *testing.T) {
	fields := []HeaderField{
		{Name: ":method", Value: "GET"},
		{Name: ":authority", Value: "example.invalid"},
		{Name: ":path", Value: "/x"},
		{Name: "accept", Value: "*/*"},
	}
	var withNil, never bytes.Buffer
	en := NewEncoder(&withNil)
	en.SetIndexingPolicy(nil)
	ev := NewEncoder(&never)
	for _, f := range fields {
		if err := en.WriteField(f); err != nil {
			t.Fatal(err)
		}
		if err := ev.WriteField(f); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(withNil.Bytes(), never.Bytes()) {
		t.Errorf("SetIndexingPolicy(nil) = % x but an encoder that never saw the hook = % x. The nil case "+
			"is the compatibility promise this patch rests on: every caller that does not install a policy "+
			"must get upstream's bytes.", withNil.Bytes(), never.Bytes())
	}
}
