package scheduler

import (
	"testing"
	"time"
)

type schedulerTestTask struct {
	id      string
	stat    RunningStat
	stopped bool
}

func (t *schedulerTestTask) GetID() string                         { return t.id }
func (t *schedulerTestTask) GetTargetTime() time.Time              { return time.Now() }
func (t *schedulerTestTask) Start(time.Duration)                   {}
func (t *schedulerTestTask) ForceStart()                           {}
func (t *schedulerTestTask) Stop()                                 { t.stopped = true }
func (t *schedulerTestTask) StopSilent()                           { t.stopped = true }
func (t *schedulerTestTask) GetStat() RunningStat                  { return t.stat }
func (t *schedulerTestTask) GetError() error                       { return nil }
func (t *schedulerTestTask) UpdateInterval(time.Duration)          {}
func (t *schedulerTestTask) UpdateStartDelay(time.Duration)        {}
func (t *schedulerTestTask) rescheduleWithNewOffset(time.Duration) {}

func TestCompleteTaskReleasesTerminalTask(t *testing.T) {
	scheduler := NewDynamicScheduler()
	task := &schedulerTestTask{id: "done", stat: StatSuccess}
	if !scheduler.AddTask(task) {
		t.Fatal("add task")
	}
	released := false
	if !scheduler.CompleteTask(task.id, func() { released = true }) {
		t.Fatal("terminal task was not removed")
	}
	if scheduler.GetTaskCount() != 0 || !released {
		t.Fatalf("count=%d released=%v", scheduler.GetTaskCount(), released)
	}
}

func TestSchedulerCloseStopsAllTasks(t *testing.T) {
	scheduler := NewDynamicScheduler()
	task := &schedulerTestTask{id: "waiting", stat: StatWaiting}
	scheduler.AddTask(task)
	scheduler.Close(nil)
	if !task.stopped || scheduler.GetTaskCount() != 0 {
		t.Fatalf("stopped=%v count=%d", task.stopped, scheduler.GetTaskCount())
	}
}
