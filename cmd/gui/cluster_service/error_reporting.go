package cluster_service

import "bilibili-ticket-golang/lib/reporting"

// reportBiliError is called at the service boundary that owns the failed
// operation. Lower-level biliutils code only returns typed/wrapped errors and
// must not report them itself, otherwise a single failure is uploaded twice.
func reportBiliError(code, operation string, err error) {
	if err != nil {
		reporting.ReportErrorOp(code, operation, err)
	}
}

func reportAction(action string) {
	reporting.ReportAction(action)
}
