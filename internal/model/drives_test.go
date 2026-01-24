package model

import (
	"runtime"
	"testing"
)

func TestGetDrives(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("skipping Windows-specific test")
	}

	drives, err := GetDrives()
	if err != nil {
		t.Fatalf("GetDrives failed: %v", err)
	}

	if len(drives) == 0 {
		t.Error("expected at least one drive")
	}

	// C: should typically exist
	hasC := false
	for _, d := range drives {
		if d.Letter == "C" {
			hasC = true
			break
		}
	}
	if !hasC {
		t.Error("expected C: drive to exist")
	}
}
