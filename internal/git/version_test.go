package git

import "testing"

func TestVersionLess(t *testing.T) {
	cases := []struct {
		a, b Version
		want bool
	}{
		{Version{2, 30, 0}, Version{2, 31, 0}, true},
		{Version{2, 31, 0}, Version{2, 31, 0}, false},
		{Version{2, 31, 1}, Version{2, 31, 0}, false},
		{Version{1, 99, 99}, Version{2, 0, 0}, true},
	}
	for _, c := range cases {
		if got := c.a.Less(c.b); got != c.want {
			t.Errorf("%s.Less(%s) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestDetectVersion(t *testing.T) {
	v, err := DetectVersion()
	if err != nil {
		t.Fatalf("DetectVersion: %v", err)
	}
	if v.Major < 2 {
		t.Errorf("unexpected git major version: %d", v.Major)
	}
}

func TestCheckVersion(t *testing.T) {
	if err := CheckVersion(); err != nil {
		t.Fatalf("CheckVersion: %v (test machine's git may be older than MinVersion)", err)
	}
}
