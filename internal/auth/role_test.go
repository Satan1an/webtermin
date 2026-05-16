package auth

import "testing"

func TestValidRole(t *testing.T) {
	for _, r := range []string{"viewer", "operator", "admin"} {
		if !ValidRole(r) {
			t.Errorf("expected %q to be valid", r)
		}
	}
	for _, r := range []string{"", "root", "Admin", "ADMIN", "viewer ", "guest"} {
		if ValidRole(r) {
			t.Errorf("expected %q to be REJECTED", r)
		}
	}
}

func TestAtLeast(t *testing.T) {
	cases := []struct {
		have, want Role
		ok         bool
	}{
		{RoleViewer, RoleViewer, true},
		{RoleOperator, RoleViewer, true},
		{RoleAdmin, RoleViewer, true},
		{RoleViewer, RoleOperator, false},
		{RoleOperator, RoleOperator, true},
		{RoleAdmin, RoleOperator, true},
		{RoleViewer, RoleAdmin, false},
		{RoleOperator, RoleAdmin, false},
		{RoleAdmin, RoleAdmin, true},
		// unknown roles deny everything
		{"", RoleViewer, false},
		{"guest", RoleViewer, false},
		{"Admin", RoleAdmin, false}, // case-sensitive
	}
	for _, c := range cases {
		if got := AtLeast(c.have, c.want); got != c.ok {
			t.Errorf("AtLeast(%q, %q) = %v, want %v", c.have, c.want, got, c.ok)
		}
	}
}
