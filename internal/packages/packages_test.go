package packages

import "testing"

func TestValidName(t *testing.T) {
	good := []string{
		"nginx", "nginx-full", "lib6.so", "python3.11", "gcc-aarch64-linux-gnu",
		"a", "package_name+", "containerd:amd64",
	}
	for _, n := range good {
		if !ValidName(n) {
			t.Errorf("expected %q valid", n)
		}
	}
	bad := []string{
		"", "Nginx", "-leading", "name with space", "name;rm", "name$(reboot)",
		"name\nrm", "../../etc/passwd",
	}
	for _, n := range bad {
		if ValidName(n) {
			t.Errorf("expected %q REJECTED", n)
		}
	}
}
