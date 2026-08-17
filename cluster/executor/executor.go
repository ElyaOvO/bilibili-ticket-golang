package executor

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"bilibili-ticket-golang/cluster/domain"
	"bilibili-ticket-golang/lib/reporting"
)

// Backend owns the Bilibili prepare/confirm/createV2 transaction. Backends that
// split one intent into independent orders must return every child state in
// Outcome.SubOrders; callers treat a mix of succeeded and incomplete children
// as an explicit partial result rather than an atomic success.
type Backend interface {
	Attempt(context.Context, domain.ExecutionSpec) Outcome
	Credentials() domain.Credentials
}

type Outcome struct {
	OrderID       string
	PaymentURL    string
	PaymentExpire int64
	OrderTime     int64
	Code          int
	Message       string
	Err           error
	SubOrders     []domain.SubOrderResult
}

type httpStatusCoder interface {
	HTTPStatusCode() int
}

func applyHTTPStatusCode(outcome Outcome) Outcome {
	var statusErr httpStatusCoder
	if errors.As(outcome.Err, &statusErr) {
		if code := statusErr.HTTPStatusCode(); code != 0 {
			outcome.Code = code
		}
	}
	return outcome
}

// ProgressBackend reports durable child-order snapshots as split execution
// advances. The worker installs the sink before the first Attempt call.
type ProgressBackend interface {
	SetProgressSink(func([]domain.SubOrderResult))
}

// EventBackend emits transaction-level diagnostic events that are not final
// API outcomes, such as metadata reconciliation during order preparation.
type EventBackend interface {
	SetEventSink(func(Event))
}

type Classification struct {
	Reason    domain.FailureReason
	Retryable bool
	Backoff   time.Duration
}

type Classifier interface{ Classify(Outcome) Classification }

type DefaultClassifier struct{}

func (DefaultClassifier) Classify(o Outcome) Classification {
	switch o.Code {
	case 0, 100048, 100079:
		if o.OrderID != "" {
			return Classification{}
		}
	case 100016, 100017:
		return Classification{Reason: domain.FailureUnrecoverable}
	case 412:
		return Classification{Reason: domain.FailureHTTP412, Retryable: true, Backoff: 5 * time.Minute}
	case -101, -111:
		return Classification{Reason: domain.FailureCookieInvalid}
	case -352:
		return Classification{Reason: domain.FailureCaptcha}
	}
	// Unknown API and transport failures intentionally remain retryable.
	return Classification{Reason: domain.FailureNone, Retryable: true}
}

type Clock interface {
	Now() time.Time
	Sleep(context.Context, time.Duration) error
}

type realClock struct{}

type OffsetClock struct{ Offset time.Duration }

func (c OffsetClock) Now() time.Time { return time.Now().Add(c.Offset) }
func (c OffsetClock) Sleep(ctx context.Context, duration time.Duration) error {
	return realClock{}.Sleep(ctx, duration)
}

func (realClock) Now() time.Time { return time.Now() }
func (realClock) Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

type Engine struct {
	Backend           Backend
	Classifier        Classifier
	Clock             Clock
	Observe           func(Event)
	ReportError       func(ErrorEvent)
	ErrorOperation    string
	AttemptErrorCode  string
	RetryErrorCode    string
	UpstreamNamespace string
	GetRetryInterval  func() int64 // optional: dynamic retry interval (ms), checked each loop iteration
}

// ErrorEvent is emitted only at a terminal business boundary. Individual
// retry attempts remain normal task logs so transient failures cannot flood
// remote error reporting.
type ErrorEvent struct {
	Code      string
	Operation string
	Err       error
}

// OutcomeError turns an upstream numeric failure into a structured error even
// when the backend returned only Outcome.Code and no Go error.
type OutcomeError struct {
	Namespace string
	Code      int
	Message   string
	Retryable bool
	Cause     error
}

func (e *OutcomeError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "upstream request failed"
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: code=%d: %v", message, e.Code, e.Cause)
	}
	return fmt.Sprintf("%s: code=%d", message, e.Code)
}

func (e *OutcomeError) Unwrap() error { return e.Cause }
func (e *OutcomeError) TelemetryCode() string {
	return strings.ToUpper(e.Namespace) + "_API_" + telemetryNumericCode(e.Code)
}
func (e *OutcomeError) TelemetryCategory() string { return "upstream" }
func (e *OutcomeError) TelemetryMessage() string  { return "upstream API rejected the request" }
func (e *OutcomeError) TelemetryRetryable() bool  { return e.Retryable }
func (e *OutcomeError) TelemetryUpstreamCode() (int, bool) {
	return e.Code, true
}

func telemetryNumericCode(code int) string {
	if code < 0 {
		return "NEG_" + strconv.Itoa(-code)
	}
	return strconv.Itoa(code)
}

type Event struct {
	Time        time.Time
	Stage       string
	Message     string
	Code        int
	Retryable   bool
	CooldownEnd time.Time // zero = no cooldown; non-zero = cooldown ends at this time
}

func (e Engine) Run(ctx context.Context, spec domain.ExecutionSpec) domain.ExecutionResult {
	now := time.Now
	if e.Clock == nil {
		e.Clock = realClock{}
	} else {
		now = e.Clock.Now
	}
	if e.Classifier == nil {
		e.Classifier = DefaultClassifier{}
	}
	emit := func(stage, message string, code int, retryable bool) {
		if e.Observe != nil {
			e.Observe(Event{Time: now(), Stage: stage, Message: message, Code: code, Retryable: retryable})
		}
	}
	emitCooldown := func(stage, message string, code int, retryable bool, cooldownEnd time.Time) {
		if e.Observe != nil {
			e.Observe(Event{Time: now(), Stage: stage, Message: message, Code: code, Retryable: retryable, CooldownEnd: cooldownEnd})
		}
	}
	if backend, ok := e.Backend.(EventBackend); ok {
		backend.SetEventSink(func(event Event) {
			if event.Time.IsZero() {
				event.Time = now()
			}
			if e.Observe != nil {
				e.Observe(event)
			}
		})
	}
	reportError := func(code string, err error) {
		if err == nil || e.ReportError == nil {
			return
		}
		operation := e.ErrorOperation
		if operation == "" {
			operation = "executor.attempt"
		}
		e.ReportError(ErrorEvent{Code: code, Operation: operation, Err: err})
	}
	configuredCode := func(configured, fallback string) string {
		if code := strings.TrimSpace(configured); code != "" {
			return code
		}
		return fallback
	}
	result := domain.ExecutionResult{AttemptID: spec.AttemptID, IntentID: spec.IntentID, SpecHash: spec.Hash(), State: domain.AttemptRunning, StartedAt: now()}
	finish := func(state domain.AttemptState, reason domain.FailureReason, message string, retryable bool) domain.ExecutionResult {
		if state != domain.AttemptSucceeded && result.Partial {
			state = domain.AttemptPartial
		}
		result.State, result.Reason, result.Message, result.Retryable = state, reason, message, retryable
		result.FinishedAt = now()
		if e.Backend != nil {
			result.Credentials = e.Backend.Credentials()
		}
		return result
	}
	if err := spec.Validate(); err != nil {
		reportError(reporting.CodeExecutorSpecInvalid, err)
		return finish(domain.AttemptFailed, domain.FailureInternal, err.Error(), false)
	}
	if e.Backend == nil {
		err := errors.New("executor backend is required")
		reportError(reporting.CodeExecutorBackendMissing, err)
		return finish(domain.AttemptFailed, domain.FailureInternal, err.Error(), false)
	}
	if !spec.Deadline.After(now()) {
		return finish(domain.AttemptFailed, domain.FailureDeadline, "deadline elapsed", false)
	}
	if spec.StartMode == domain.StartScheduled && spec.StartAt.After(now()) {
		// Apply start delay: shift the scheduled time earlier by StartDelayMS.
		adjustedStart := spec.StartAt
		if spec.StartDelayMS > 0 {
			adjustedStart = adjustedStart.Add(-time.Duration(spec.StartDelayMS) * time.Millisecond)
			if adjustedStart.Before(now()) {
				adjustedStart = now()
			}
		}
		emit("scheduled", "waiting until scheduled start", 0, false)
		if err := e.Clock.Sleep(ctx, adjustedStart.Sub(now())); err != nil {
			return finish(domain.AttemptStopped, domain.FailureStopped, err.Error(), false)
		}
	}

	var lastAttemptErr error
	for {
		// Resolve the retry interval dynamically — if the caller provided
		// a GetRetryInterval hook, it is checked on every loop iteration
		// so that global config changes take effect for running tasks.
		interval := time.Duration(spec.IntervalMS) * time.Millisecond
		if e.GetRetryInterval != nil {
			if dyn := e.GetRetryInterval(); dyn > 0 {
				interval = time.Duration(dyn) * time.Millisecond
			}
		}
		if interval <= 0 {
			interval = 500 * time.Millisecond
		}

		if err := ctx.Err(); err != nil {
			return finish(domain.AttemptStopped, domain.FailureStopped, err.Error(), false)
		}
		if !spec.Deadline.After(now()) {
			reportError(configuredCode(e.RetryErrorCode, reporting.CodeExecutorRetryExhausted), lastAttemptErr)
			return finish(domain.AttemptFailed, domain.FailureDeadline, "deadline elapsed", false)
		}
		emit("request", "starting purchase API transaction", 0, false)
		outcome := applyHTTPStatusCode(e.Backend.Attempt(ctx, spec))
		if outcome.SubOrders != nil {
			result.SubOrders = append([]domain.SubOrderResult(nil), outcome.SubOrders...)
			result.Partial = hasPartialSuccess(result.SubOrders)
		}
		classification := e.Classifier.Classify(outcome)
		attemptErr := outcome.Err
		if outcome.Code != 0 && e.UpstreamNamespace != "" &&
			(attemptErr == nil || (outcome.Code != -1 && outcome.Code != -999)) {
			attemptErr = &OutcomeError{
				Namespace: e.UpstreamNamespace,
				Code:      outcome.Code,
				Message:   outcome.Message,
				Retryable: classification.Retryable,
				Cause:     outcome.Err,
			}
		}
		if attemptErr != nil {
			lastAttemptErr = attemptErr
		}
		message := outcome.Message
		if outcome.Err != nil {
			if message != "" {
				message += ": "
			}
			message += outcome.Err.Error()
		}
		if message == "" {
			message = "purchase API returned no order"
		}
		// Format as "msg (code)" for frontend, except for transient 429/412.
		if outcome.Code != 429 && outcome.Code != 412 && outcome.Err == nil {
			message = fmt.Sprintf("%s (%d)", message, outcome.Code)
		}
		emit("response", message, outcome.Code, classification.Retryable)
		if outcome.OrderID != "" && classification.Reason == domain.FailureNone && !classification.Retryable {
			result.Success, result.OrderID = true, outcome.OrderID
			result.PaymentURL = outcome.PaymentURL
			result.PaymentExpire = outcome.PaymentExpire
			result.OrderTime = outcome.OrderTime
			return finish(domain.AttemptSucceeded, domain.FailureNone, outcome.Message, false)
		}
		if !classification.Retryable {
			reportError(configuredCode(e.AttemptErrorCode, reporting.CodeExecutorAttemptFailed), attemptErr)
			message := outcome.Message
			if message == "" && outcome.Err != nil {
				message = outcome.Err.Error()
			}
			return finish(domain.AttemptFailed, classification.Reason, message, false)
		}
		wait := interval
		if classification.Backoff > wait {
			wait = classification.Backoff
		}
		if remaining := spec.Deadline.Sub(now()); wait > remaining {
			wait = remaining
		}
		const cooldownThreshold = 10 * time.Second
		if wait >= cooldownThreshold {
			cooldownEnd := now().Add(wait)
			emitCooldown("cooldown", fmt.Sprintf("cooling down for %s", wait.Truncate(time.Second)), outcome.Code, true, cooldownEnd)
		} else {
			emit("retry", "retrying after "+wait.String(), outcome.Code, true)
		}
		if err := e.Clock.Sleep(ctx, wait); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return finish(domain.AttemptStopped, domain.FailureStopped, err.Error(), false)
		}
	}
}

func hasPartialSuccess(subOrders []domain.SubOrderResult) bool {
	succeeded := 0
	for _, subOrder := range subOrders {
		if subOrder.State == domain.SubOrderSucceeded {
			succeeded++
		}
	}
	return succeeded > 0 && succeeded < len(subOrders)
}
