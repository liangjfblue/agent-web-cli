package ipc

import "testing"

func TestSelectProfileByIDOrName(t *testing.T) {
	profiles := []ProfileRegistration{
		{ProfileID: "awc-11111111", ProfileName: "work"},
		{ProfileID: "awc-22222222", ProfileName: "personal"},
	}
	for _, selector := range []string{"awc-11111111", "work"} {
		got, err := SelectProfile(profiles, selector)
		if err != nil {
			t.Fatalf("SelectProfile(%q): %v", selector, err)
		}
		if got.ProfileID != "awc-11111111" {
			t.Fatalf("SelectProfile(%q) = %q", selector, got.ProfileID)
		}
	}
}

func TestSelectProfileRejectsMissingAndAmbiguousNames(t *testing.T) {
	profiles := []ProfileRegistration{
		{ProfileID: "awc-1", ProfileName: "work"},
		{ProfileID: "awc-2", ProfileName: "work"},
	}
	for _, selector := range []string{"missing", "work"} {
		if _, err := SelectProfile(profiles, selector); err == nil {
			t.Fatalf("SelectProfile(%q) should fail", selector)
		}
	}
}
