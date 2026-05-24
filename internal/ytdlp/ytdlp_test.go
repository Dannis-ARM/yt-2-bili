package ytdlp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExistingDownloadReturnsCompleteMP4AndThumbnail(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "abc123.mp4")
	thumbnail := filepath.Join(dir, "abc123.webp")
	if err := os.WriteFile(video, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(thumbnail, []byte("thumbnail"), 0644); err != nil {
		t.Fatal(err)
	}

	result, ok := existingDownload(dir, &VideoInfo{ID: "abc123", Ext: "webm"})
	if !ok {
		t.Fatal("expected existing download to be reused")
	}
	if result.VideoPath != video {
		t.Fatalf("expected video path %q, got %q", video, result.VideoPath)
	}
	if result.ThumbnailPath != thumbnail {
		t.Fatalf("expected thumbnail path %q, got %q", thumbnail, result.ThumbnailPath)
	}
}

func TestExistingDownloadIgnoresEmptyMP4(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "abc123.mp4")
	if err := os.WriteFile(video, nil, 0644); err != nil {
		t.Fatal(err)
	}

	_, ok := existingDownload(dir, &VideoInfo{ID: "abc123", Ext: "mp4"})
	if ok {
		t.Fatal("expected empty existing video to be ignored")
	}
}
