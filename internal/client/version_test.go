package client

import "testing"

func TestParseVersion(t *testing.T) {
	cases := map[string]bool{"1.2.3": true, "v1.2.3": true, "1.2.3-rc.1": true, "dev": false, "1.2": false, "": false, "a.b.c": false}
	for in, wantOK := range cases {
		if _, ok := ParseVersion(in); ok != wantOK {
			t.Errorf("ParseVersion(%q) ok = %v, want %v", in, ok, wantOK)
		}
	}
}

func TestCheckMinimum(t *testing.T) {
	cases := []struct {
		server, min string
		wantErr     bool
	}{
		{"0.6.0", "0.6.0", false},
		{"0.7.1", "0.6.0", false},
		{"1.0.0", "0.9.9", false},
		{"0.5.9", "0.6.0", true},
		{"0.6.0-rc.1", "0.6.1", true},
		{"dev", "9.9.9", false}, // unparseable builds are never rejected
		{"0.1.0", "0.0.0", false},
	}
	for _, c := range cases {
		if err := CheckMinimum(c.server, c.min); (err != nil) != c.wantErr {
			t.Errorf("CheckMinimum(%q, %q) err = %v, wantErr %v", c.server, c.min, err, c.wantErr)
		}
	}
}
