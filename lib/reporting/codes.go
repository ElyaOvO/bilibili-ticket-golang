package reporting

// Stable top-level error codes. Domain packages should define their own codes
// alongside the operation that can fail.
const (
	CodeGUICallError    = "GUI_CALL_ERROR"
	CodeGUIRuntimeError = "GUI_RUNTIME_ERROR"
	CodeWorkerFatal     = "WORKER_FATAL"
)
