package scheduler

import (
	"testing"
	"time"
)

func TestLogStorageReleaseKeepsDiskHistory(t *testing.T) {
	storage := NewLogStorage()
	storage.dirPath = t.TempDir()
	if err := storage.Load(); err != nil {
		t.Fatal(err)
	}
	storage.Append("task", LogEntry{TaskID: "task", Message: "saved", Timestamp: time.Now()})
	storage.Release("task")
	if len(storage.entries) != 0 {
		t.Fatalf("in-memory entries=%d", len(storage.entries))
	}
	entries := storage.GetEntries("task")
	if len(entries) != 1 || entries[0].Message != "saved" {
		t.Fatalf("entries=%#v", entries)
	}
	if len(storage.entries) != 0 {
		t.Fatal("historical read should not repopulate the cache")
	}
}

func TestLogBrokerCloseStreamReleasesRuntimeState(t *testing.T) {
	storage := NewLogStorage()
	storage.dirPath = t.TempDir()
	if err := storage.Load(); err != nil {
		t.Fatal(err)
	}
	broker := NewLogBroker(storage)
	stream := broker.CreateStream("task")
	stream <- LogEntry{TaskID: "task", Message: "saved", Timestamp: time.Now()}
	broker.CloseStream("task")
	if len(broker.streams) != 0 || len(broker.done) != 0 || len(broker.rings) != 0 {
		t.Fatalf("runtime state streams=%d done=%d rings=%d", len(broker.streams), len(broker.done), len(broker.rings))
	}
	if len(storage.entries) != 0 {
		t.Fatalf("in-memory persisted logs=%d", len(storage.entries))
	}
	if entries := storage.GetEntries("task"); len(entries) != 1 || entries[0].Message != "saved" {
		t.Fatalf("disk entries=%#v", entries)
	}
}
