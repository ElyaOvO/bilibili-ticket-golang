package bws_service

import (
	"errors"
	"fmt"
	"time"

	"bilibili-ticket-golang/cmd/gui/i18n"
	"bilibili-ticket-golang/cmd/gui/store/configuration"
	"bilibili-ticket-golang/lib/biliutils"
	"bilibili-ticket-golang/lib/notify"
	"bilibili-ticket-golang/lib/reporting"
	"bilibili-ticket-golang/lib/scheduler"
	"bilibili-ticket-golang/lib/tasklog"
)

// FrontendBWSEntry mirrors configuration.BWSEntry for Wails serialization.
type FrontendBWSEntry struct {
	Hash          string `json:"hash"`
	ActivityID    int    `json:"activityId"`
	TicketNo      string `json:"ticketNo"`
	ActivityTitle string `json:"activityTitle"`
	ReserveTime   int64  `json:"reserveTime"`
	ReserveDate   string `json:"reserveDate"`
	Expire        int64  `json:"expire"`
	StartDelayMs  int    `json:"startDelayMs"`
	LoopDelayMs   int    `json:"loopDelayMs"`
	Stat          int    `json:"stat"`
}

// BWSService owns legacy local BWS reservation scheduling.
type BWSService struct {
	scheduler *scheduler.DynamicScheduler
	client    *biliutils.BiliClient
	logs      *tasklog.LogBroker
	data      *configuration.BWSScheduler
	notifier  *notify.MultiNotifier
	store     *configuration.DataStorage
}

func New(client *biliutils.BiliClient, logs *tasklog.LogBroker, data *configuration.BWSScheduler, notifier *notify.MultiNotifier, store *configuration.DataStorage) *BWSService {
	return &BWSService{
		scheduler: scheduler.NewDynamicScheduler(),
		client:    client,
		logs:      logs,
		data:      data,
		notifier:  notifier,
		store:     store,
	}
}

func (svc *BWSService) AddBWSEntry(entry FrontendBWSEntry) (string, error) {
	reporting.ReportAction(reporting.ActionBWSLocalEntryAdd)
	value := configuration.BWSEntry{
		ActivityID: entry.ActivityID, TicketNo: entry.TicketNo, ActivityTitle: entry.ActivityTitle,
		ReserveTime: entry.ReserveTime, ReserveDate: entry.ReserveDate, Expire: entry.Expire,
		StartDelayMs: entry.StartDelayMs, LoopDelayMs: entry.LoopDelayMs,
	}
	hash := value.Hash()
	if !value.Valid() {
		return "", errors.New("BWS entry is invalid or expired")
	}
	if !svc.data.AddEntry(value) {
		return "", errors.New(i18n.T("bws.error.duplicate", nil))
	}
	svc.persist("BWS entry")
	return hash, nil
}

func (svc *BWSService) AddBWSTask(hash string) error {
	reporting.ReportAction(reporting.ActionBWSLocalTaskAdd)
	return svc.addBWSTask(hash)
}

func (svc *BWSService) addBWSTask(hash string) error {
	entries := svc.data.GetEntriesNoMutate()
	var entry *configuration.BWSEntry
	for i := range entries {
		if entries[i].Hash() == hash {
			entry = &entries[i]
			break
		}
	}
	if entry == nil {
		return fmt.Errorf("BWS entry not found: %s", hash)
	}
	if !entry.Valid() {
		return errors.New("BWS entry is expired or invalid")
	}
	if svc.scheduler.HasTask(hash) {
		return errors.New("BWS task already exists")
	}

	logCh := svc.logs.CreateStream(hash)
	var notifyFn func(string)
	if svc.notifier != nil && svc.notifier.Count() > 0 {
		notifyFn = func(message string) { svc.notifier.Notify(message) }
	}
	task, err := NewBWSTask(svc.client, *entry, notifyFn, logCh, func(stat scheduler.RunningStat, _ bool) {
		svc.data.UpdateEntryStat(hash, int(stat))
		svc.scheduler.CompleteTask(hash, func() { svc.logs.CloseStream(hash) })
	})
	if err != nil {
		return fmt.Errorf("create BWS task: %w", err)
	}
	task.ID = hash
	task.TargetTime = time.Unix(entry.ReserveTime, 0)
	if !svc.scheduler.AddTask(task) {
		return errors.New("BWS task already exists")
	}
	return nil
}

func (svc *BWSService) Close() {
	svc.scheduler.Close(func(taskID string) { svc.logs.CloseStream(taskID) })
}

func (svc *BWSService) RemoveBWSEntry(hash string) {
	reporting.ReportAction(reporting.ActionBWSLocalEntryRemove)
	svc.scheduler.RemoveTaskAndStream(hash, func() { svc.logs.CloseStream(hash) })
	svc.data.RemoveEntryByHash(hash)
	svc.persist("BWS entry removal")
}

func (svc *BWSService) GetBWSEntries() []FrontendBWSEntry {
	entries := svc.data.GetEntriesNoMutate()
	result := make([]FrontendBWSEntry, len(entries))
	for i, entry := range entries {
		result[i] = FrontendBWSEntry{
			Hash: entry.Hash(), ActivityID: entry.ActivityID, TicketNo: entry.TicketNo,
			ActivityTitle: entry.ActivityTitle, ReserveTime: entry.ReserveTime, ReserveDate: entry.ReserveDate,
			Expire: entry.Expire, StartDelayMs: entry.StartDelayMs, LoopDelayMs: entry.LoopDelayMs, Stat: entry.Stat,
		}
	}
	return result
}

func (svc *BWSService) ReloadBWSTasks() {
	existing := svc.scheduler.GetTaskStatus()
	for _, entry := range svc.data.GetEntriesNoMutate() {
		hash := entry.Hash()
		if _, found := existing[hash]; found || !entry.Valid() {
			continue
		}
		if entry.Stat == int(scheduler.StatSuccess) || entry.Stat == int(scheduler.StatFailed) || entry.Stat == int(scheduler.StatError) {
			continue
		}
		_ = svc.addBWSTask(hash)
	}
}

func (svc *BWSService) persist(subject string) {
	if svc.store != nil {
		if err := svc.store.Save(); err != nil {
			println("Failed to persist "+subject+":", err.Error())
		}
	}
}
