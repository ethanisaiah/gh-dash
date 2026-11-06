package utils

import (
	"reflect"
	"testing"
)

func TestExtractImageURLs(t *testing.T) {
	body := "Intro ![logo](https://example.com/a/logo.png \"Logo\")\n" +
		"<img width=\"300\" alt=\"Screen &amp; shot\" src=\"https://github.com/user-attachments/assets/123\">\n" +
		"![dup](https://example.com/a/logo.png)\n" +
		"`![in code](https://example.com/code.png)`\n" +
		"```\n![fenced](https://example.com/fenced.png)\n```\n" +
		"![relative](docs/x.png) <img src='https://example.com/b.gif'>\n" +
		"![](<https://example.com/c d.jpg>)"

	got := ExtractImageURLs(body)
	want := []ImageRef{
		{URL: "https://example.com/a/logo.png", Alt: "logo"},
		{URL: "https://github.com/user-attachments/assets/123", Alt: "Screen & shot"},
		{URL: "https://example.com/b.gif"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestImageRefLabel(t *testing.T) {
	cases := map[ImageRef]string{
		{URL: "https://x.com/a/shot.png", Alt: "Login page"}: "Login page",
		{URL: "https://x.com/a/shot.png", Alt: "image"}:      "shot.png",
		{URL: "https://x.com/"}:                              "https://x.com/",
	}
	for ref, want := range cases {
		if got := ref.Label(); got != want {
			t.Errorf("%+v: got %q want %q", ref, got, want)
		}
	}
}
