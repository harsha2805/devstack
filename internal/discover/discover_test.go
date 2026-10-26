package discover

import "testing"

func v(s string) Version {
	out, err := ParseVersion(s)
	if err != nil {
		panic(err)
	}
	return out
}

func TestVersionMatchAndOrder(t *testing.T) {
	if !v("8.3.36").Matches(v("8.3")) || !v("8.3.36").Matches(v("8")) || !v("8.3.36").Matches(v("8.3.36")) {
		t.Error("prefix match failed")
	}
	if v("8.3.36").Matches(v("8.4")) || v("8.3").Matches(v("8.3.1")) {
		t.Error("false match")
	}
	if !v("8.3.9").Less(v("8.3.10")) || v("8.4.0").Less(v("8.3.99")) {
		t.Error("numeric ordering wrong")
	}
	if v("8.3.36").MajorMinor() != "8.3" {
		t.Error("MajorMinor")
	}
	for _, bad := range []string{"", "8.x", "-1", "8..3"} {
		if _, err := ParseVersion(bad); err == nil {
			t.Errorf("ParseVersion(%q) should fail", bad)
		}
	}
}

func TestPick(t *testing.T) {
	r := Result{Builds: map[Kind][]Build{
		PHP: {{Kind: PHP, Version: v("8.4.1")}, {Kind: PHP, Version: v("8.3.36")}, {Kind: PHP, Version: v("8.3.10")}},
	}}
	cases := map[string]string{"": "8.4.1", "8.3": "8.3.36", "8.3.10": "8.3.10", "8": "8.4.1"}
	for c, want := range cases {
		b, err := r.Pick(PHP, c)
		if err != nil || b.Version.String() != want {
			t.Errorf("Pick(%q) = %v, %v; want %s", c, b.Version, err, want)
		}
	}
	if _, err := r.Pick(PHP, "7.4"); err == nil {
		t.Error("Pick(7.4) should fail")
	}
	if _, err := r.Pick(Nginx, ""); err == nil {
		t.Error("Pick of a missing kind should fail")
	}
}
