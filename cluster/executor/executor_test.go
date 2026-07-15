package executor

import (
	"context"
	"errors"
	"testing"
	"time"

	"bilibili-ticket-golang/cluster/domain"
	"bilibili-ticket-golang/lib/reporting"
)

type fakeBackend struct {
	outcomes []Outcome
	calls    int
	creds    domain.Credentials
}

func (f *fakeBackend) Attempt(context.Context, domain.ExecutionSpec) Outcome {
	o := f.outcomes[f.calls]
	f.calls++
	return o
}
func (f *fakeBackend) Credentials() domain.Credentials { return f.creds }

func validSpec() domain.ExecutionSpec {
	return domain.ExecutionSpec{AttemptID: "a", IntentID: "i", ProjectID: 1, ScreenID: 2, SKUID: 3, Buyers: []domain.Buyer{{LogicalID: "b"}}, StartMode: domain.StartImmediate, Deadline: time.Now().Add(time.Minute), IntervalMS: 1}
}

func TestUnknownErrorRetriesAndReturnsCredentials(t *testing.T) {
	b := &fakeBackend{outcomes: []Outcome{{Code: 7654}, {Code: 0, OrderID: "order"}}, creds: domain.Credentials{Version: 2}}
	var events []Event
	r := (Engine{Backend: b, Observe: func(event Event) { events = append(events, event) }}).Run(context.Background(), validSpec())
	if !r.Success || b.calls != 2 || r.Credentials.Version != 2 {
		t.Fatalf("unexpected result: %#v calls=%d", r, b.calls)
	}
	foundRetry := false
	for _, event := range events {
		if event.Stage == "response" && event.Code == 7654 && event.Retryable {
			foundRetry = true
		}
	}
	if !foundRetry {
		t.Fatalf("retry outcome missing from events: %#v", events)
	}
}

func TestUnrecoverableAndCancellation(t *testing.T) {
	b := &fakeBackend{outcomes: []Outcome{{Code: 100016}}}
	r := (Engine{Backend: b}).Run(context.Background(), validSpec())
	if r.Reason != domain.FailureUnrecoverable || r.Retryable {
		t.Fatalf("unexpected result: %#v", r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = (Engine{Backend: b}).Run(ctx, validSpec())
	if r.State != domain.AttemptStopped {
		t.Fatalf("unexpected cancelled result: %#v", r)
	}
}

func TestExpiredDeadlineDoesNotCallBackend(t *testing.T) {
	b := &fakeBackend{}
	s := validSpec()
	s.Deadline = time.Now().Add(-time.Second)
	r := (Engine{Backend: b}).Run(context.Background(), s)
	if r.Reason != domain.FailureDeadline || b.calls != 0 {
		t.Fatalf("unexpected result: %#v", r)
	}
}

func TestPartialSubOrdersProduceExplicitPartialTerminalResult(t *testing.T) {
	b := &fakeBackend{outcomes: []Outcome{{
		Code: 100016,
		SubOrders: []domain.SubOrderResult{
			{BuyerIndex: 0, State: domain.SubOrderSucceeded, OrderID: "order-1"},
			{BuyerIndex: 1, State: domain.SubOrderFailed, Code: 100016},
		},
	}}}
	r := (Engine{Backend: b}).Run(context.Background(), validSpec())
	if r.State != domain.AttemptPartial || !r.Partial || r.Success || len(r.SubOrders) != 2 {
		t.Fatalf("unexpected partial result: %#v", r)
	}
}

func TestTerminalBackendErrorIsReportedOnce(t *testing.T) {
	backendErr := errors.New("terminal backend failure")
	b := &fakeBackend{outcomes: []Outcome{{Code: 100016, Err: backendErr}}}
	var reports []ErrorEvent
	result := (Engine{
		Backend:        b,
		ErrorOperation: "ticket.purchase",
		ReportError:    func(event ErrorEvent) { reports = append(reports, event) },
	}).Run(context.Background(), validSpec())

	if result.State != domain.AttemptFailed || len(reports) != 1 {
		t.Fatalf("result=%+v reports=%+v", result, reports)
	}
	if reports[0].Code != reporting.CodeExecutorAttemptFailed || reports[0].Operation != "ticket.purchase" ||
		!errors.Is(reports[0].Err, backendErr) {
		t.Fatalf("unexpected report: %+v", reports[0])
	}
}

func TestRecoveredTransientErrorIsNotReported(t *testing.T) {
	b := &fakeBackend{outcomes: []Outcome{
		{Code: 7654, Err: errors.New("temporary network error")},
		{Code: 0, OrderID: "order"},
	}}
	var reports []ErrorEvent
	result := (Engine{
		Backend:     b,
		ReportError: func(event ErrorEvent) { reports = append(reports, event) },
	}).Run(context.Background(), validSpec())

	if !result.Success || len(reports) != 0 {
		t.Fatalf("result=%+v reports=%+v", result, reports)
	}
}

func TestInvalidSpecIsReportedAtExecutorBoundary(t *testing.T) {
	var reports []ErrorEvent
	result := (Engine{
		Backend:     &fakeBackend{},
		ReportError: func(event ErrorEvent) { reports = append(reports, event) },
	}).Run(context.Background(), domain.ExecutionSpec{})

	if result.State != domain.AttemptFailed || len(reports) != 1 || reports[0].Code != reporting.CodeExecutorSpecInvalid {
		t.Fatalf("result=%+v reports=%+v", result, reports)
	}
}

func TestNumericBiliOutcomeCreatesReportableError(t *testing.T) {
	b := &fakeBackend{outcomes: []Outcome{{Code: 100016, Message: "sku not found"}}}
	var reports []ErrorEvent
	result := (Engine{
		Backend:           b,
		AttemptErrorCode:  reporting.CodeBiliAttemptFailed,
		UpstreamNamespace: "BILI",
		ErrorOperation:    "ticket.purchase",
		ReportError:       func(event ErrorEvent) { reports = append(reports, event) },
	}).Run(context.Background(), validSpec())

	if result.State != domain.AttemptFailed || len(reports) != 1 {
		t.Fatalf("result=%+v reports=%+v", result, reports)
	}
	if reports[0].Code != reporting.CodeBiliAttemptFailed {
		t.Fatalf("code=%q", reports[0].Code)
	}
	var upstream *OutcomeError
	if !errors.As(reports[0].Err, &upstream) || upstream.Code != 100016 || upstream.TelemetryCode() != "BILI_API_100016" {
		t.Fatalf("unexpected upstream error: %T %+v", reports[0].Err, reports[0].Err)
	}
}
