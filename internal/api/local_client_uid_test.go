package api

import "testing"

func TestParseLocalCreateUID(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		want  string
		found bool
	}{
		{name: "string", raw: `"abc"`, want: "abc", found: true},
		{name: "uid field", raw: `{ "uid": "def" }`, want: "def", found: true},
		{name: "block uid field", raw: `{ "block/uid": "ghi" }`, want: "ghi", found: true},
		{name: "nested block uid", raw: `{ "block": { "uid": "jkl" } }`, want: "jkl", found: true},
		{name: "missing", raw: `{ "foo": "bar" }`, want: "", found: false},
	}

	for _, tc := range cases {
		uid, ok := parseLocalCreateUID([]byte(tc.raw))
		if ok != tc.found || uid != tc.want {
			t.Fatalf("%s: expected (%v, %q), got (%v, %q)", tc.name, tc.found, tc.want, ok, uid)
		}
	}
}
