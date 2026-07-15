package bws_service

import (
	"bilibili-ticket-golang/lib/scheduler"
	"bilibili-ticket-golang/lib/tasklog"
)

type RunningStat = scheduler.RunningStat

const (
	StatWaiting = scheduler.StatWaiting
	StatPending = scheduler.StatPending
	StatSuccess = scheduler.StatSuccess
	StatFailed  = scheduler.StatFailed
	StatError   = scheduler.StatError
)

type LogEntry = tasklog.LogEntry
type LogLevel = tasklog.LogLevel

const (
	LogDebug   = tasklog.LogDebug
	LogInfo    = tasklog.LogInfo
	LogWarn    = tasklog.LogWarn
	LogError   = tasklog.LogError
	LogSuccess = tasklog.LogSuccess
)
