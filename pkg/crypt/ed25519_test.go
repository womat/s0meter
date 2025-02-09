package crypt

import (
	"os"
	"path/filepath"
	"testing"
)

// empty params lead to default values on dir and filename
func TestGenerateEd25519KeyDefaults(t *testing.T) {
	tmpDir := os.TempDir()
	f, err := GenerateEd25519KeyFiles("", "")
	if err != nil {
		t.Error(err)
	}
	files := []string{filepath.Join(tmpDir, "id_ed25519"), f}
	for _, f := range files {
		_, err := os.Stat(f)
		if err != nil {
			t.Errorf("file %s not found (%s)", f, err)
		}
	}
	// cleanup
	for _, f := range files {
		_ = os.Remove(f)
	}
}

func TestGenerateEd25519KeyFiles(t *testing.T) {
	tmpDir := os.TempDir()

	f, err := GenerateEd25519KeyFiles(tmpDir, "unittest_edkey")
	if err != nil {
		t.Error(err)
	}

	files := []string{filepath.Join(tmpDir, "unittest_edkey"), f}

	for _, f := range files {
		_, err := os.Stat(f)
		if err != nil {
			t.Errorf("file %s not found (%s)", f, err)
		}
	}

	_, err = GenerateEd25519KeyFiles(tmpDir, "unittest_edkey")
	if err == nil {
		t.Error("this should not be - existing keys are not overwritten")
	}

	// cleanup
	for _, f := range files {
		_ = os.Remove(f)
	}

}
