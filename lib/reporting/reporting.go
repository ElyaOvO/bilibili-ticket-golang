// Package reporting provides a process-wide, asynchronous reporting facade.
package reporting

import (
	"bilibili-ticket-golang/lib/tasklog"
	"context"
	"log"
	"sync"
)

const defaultQueueCapacity = 256

// Reporter is implemented by a concrete remote reporting client.
type Reporter interface {
	ReportError(code string, err error) error
	ReportAction(action string) error
	ReportTaskLog(entry tasklog.LogEntry, code int64) error
}

type eventKind uint8

const (
	eventError eventKind = iota + 1
	eventAction
	eventBarrier
	eventTaskLog
)

type event struct {
	kind   eventKind
	code   string
	err    error
	action string
	entry  tasklog.LogEntry
	value  int64
	done   chan struct{}
}

type dispatcher struct {
	mu       sync.RWMutex
	reporter Reporter
	queue    chan event
	start    sync.Once
}

var defaultDispatcher = dispatcher{queue: make(chan event, defaultQueueCapacity)}

// SetDefault installs the process-wide reporter. Passing nil disables reporting.
func SetDefault(reporter Reporter) {
	defaultDispatcher.mu.Lock()
	defaultDispatcher.reporter = reporter
	defaultDispatcher.mu.Unlock()
	if reporter != nil {
		defaultDispatcher.start.Do(func() { go defaultDispatcher.run() })
	}
}

// ReportError queues an error without blocking the caller. It returns false
// when reporting is disabled or the bounded queue is full.
func ReportError(code string, err error) bool {
	if err == nil || !defaultDispatcher.enabled() {
		return false
	}
	return defaultDispatcher.enqueue(event{kind: eventError, code: code, err: err})
}

// ReportAction queues an action without blocking the caller.
func ReportAction(action string) bool {
	if action == "" || !defaultDispatcher.enabled() {
		return false
	}
	return defaultDispatcher.enqueue(event{kind: eventAction, action: action})
}

// ReportTaskLog queues a structured task log without blocking the caller.
func ReportTaskLog(entry tasklog.LogEntry, code int64) bool {
	if !defaultDispatcher.enabled() {
		return false
	}
	return defaultDispatcher.enqueue(event{kind: eventTaskLog, entry: entry, value: code})
}

// Flush waits until all events queued before this call have been processed.
func Flush(ctx context.Context) error {
	if !defaultDispatcher.enabled() {
		return nil
	}
	done := make(chan struct{})
	select {
	case defaultDispatcher.queue <- event{kind: eventBarrier, done: done}:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *dispatcher) enabled() bool {
	d.mu.RLock()
	enabled := d.reporter != nil
	d.mu.RUnlock()
	return enabled
}

func (d *dispatcher) enqueue(e event) bool {
	select {
	case d.queue <- e:
		return true
	default:
		return false
	}
}

func (d *dispatcher) run() {
	for e := range d.queue {
		if e.kind == eventBarrier {
			close(e.done)
			continue
		}
		d.mu.RLock()
		reporter := d.reporter
		d.mu.RUnlock()
		if reporter == nil {
			continue
		}
		var err error
		switch e.kind {
		case eventError:
			err = reporter.ReportError(e.code, e.err)
		case eventAction:
			err = reporter.ReportAction(e.action)
		case eventTaskLog:
			err = reporter.ReportTaskLog(e.entry, e.value)
		}
		if err != nil {
			log.Printf("[reporting] send failed: %v", err)
		}
	}
}
