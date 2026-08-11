package cmd

import "testing"

func TestCookieRequestArgsDefaultsToActiveTab(t *testing.T) {
	args, err := cookieRequestArgs("", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if args["activeTab"] != true {
		t.Fatalf("args = %#v, want activeTab=true", args)
	}
	if _, ok := args["all"]; ok {
		t.Fatalf("args = %#v, did not want all", args)
	}
}

func TestCookieRequestArgsRequiresExplicitAll(t *testing.T) {
	args, err := cookieRequestArgs("", "session", true)
	if err != nil {
		t.Fatal(err)
	}
	if args["all"] != true || args["name"] != "session" {
		t.Fatalf("args = %#v, want all=true and name filter", args)
	}
}

func TestCookieRequestArgsRejectsURLWithAll(t *testing.T) {
	if _, err := cookieRequestArgs("https://example.com", "", true); err == nil {
		t.Fatal("expected --url and --all to be rejected")
	}
}
