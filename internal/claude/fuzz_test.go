package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

// FuzzParseObject feeds parseObject arbitrary settings files, seeded with
// the package's real settings.json. Whatever it accepts is an object whose
// top-level keys keep their order, and marshalling it and parsing it again
// gives the same keys and values.
func FuzzParseObject(f *testing.F) {
	for _, s := range []string{
		existing,
		"{}", "{} {}", "[]", "", "null", `{"a":1,"a":2}`, `{"sshConfigs": null}`,
		`{"sshConfigs": {"id": "limabean-x"}}`, `{"k": "<&> ", "nested": {"x": [1, 2.5e3, true, null]}}`,
		`{"a": 1`, `{1: 2}`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := parseObject(data)
		if err != nil {
			if doc != nil {
				t.Fatalf("parseObject returned a document with an error: %v", err)
			}
			return
		}
		if !json.Valid(data) {
			t.Fatalf("accepted invalid JSON %q", data)
		}
		if len(doc.keys) != len(doc.values) {
			t.Fatalf("%d keys for %d values", len(doc.keys), len(doc.values))
		}
		for _, k := range doc.keys {
			if _, ok := doc.values[k]; !ok {
				t.Fatalf("key %q has no value", k)
			}
		}
		out, err := doc.marshal()
		if err != nil {
			t.Fatalf("marshalling an accepted document: %v", err)
		}
		again, err := parseObject(out)
		if err != nil {
			t.Fatalf("re-parsing %q: %v", out, err)
		}
		if len(again.keys) != len(doc.keys) {
			t.Fatalf("keys %q became %q", doc.keys, again.keys)
		}
		for i, k := range doc.keys {
			if again.keys[i] != k {
				t.Fatalf("keys %q became %q", doc.keys, again.keys)
			}
			if a, b := compact(t, doc.values[k]), compact(t, again.values[k]); !bytes.Equal(a, b) {
				t.Fatalf("value of %q: %s became %s", k, a, b)
			}
		}
		if _, err := doc.entries(); err != nil && !bytes.Contains([]byte(err.Error()), []byte("isn't a list")) {
			t.Fatalf("entries: %v", err)
		}
	})
}

func compact(t *testing.T, raw json.RawMessage) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		t.Fatalf("value %q isn't JSON: %v", raw, err)
	}
	return b.Bytes()
}

// TestParseObjectRefusesNonObjects pins the refusals the fuzz target
// relies on: anything but one JSON object is ErrInvalid or a JSON error.
func TestParseObjectRefusesNonObjects(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "[]", "null", "{} {}", `{"a": 1`, `{"a" 1}`} {
		if doc, err := parseObject([]byte(in)); err == nil || doc != nil {
			t.Errorf("parseObject(%q) = %v, %v", in, doc, err)
		}
	}
	if _, err := parseObject([]byte("{} {}")); !errors.Is(err, ErrInvalid) {
		t.Errorf("two objects: %v, want ErrInvalid", err)
	}
}
