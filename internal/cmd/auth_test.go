package cmd

import "testing"

func TestDiffCookiesFindsNewAndChangedValues(t *testing.T) {
	before := []map[string]any{
		{"name": "session", "value": "anonymous", "domain": ".example.com", "path": "/", "storeId": "0"},
		{"name": "stable", "value": "same", "domain": ".example.com", "path": "/", "storeId": "0"},
	}
	after := []map[string]any{
		{"name": "session", "value": "authenticated", "domain": ".example.com", "path": "/", "storeId": "0"},
		{"name": "stable", "value": "same", "domain": ".example.com", "path": "/", "storeId": "0"},
		{"name": "csrf", "value": "new", "domain": ".example.com", "path": "/", "storeId": "0"},
	}

	diffs := diffCookies(before, after)
	if len(diffs) != 2 {
		t.Fatalf("diffs = %#v, want 2", diffs)
	}
	if diffs[0].Name != "session" || diffs[0].Status != "changed" {
		t.Fatalf("first diff = %#v, want changed session", diffs[0])
	}
	if diffs[1].Name != "csrf" || diffs[1].Status != "new" {
		t.Fatalf("second diff = %#v, want new csrf", diffs[1])
	}
}

func TestDiffCookiesDistinguishesCookieScope(t *testing.T) {
	before := []map[string]any{
		{"name": "session", "value": "old", "domain": "a.example.com", "path": "/", "storeId": "0"},
	}
	after := []map[string]any{
		{"name": "session", "value": "old", "domain": "a.example.com", "path": "/", "storeId": "0"},
		{"name": "session", "value": "new", "domain": "b.example.com", "path": "/", "storeId": "0"},
	}

	diffs := diffCookies(before, after)
	if len(diffs) != 1 || diffs[0].Domain != "b.example.com" || diffs[0].Status != "new" {
		t.Fatalf("diffs = %#v, want new cookie on b.example.com", diffs)
	}
}
