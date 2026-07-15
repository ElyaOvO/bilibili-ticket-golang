package reporting

// Stable top-level error codes submitted by business and process boundaries.
// Keep their string values here so the cloud-control protocol has one registry.
// Fine-grained codes derived from concrete errors (for example BILI_API_412 or
// NETWORK_TIMEOUT) remain the responsibility of process/error_report.go.
const (
	// Executor boundaries.
	CodeExecutorSpecInvalid    = "EXECUTOR_SPEC_INVALID"
	CodeExecutorBackendMissing = "EXECUTOR_BACKEND_MISSING"
	CodeExecutorAttemptFailed  = "EXECUTOR_ATTEMPT_FAILED"
	CodeExecutorRetryExhausted = "EXECUTOR_RETRY_EXHAUSTED"

	// Worker process and RPC boundaries.
	CodeWorkerBackendInitFailed   = "WORKER_BACKEND_INIT_FAILED"
	CodeWorkerResultPersistFailed = "WORKER_RESULT_PERSIST_FAILED"
	CodeWorkerResultAckFailed     = "WORKER_RESULT_ACK_FAILED"
	CodeWorkerAccountQueryFailed  = "WORKER_ACCOUNT_QUERY_FAILED"
	CodeWorkerAccountCreateFailed = "WORKER_ACCOUNT_CREATE_FAILED"
	CodeWorkerAccountDeleteFailed = "WORKER_ACCOUNT_DELETE_FAILED"
	CodeWorkerBWSQueryFailed      = "WORKER_BWS_QUERY_FAILED"
	CodeWorkerBWSBindFailed       = "WORKER_BWS_BIND_FAILED"
	CodeWorkerFatal               = "WORKER_FATAL"

	// Direct Bilibili service boundaries.
	CodeBiliClientInitFailed    = "BILI_CLIENT_INIT_FAILED"
	CodeBiliLoginQRFailed       = "BILI_LOGIN_QR_FAILED"
	CodeBiliLoginSMSFailed      = "BILI_LOGIN_SMS_FAILED"
	CodeBiliLoginPasswordFailed = "BILI_LOGIN_PASSWORD_FAILED"
	CodeBiliSafecenterFailed    = "BILI_LOGIN_SAFECENTER_FAILED"
	CodeBiliCaptchaFailed       = "BILI_CAPTCHA_FAILED"
	CodeBiliCountryListFailed   = "BILI_COUNTRY_LIST_FAILED"
	CodeBiliAccountStatusFailed = "BILI_ACCOUNT_STATUS_FAILED"
	CodeBiliCookieRefreshFailed = "BILI_ACCOUNT_COOKIE_REFRESH_FAILED"
	CodeBiliGaiaFailed          = "BILI_LOGIN_GAIA_FAILED"
	CodeBiliCatalogFailed       = "BILI_CATALOG_FAILED"
	CodeBiliAttemptFailed       = "BILI_ATTEMPT_FAILED"
	CodeBiliRetryExhausted      = "BILI_RETRY_EXHAUSTED"

	// BWS execution boundaries.
	CodeBWSReservationFailed = "BWS_RESERVATION_FAILED"
	CodeBWSAttemptFailed     = "BWS_ATTEMPT_FAILED"
	CodeBWSRetryExhausted    = "BWS_RETRY_EXHAUSTED"
)
