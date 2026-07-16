package logfile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenRotatingAtCreatesFreshLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "main.log")

	file, archived, err := openRotatingAt(path, time.Date(2026, 7, 15, 13, 14, 15, 0, time.Local))
	if err != nil {
		t.Fatalf("OpenRotatingAt() error = %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })
	if archived != "" {
		t.Fatalf("archived = %q, want empty", archived)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("fresh log does not exist: %v", err)
	}
}

func TestOpenRotatingAtArchivesPreviousLog(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "logs", "main.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("previous log"), 0o644); err != nil {
		t.Fatal(err)
	}

	file, archived, err := openRotatingAt(path, time.Date(2026, 7, 15, 13, 14, 15, 0, time.Local))
	if err != nil {
		t.Fatalf("OpenRotatingAt() error = %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })

	wantArchive := filepath.Join(root, "logs", "old", "2026-07-15 13-14-15.log")
	if archived != wantArchive {
		t.Fatalf("archived = %q, want %q", archived, wantArchive)
	}
	contents, err := os.ReadFile(wantArchive)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	if string(contents) != "previous log" {
		t.Fatalf("archive contents = %q", contents)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("fresh log does not exist: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("fresh log size = %d, want 0", info.Size())
	}
}

func TestOpenRotatingAtDoesNotOverwriteSameSecondArchive(t *testing.T) {
	root := t.TempDir()
	logDirectory := filepath.Join(root, "logs")
	archiveDirectory := filepath.Join(logDirectory, "old")
	path := filepath.Join(logDirectory, "main.log")
	if err := os.MkdirAll(archiveDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	existingArchive := filepath.Join(archiveDirectory, "2026-07-15 13-14-15.log")
	if err := os.WriteFile(existingArchive, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}

	file, archived, err := openRotatingAt(path, time.Date(2026, 7, 15, 13, 14, 15, 0, time.Local))
	if err != nil {
		t.Fatalf("OpenRotatingAt() error = %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })

	wantArchive := filepath.Join(archiveDirectory, "2026-07-15 13-14-15-2.log")
	if archived != wantArchive {
		t.Fatalf("archived = %q, want %q", archived, wantArchive)
	}
	contents, err := os.ReadFile(existingArchive)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "first" {
		t.Fatalf("original archive was overwritten: %q", contents)
	}
}
