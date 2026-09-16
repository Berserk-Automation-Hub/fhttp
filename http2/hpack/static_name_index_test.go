package hpack

import (
	"bytes"
	"testing"
)

// TestStaticNameIndexPolicyIsOnTheWire pins WHICH static entry a duplicated header NAME resolves to,
// because the choice is a per-engine wire signature and it differs by one byte per occurrence.
//
// RFC 7541 Appendix A gives `:path` two entries — 4 (`/`) and 5 (`/index.html`) — and `:method` two,
// 2 (GET) and 3 (POST). When an encoder spells the field out but cites the NAME by index, it emits
// whichever it resolved to. Measured:
//
//	Chrome 153   first match: :path -> 4, :method -> 2  (114 :path / 8 :method observations)
//	Firefox 156  last  match: :path -> 5, :method -> 3  (41 of 41 attributed HEADERS blocks)
//
// A literal-without-indexing with a 4-bit name index encodes as 0x00|index, so :path is one byte:
// 0x04 for first-match and 0x05 for last-match. That single byte is what this asserts.
func TestStaticNameIndexPolicyIsOnTheWire(t *testing.T) {
	for _, tc := range []struct {
		name      string
		lastMatch bool
		wantPath  byte
	}{
		{"first-match (Chrome)", false, 0x04},
		{"last-match (Firefox, upstream)", true, 0x05},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			e := NewEncoder(&buf)
			e.SetStaticNameIndexPolicy(tc.lastMatch)
			// A value NEITHER static entry carries, so the name-only path is what runs.
			f := HeaderField{Name: ":path", Value: "/sightglass", Sensitive: true}
			if err := e.WriteField(f); err != nil {
				t.Fatalf("WriteField: %v", err)
			}
			got := buf.Bytes()
			if len(got) == 0 {
				t.Fatal("no bytes written")
			}
			// Sensitive => never-indexed (0001xxxx), 4-bit name index in the low nibble.
			if idx := got[0] & 0x0f; idx != tc.wantPath {
				t.Errorf(":path name index = %d, want %d (leading byte %#02x)", idx, tc.wantPath, got[0])
			}
			// The round trip must still decode to the same field under either policy — the choice is
			// an encoder signature, not a protocol change.
			var out []HeaderField
			d := NewDecoder(4096, func(hf HeaderField) { out = append(out, hf) })
			if _, err := d.Write(got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(out) != 1 || out[0].Name != ":path" || out[0].Value != "/sightglass" {
				t.Errorf("round trip = %+v, want one :path=/sightglass", out)
			}
		})
	}
}

// TestStaticNameIndexPolicyLeavesNameValueHitsAlone is the control. Only the NAME-only lookup may
// change: a name+value hit and an entry whose name is unique must be identical under both policies,
// or the patch is changing more than it claims.
func TestStaticNameIndexPolicyLeavesNameValueHitsAlone(t *testing.T) {
	for _, f := range []HeaderField{
		{Name: ":method", Value: "GET"},             // exact static hit, index 2
		{Name: ":path", Value: "/"},                 // exact static hit, index 4
		{Name: ":authority", Value: "example.com"},  // unique name, no duplicate
		{Name: "user-agent", Value: "sightglass/1"}, // unique name
	} {
		var a, b bytes.Buffer
		ea, eb := NewEncoder(&a), NewEncoder(&b)
		ea.SetStaticNameIndexPolicy(false)
		eb.SetStaticNameIndexPolicy(true)
		if err := ea.WriteField(f); err != nil {
			t.Fatalf("first-match WriteField(%v): %v", f, err)
		}
		if err := eb.WriteField(f); err != nil {
			t.Fatalf("last-match WriteField(%v): %v", f, err)
		}
		if !bytes.Equal(a.Bytes(), b.Bytes()) {
			t.Errorf("%s=%s differs between policies: first-match %x, last-match %x — only a "+
				"NAME-ONLY lookup may be affected", f.Name, f.Value, a.Bytes(), b.Bytes())
		}
	}
}

// TestStaticTablesAgreeExceptOnByName pins the invariant the two tables must satisfy: identical
// entries and identical name+value maps, differing ONLY in byName. If they ever diverge elsewhere,
// idToIndex and the decoder are no longer safe to share.
func TestStaticTablesAgreeExceptOnByName(t *testing.T) {
	if len(staticTable.ents) != len(staticTableLastMatch.ents) {
		t.Fatalf("entry counts differ: %d vs %d", len(staticTable.ents), len(staticTableLastMatch.ents))
	}
	for i := range staticTable.ents {
		if staticTable.ents[i] != staticTableLastMatch.ents[i] {
			t.Errorf("ents[%d] differs: %+v vs %+v", i, staticTable.ents[i], staticTableLastMatch.ents[i])
		}
	}
	if len(staticTable.byNameValue) != len(staticTableLastMatch.byNameValue) {
		t.Errorf("byNameValue sizes differ: %d vs %d", len(staticTable.byNameValue), len(staticTableLastMatch.byNameValue))
	}
	for k, v := range staticTable.byNameValue {
		if staticTableLastMatch.byNameValue[k] != v {
			t.Errorf("byNameValue[%v] differs: %d vs %d", k, v, staticTableLastMatch.byNameValue[k])
		}
	}
	if !staticTable.static || !staticTableLastMatch.static {
		t.Error("both static tables must be marked static, or idToIndex treats one as dynamic")
	}
	// And they MUST differ on the duplicated names, or the patch does nothing.
	diff := 0
	for name, id := range staticTable.byName {
		if staticTableLastMatch.byName[name] != id {
			diff++
		}
	}
	if diff == 0 {
		t.Error("byName is identical in both tables — the last-match table is not being built")
	}
	t.Logf("byName differs on %d names (the duplicated ones)", diff)
}
