package issue

import "testing"

func TestSplit(t *testing.T) {
	cases := []struct {
		arg       string
		wantType  string
		wantTitle string
	}{
		{"bug/dboverflow", "bug", "dboverflow"},
		{"bug", "bug", ""},
		{"bug/foo/bar", "bug", "foo/bar"}, // only splits on the first '/'
	}
	for _, c := range cases {
		gotType, gotTitle := Split(c.arg)
		if gotType != c.wantType || gotTitle != c.wantTitle {
			t.Errorf("Split(%q) = (%q, %q), want (%q, %q)", c.arg, gotType, gotTitle, c.wantType, c.wantTitle)
		}
	}
}

func TestValidateTypeValid(t *testing.T) {
	for _, v := range []string{"bug", "feature", "chore"} {
		if err := ValidateType(v); err != nil {
			t.Errorf("ValidateType(%q) = %v, want nil", v, err)
		}
	}
}

func TestValidateTypeInvalid(t *testing.T) {
	for _, v := range []string{"", "-bug", "bug/sub", "bug..x"} {
		if err := ValidateType(v); err == nil {
			t.Errorf("ValidateType(%q) = nil, want error", v)
		}
	}
}

func TestValidateTitleValid(t *testing.T) {
	for _, v := range []string{"dboverflow", "0001", "fix-the-thing"} {
		if err := ValidateTitle("bug", v); err != nil {
			t.Errorf("ValidateTitle(bug, %q) = %v, want nil", v, err)
		}
	}
}

func TestValidateTitleRejectsEmbeddedSlash(t *testing.T) {
	if err := ValidateTitle("bug", "foo/bar"); err == nil {
		t.Error("expected error for title containing '/'")
	}
}

func TestValidateTitleRejectsInvalidRefChars(t *testing.T) {
	for _, v := range []string{"foo..bar", "foo~bar", "foo bar", "foo.lock"} {
		if err := ValidateTitle("bug", v); err == nil {
			t.Errorf("ValidateTitle(bug, %q) = nil, want error", v)
		}
	}
}

func TestValidateTitleRejectsWindowsReservedNames(t *testing.T) {
	for _, v := range []string{"CON", "con", "NUL", "com1", "LPT9"} {
		if err := ValidateTitle("bug", v); err == nil {
			t.Errorf("ValidateTitle(bug, %q) = nil, want error (reserved Windows name)", v)
		}
	}
}

func TestValidateTitleRejectsTrailingDotOrSpace(t *testing.T) {
	for _, v := range []string{"foo.", "foo "} {
		if err := ValidateTitle("bug", v); err == nil {
			t.Errorf("ValidateTitle(bug, %q) = nil, want error (trailing dot/space)", v)
		}
	}
}
