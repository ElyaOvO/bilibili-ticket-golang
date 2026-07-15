// Package reporting provides a process-wide, asynchronous reporting facade.
package reporting

import (
	"bilibili-ticket-golang/lib/tasklog"
	"context"
	"log"
	"strings"
	"sync"
)

const defaultQueueCapacity = 256

// Reporter is implemented by a concrete remote reporting client.
type Reporter interface {
	ReportError(code string, err error) error
	ReportAction(action string) error
	ReportTaskLog(entry tasklog.LogEntry, code int64) error
}

// ErrorContext carries stable business context for error aggregation. Operation
// should be an explicit value such as "ticket.confirm_order" rather than a Go
// function name, so it remains stable across refactors and Garble builds.
type ErrorContext struct {
	Code      string
	Operation string
}

// ContextReporter is an optional Reporter extension. Existing Reporter
// implementations remain compatible and receive the broad code through the
// original ReportError method.
type ContextReporter interface {
	ReportErrorContext(ErrorContext, error) error
}

// LoginReporter is an optional Reporter extension for verified account-login
// events. Keeping it optional preserves compatibility with existing reporters.
type LoginReporter interface {
	ReportLogin(uid string, isRelogin bool) error
}

type eventKind uint8

const (
	eventError eventKind = iota + 1
	eventAction
	eventBarrier
	eventTaskLog
	eventLogin
)

type event struct {
	kind    eventKind
	code    string
	op      string
	err     error
	action  string
	entry   tasklog.LogEntry
	value   int64
	uid     string
	relogin bool
	done    chan struct{}
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
	return ReportErrorContext(ErrorContext{Code: code}, err)
}

// ReportErrorOp reports an error with a stable, explicit business operation.
// It is the preferred API for non-GUI code and new call sites.
func ReportErrorOp(code, operation string, err error) bool {
	operation = strings.TrimSpace(operation)
	if operation == "" {
		return false
	}
	return ReportErrorContext(ErrorContext{Code: code, Operation: operation}, err)
}

// ReportErrorContext queues an error with structured business context without
// blocking the caller.
func ReportErrorContext(errorContext ErrorContext, err error) bool {
	if err == nil || !defaultDispatcher.enabled() {
		return false
	}
	return defaultDispatcher.enqueue(event{
		kind: eventError,
		code: errorContext.Code,
		op:   errorContext.Operation,
		err:  err,
	})
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

// ReportLogin queues a verified account-login event. UID is deliberately
// restricted to a Bilibili numeric UID so arbitrary user data cannot enter
// this payload.
func ReportLogin(uid string, isRelogin bool) bool {
	uid = strings.TrimSpace(uid)
	if !validLoginUID(uid) || !defaultDispatcher.enabled() {
		return false
	}
	return defaultDispatcher.enqueue(event{kind: eventLogin, uid: uid, relogin: isRelogin})
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
			if contextual, ok := reporter.(ContextReporter); ok {
				err = contextual.ReportErrorContext(ErrorContext{Code: e.code, Operation: e.op}, e.err)
			} else {
				err = reporter.ReportError(e.code, e.err)
			}
		case eventAction:
			err = reporter.ReportAction(e.action)
		case eventTaskLog:
			err = reporter.ReportTaskLog(e.entry, e.value)
		case eventLogin:
			if loginReporter, ok := reporter.(LoginReporter); ok {
				err = loginReporter.ReportLogin(e.uid, e.relogin)
			}
		}
		if err != nil {
			log.Printf("[reporting] send failed: %v", err)
		}
	}
}

func validLoginUID(uid string) bool {
	if uid == "" || uid == "0" || len(uid) > 20 {
		return false
	}
	for _, r := range uid {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
