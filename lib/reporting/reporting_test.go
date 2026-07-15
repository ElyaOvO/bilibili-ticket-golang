package reporting

import (
	"context"
	"errors"
	"testing"
	"time"

	"bilibili-ticket-golang/lib/tasklog"
)

type contextTestReporter struct {
	received chan ErrorContext
	logins   chan loginEvent
}

type loginEvent struct {
	uid       string
	isRelogin bool
}

func (r *contextTestReporter) ReportError(string, error) error { return nil }
func (r *contextTestReporter) ReportAction(string) error       { return nil }
func (r *contextTestReporter) ReportTaskLog(tasklog.LogEntry, int64) error {
	return nil
}
func (r *contextTestReporter) ReportErrorContext(value ErrorContext, _ error) error {
	r.received <- value
	return nil
}
func (r *contextTestReporter) ReportLogin(uid string, isRelogin bool) error {
	if r.logins != nil {
		r.logins <- loginEvent{uid: uid, isRelogin: isRelogin}
	}
	return nil
}

func TestReportErrorOpUsesContextReporter(t *testing.T) {
	reporter := &contextTestReporter{received: make(chan ErrorContext, 1)}
	SetDefault(reporter)
	t.Cleanup(func() { SetDefault(nil) })

	if !ReportErrorOp("WORKER_TASK_ERROR", " ticket.confirm_order ", errors.New("failed")) {
		t.Fatal("ReportErrorOp did not enqueue the error")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := Flush(ctx); err != nil {
		t.Fatal(err)
	}

	select {
	case received := <-reporter.received:
		if received.Code != "WORKER_TASK_ERROR" || received.Operation != "ticket.confirm_order" {
			t.Fatalf("unexpected context: %+v", received)
		}
	default:
		t.Fatal("context reporter was not called")
	}
}

func TestReportLoginUsesOptionalLoginReporter(t *testing.T) {
	reporter := &contextTestReporter{
		received: make(chan ErrorContext, 1),
		logins:   make(chan loginEvent, 1),
	}
	SetDefault(reporter)
	t.Cleanup(func() { SetDefault(nil) })

	if !ReportLogin(" 123456789 ", true) {
		t.Fatal("ReportLogin did not enqueue the login")
	}
	if ReportLogin("uid=123", false) {
		t.Fatal("ReportLogin accepted a non-numeric UID")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := Flush(ctx); err != nil {
		t.Fatal(err)
	}

	select {
	case received := <-reporter.logins:
		if received.uid != "123456789" || !received.isRelogin {
			t.Fatalf("unexpected login: %+v", received)
		}
	default:
		t.Fatal("login reporter was not called")
	}
}
