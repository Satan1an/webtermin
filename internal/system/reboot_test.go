package system

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewestKernel(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().Add(-48 * time.Hour)
	for name, mod := range map[string]time.Time{
		"vmlinuz-6.18.30-current-rockchip64":     old,
		"vmlinuz-6.18.60-current-rockchip64":     time.Now(),
		"vmlinuz-6.18.70-current-rockchip64.old": time.Now().Add(time.Hour),
	} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		_ = os.Chtimes(p, mod, mod)
	}
	if got := newestKernel(dir); got != "6.18.60-current-rockchip64" {
		t.Errorf("newestKernel = %q", got)
	}
	if got := newestKernel(t.TempDir()); got != "" {
		t.Errorf("empty /boot should give empty string, got %q", got)
	}
}
