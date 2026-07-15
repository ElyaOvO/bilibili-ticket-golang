package scheduler

import (
	"sync"
	"time"
)

// DynamicScheduler manages multiple timed tasks with a global time offset.
type DynamicScheduler struct {
	tasks        map[string]ITask
	globalOffset time.Duration
	mutex        sync.RWMutex
}

// TaskStatus represents the current status of a task for external reporting.
type TaskStatus struct {
	TargetTime   time.Time
	AdjustedTime time.Time
	Remaining    time.Duration
	Stat         RunningStat
	Error        error
}

// NewDynamicScheduler creates a new DynamicScheduler.
func NewDynamicScheduler() *DynamicScheduler {
	return &DynamicScheduler{
		tasks:        make(map[string]ITask),
		globalOffset: 0,
	}
}

// SetGlobalOffset updates the global time offset, rescheduling all waiting tasks.
// SetGlobalOffset updates the global time offset, rescheduling all waiting tasks
// whose remaining time is greater than 10 seconds. Tasks within 10 seconds of
// execution are left untouched to avoid disturbing precise timing.
func (ds *DynamicScheduler) SetGlobalOffset(offset time.Duration) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	oldOffset := ds.globalOffset
	ds.globalOffset = offset

	for _, task := range ds.tasks {
		if task.GetStat() == StatWaiting && time.Until(task.GetTargetTime().Add(oldOffset)) > 10*time.Second {
			task.RescheduleWithNewOffset(offset)
		}
	}
}

// GetGlobalOffset returns the current global offset.
func (ds *DynamicScheduler) GetGlobalOffset() time.Duration {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()
	return ds.globalOffset
}

// AddTask adds a one-shot scheduled task.
// Returns true if the task was added, false if a task with the same ID is
// already registered.
func (ds *DynamicScheduler) AddTask(task ITask) bool {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	if _, exists := ds.tasks[task.GetID()]; exists {
		return false
	}
	ds.tasks[task.GetID()] = task
	task.Start(ds.globalOffset)
	return true
}

// HasTask reports whether a task with the given ID is currently registered.
func (ds *DynamicScheduler) HasTask(id string) bool {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()
	_, exists := ds.tasks[id]
	return exists
}

// RemoveTask removes a task by ID.
func (ds *DynamicScheduler) RemoveTask(taskID string) {
	ds.mutex.Lock()
	task := ds.tasks[taskID]
	delete(ds.tasks, taskID)
	ds.mutex.Unlock()
	if task != nil {
		task.Stop()
	}
}

// RemoveTaskAndStream removes a task by ID, stops it outside the scheduler
// lock, and then closes associated resources. Stopping outside the lock avoids
// deadlocking when a task's terminal callback re-enters the scheduler.
func (ds *DynamicScheduler) RemoveTaskAndStream(taskID string, onRemove func()) {
	ds.mutex.Lock()
	task := ds.tasks[taskID]
	delete(ds.tasks, taskID)
	ds.mutex.Unlock()
	if task != nil {
		task.Stop()
	}
	if onRemove != nil {
		onRemove()
	}
}

// RemoveTaskSilent removes a task by ID using StopSilent (no onComplete, no
// persisted stat update) and invokes onRemove after deletion. Used by
// ReorderTickets to swap the running task without marking it as Failed.
func (ds *DynamicScheduler) RemoveTaskSilent(taskID string, onRemove func()) {
	ds.mutex.Lock()
	task := ds.tasks[taskID]
	delete(ds.tasks, taskID)
	ds.mutex.Unlock()
	if task != nil {
		task.StopSilent()
	}
	if onRemove != nil {
		onRemove()
	}
}

// CompleteTask removes one terminal task and then releases its associated
// resources. It is safe to call from the task's completion callback.
func (ds *DynamicScheduler) CompleteTask(taskID string, onRemove func()) bool {
	ds.mutex.Lock()
	task, exists := ds.tasks[taskID]
	if !exists || task.GetStat() <= StatPending {
		ds.mutex.Unlock()
		return false
	}
	delete(ds.tasks, taskID)
	ds.mutex.Unlock()
	if onRemove != nil {
		onRemove()
	}
	return true
}

// Close stops and removes every task, releasing per-task resources.
func (ds *DynamicScheduler) Close(onRemove func(string)) {
	ds.mutex.Lock()
	tasks := ds.tasks
	ds.tasks = make(map[string]ITask)
	ds.mutex.Unlock()
	for id, task := range tasks {
		task.Stop()
		if onRemove != nil {
			onRemove(id)
		}
	}
}

// GetTaskStatus returns status for all tasks.
func (ds *DynamicScheduler) GetTaskStatus() map[string]TaskStatus {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	status := make(map[string]TaskStatus)
	for id, task := range ds.tasks {
		adjustedTime := task.GetTargetTime().Add(ds.globalOffset)
		status[id] = TaskStatus{
			TargetTime:   task.GetTargetTime(),
			AdjustedTime: adjustedTime,
			Remaining:    time.Until(adjustedTime),
			Stat:         task.GetStat(),
			Error:        task.GetError(),
		}
	}
	return status
}

// GetTaskCount returns the number of registered tasks.
func (ds *DynamicScheduler) GetTaskCount() int {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()
	return len(ds.tasks)
}

// ForceStartTask immediately executes a task by ID, skipping its timer.
func (ds *DynamicScheduler) ForceStartTask(id string) {
	ds.mutex.RLock()
	task, exists := ds.tasks[id]
	ds.mutex.RUnlock()
	if exists {
		task.ForceStart()
	}
}

// BroadcastInterval updates the retry interval for all running tasks.
func (ds *DynamicScheduler) BroadcastInterval(newInterval time.Duration) {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()
	for _, task := range ds.tasks {
		task.UpdateInterval(newInterval)
	}
}

// BroadcastStartDelay updates the start delay (random jitter) for all running tasks.
func (ds *DynamicScheduler) BroadcastStartDelay(newDelay time.Duration) {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()
	for _, task := range ds.tasks {
		task.UpdateStartDelay(newDelay)
	}
}

// CleanupCompletedTasks removes tasks that have finished (success, failed, or error).
func (ds *DynamicScheduler) CleanupCompletedTasks() {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	for id, task := range ds.tasks {
		stat := task.GetStat()
		if stat == StatSuccess || stat == StatFailed || stat == StatError {
			delete(ds.tasks, id)
		}
	}
}
