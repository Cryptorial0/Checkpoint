package main

import (
	"testing"
)

func TestIsNewerVersion(t *testing.T) {
	testCases := []struct {
		latest   string
		current  string
		expected bool
	}{
		{"1.1.0", "1.0.0", true},
		{"v1.1.0", "1.0.0", true},
		{"1.1.1", "1.1.0", true},
		{"1.1.2", "1.1.1", true},
		{"1.2.0", "1.1.0", true},
		{"2.0.0", "1.9.9", true},
		{"1.1.2", "1.1.2", false},
		{"1.1.1", "1.1.2", false},
		{"1.1.0", "1.1.0", false},
		{"v1.1.0", "v1.1.0", false},
		{"1.0.9", "1.1.0", false},
		{"1.0.0", "1.1.0", false},
		{"0.9.5", "1.1.0", false},
		{"1.1.0-beta", "1.1.0", false},
	}

	for _, tc := range testCases {
		result := isNewerVersion(tc.latest, tc.current)
		if result != tc.expected {
			t.Errorf("isNewerVersion(%q, %q) = %v; expected %v", tc.latest, tc.current, result, tc.expected)
		}
	}
}

func TestFindMatchingAsset(t *testing.T) {
	assets := []ReleaseAsset{
		{Name: "checkpoint-linux-amd64", BrowserDownloadURL: "https://example.com/linux"},
		{Name: "checkpoint-windows-amd64.exe", BrowserDownloadURL: "https://example.com/windows"},
	}

	matched, err := findMatchingAsset(assets)
	if err != nil {
		t.Fatalf("findMatchingAsset returned unexpected error: %v", err)
	}

	if matched.BrowserDownloadURL == "" {
		t.Errorf("Expected matched asset to have a valid download URL")
	}
}

func TestFindMatchingAssetAppImageSelection(t *testing.T) {
	assets := []ReleaseAsset{
		{Name: "checkpoint", BrowserDownloadURL: "https://example.com/checkpoint"},
		{Name: "checkpoint-v1.1.2.AppImage", BrowserDownloadURL: "https://example.com/checkpoint.AppImage"},
		{Name: "checkpoint.exe", BrowserDownloadURL: "https://example.com/checkpoint.exe"},
		{Name: "SHA256SUMS.txt", BrowserDownloadURL: "https://example.com/checksums"},
	}

	// 1. In AppImage mode: should match .AppImage
	t.Setenv("APPIMAGE", "/tmp/fake/checkpoint.AppImage")
	matchedAppImg, err := findMatchingAsset(assets)
	if err != nil {
		t.Fatalf("Failed to match asset in AppImage mode: %v", err)
	}
	if matchedAppImg.Name != "checkpoint-v1.1.2.AppImage" {
		t.Errorf("Expected AppImage asset, got %s", matchedAppImg.Name)
	}

	// 2. In non-AppImage mode: should match native ELF "checkpoint"
	t.Setenv("APPIMAGE", "")
	matchedNative, err := findMatchingAsset(assets)
	if err != nil {
		t.Fatalf("Failed to match native asset: %v", err)
	}
	if matchedNative.Name != "checkpoint" {
		t.Errorf("Expected native checkpoint binary, got %s", matchedNative.Name)
	}
}
