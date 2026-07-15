package main

import (
	"bilibili-ticket-golang/lib/reporting"
	"bilibili-ticket-golang/lib/tasklog"
)

// TaskLogService exposes task-log queries to Wails without coupling the core
// tasklog package to the GUI runtime.
type TaskLogService struct {
	broker *tasklog.LogBroker
}

func NewTaskLogService(broker *tasklog.LogBroker) *TaskLogService {
	return &TaskLogService{broker: broker}
}

func (svc *TaskLogService) PutLog(taskID string, level tasklog.LogLevel, message string) {
	svc.broker.PutLog(taskID, level, message)
}

func (svc *TaskLogService) GetHistory(taskID string) []tasklog.LogEntry {
	return svc.broker.GetHistory(taskID)
}

func (svc *TaskLogService) GetRecentLogs() map[string]tasklog.LogEntry {
	return svc.broker.GetRecentLogs()
}

func (svc *TaskLogService) ClearHistory(taskID string) {
	reporting.ReportAction(reporting.ActionTaskLogClear)
	svc.broker.ClearHistory(taskID)
}

func (svc *TaskLogService) GetPersistedTaskIDs() []string {
	return svc.broker.GetPersistedTaskIDs()
}

func (svc *TaskLogService) FlushLogs() {
	svc.broker.FlushLogs()
}
