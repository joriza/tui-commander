package core

import (
	"os"
	"regexp"
	"runtime"
	"testing"
)

// ---------------------------------------------------------------------------
// TestVolumes_NonEmpty
// ---------------------------------------------------------------------------

func TestVolumes_NonEmpty(t *testing.T) {
	vols := Volumes()
	if len(vols) == 0 {
		t.Fatal("Volumes() returned no volumes, want at least one on every platform")
	}
}

// ---------------------------------------------------------------------------
// TestVolumes_WindowsFormat
// ---------------------------------------------------------------------------

func TestVolumes_WindowsFormat(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("drive-letter volume format only applies on Windows")
	}

	pattern := regexp.MustCompile(`^[A-Z]:\\$`)
	for _, v := range Volumes() {
		if !pattern.MatchString(v.Path) {
			t.Errorf("Volume %+v has Path %q, want match %q", v, v.Path, pattern)
		}
		if v.Name != v.Path {
			t.Errorf("Volume %+v has Name %q, want Name == Path %q", v, v.Name, v.Path)
		}
		if _, err := os.Stat(v.Path); err != nil {
			t.Errorf("Volume %+v path does not exist: %v", v, err)
		}
	}
}

// ---------------------------------------------------------------------------
// TestVolumes_NonWindowsSingleton
// ---------------------------------------------------------------------------

func TestVolumes_NonWindowsSingleton(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("single-root volume layout only applies on non-Windows platforms")
	}

	vols := Volumes()
	if len(vols) != 1 {
		t.Fatalf("Volumes() returned %d volumes, want exactly 1", len(vols))
	}
	if vols[0].Name != "/" || vols[0].Path != "/" {
		t.Errorf("Volumes()[0] = %+v, want {Name: \"/\", Path: \"/\"}", vols[0])
	}
}
