package utils

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"[throwaway] gh-dash Projects v2 test - will be closed": "throwaway-gh-dash-projects-v2-test-will",
		"Task detail drawer":                   "task-detail-drawer",
		"Fix: café crash (iOS 18)!":            "fix-caf-crash-ios-18",
		"Supercalifragilisticexpialidocious-x": "supercalifragilisticexpialidocious-x",
		"   ":                                  "",
	}
	for in, want := range cases {
		if got := Slugify(in, 40); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
	if got := Slugify("abcdefghij", 4); got != "abcd" {
		t.Errorf("long single word should be cut, got %q", got)
	}
}
