package main

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateTarGzFilteredIntegrity(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "checkpoint_archiver_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0755); err != nil {
		t.Fatalf("Failed to create sub dir: %v", err)
	}

	testData := []byte("CHECKPOINT_TAR_GZ_INTEGRITY_DATA_1234567890")
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "save.bin"), testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	tarPath := filepath.Join(tmpDir, "data.tar.gz")
	if err := createTarGzFiltered(srcDir, tarPath, nil); err != nil {
		t.Fatalf("createTarGzFiltered failed: %v", err)
	}

	// Read back and verify tar structure
	f, err := os.Open(tarPath)
	if err != nil {
		t.Fatalf("Failed to open output tar.gz: %v", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip.NewReader failed: %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	var foundFiles int
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tr.Next() encountered error (possible unexpected EOF): %v", err)
		}
		if !hdr.FileInfo().IsDir() {
			foundFiles++
			content, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("Failed reading tar entry %s: %v", hdr.Name, err)
			}
			if string(content) != string(testData) {
				t.Errorf("Content mismatch: expected %q, got %q", testData, content)
			}
		}
	}

	if foundFiles != 1 {
		t.Errorf("Expected 1 file in tarball, found %d", foundFiles)
	}
}
