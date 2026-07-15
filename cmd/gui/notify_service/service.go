package notify_service

import (
	"errors"
	"fmt"
	"sync"

	"bilibili-ticket-golang/cmd/gui/i18n"
	"bilibili-ticket-golang/cmd/gui/store/configuration"
	"bilibili-ticket-golang/lib/notify"
	"bilibili-ticket-golang/lib/reporting"
)

// FrontendNotifyChannel mirrors configuration.NotifyChannel for Wails.
type FrontendNotifyChannel struct {
	Index   int               `json:"index"`
	Type    string            `json:"type"`
	Name    string            `json:"name"`
	Enabled bool              `json:"enabled"`
	Params  map[string]string `json:"params"`
}

type SelectOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type FieldCondition struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type NotifyChannelFieldMeta struct {
	Key         string          `json:"key"`
	Label       string          `json:"label"`
	Type        string          `json:"type"`
	Placeholder string          `json:"placeholder"`
	Required    bool            `json:"required"`
	Hint        string          `json:"hint"`
	Default     string          `json:"default"`
	Options     []SelectOption  `json:"options"`
	DependsOn   *FieldCondition `json:"dependsOn"`
}

type NotifyChannelTypeMeta struct {
	Type   string                   `json:"type"`
	Label  string                   `json:"label"`
	Fields []NotifyChannelFieldMeta `json:"fields"`
}

// NotifyService owns GUI notification-channel configuration and metadata.
type NotifyService struct {
	notifier *notify.MultiNotifier
	data     *configuration.NotifyChannelData
	store    *configuration.DataStorage
	mu       sync.Mutex
}

func New(notifier *notify.MultiNotifier, data *configuration.NotifyChannelData, store *configuration.DataStorage) *NotifyService {
	return &NotifyService{notifier: notifier, data: data, store: store}
}

func (svc *NotifyService) GetNotifyChannels() []FrontendNotifyChannel {
	channels := svc.data.GetAll()
	result := make([]FrontendNotifyChannel, len(channels))
	for i, ch := range channels {
		result[i] = FrontendNotifyChannel{Index: i, Type: ch.Type, Name: ch.Name, Enabled: ch.Enabled, Params: ch.Params}
	}
	return result
}

func (svc *NotifyService) AddNotifyChannel(ch FrontendNotifyChannel) (int, error) {
	reporting.ReportAction(reporting.ActionNotifyChannelAdd)
	svc.mu.Lock()
	defer svc.mu.Unlock()
	nc := configuration.NotifyChannel{Type: ch.Type, Name: ch.Name, Enabled: ch.Enabled, Params: ch.Params}
	n, err := nc.ToNotifier()
	if err != nil {
		return -1, fmt.Errorf("%s: %w", i18n.T("notify.error.create", nil), err)
	}
	index := svc.data.Add(nc)
	svc.notifier.Add(n)
	svc.persistLocked()
	return index, nil
}

func (svc *NotifyService) RemoveNotifyChannel(index int) error {
	reporting.ReportAction(reporting.ActionNotifyChannelRemove)
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if !svc.data.Remove(index) {
		return errors.New(i18n.T("notify.error.index_not_found", map[string]interface{}{"Index": index}))
	}
	svc.rebuildLocked()
	svc.persistLocked()
	return nil
}

func (svc *NotifyService) UpdateNotifyChannel(index int, ch FrontendNotifyChannel) error {
	reporting.ReportAction(reporting.ActionNotifyChannelUpdate)
	svc.mu.Lock()
	defer svc.mu.Unlock()
	nc := configuration.NotifyChannel{Type: ch.Type, Name: ch.Name, Enabled: ch.Enabled, Params: ch.Params}
	if _, err := nc.ToNotifier(); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("notify.error.update", nil), err)
	}
	if !svc.data.Update(index, nc) {
		return errors.New(i18n.T("notify.error.index_not_found", map[string]interface{}{"Index": index}))
	}
	svc.rebuildLocked()
	svc.persistLocked()
	return nil
}

func (svc *NotifyService) TestNotifyChannel(index int) error {
	reporting.ReportAction(reporting.ActionNotifyChannelTest)
	channels := svc.data.GetAll()
	if index < 0 || index >= len(channels) {
		return errors.New(i18n.T("notify.error.index_not_found", map[string]interface{}{"Index": index}))
	}
	n, err := channels[index].ToNotifier()
	if err != nil {
		return err
	}
	if ok, message := n.Test(); !ok {
		return errors.New(i18n.T("notify.error.test_failed", map[string]interface{}{"Error": message}))
	}
	return nil
}

func (svc *NotifyService) GetNotifyChannelTypes() []NotifyChannelTypeMeta {
	return notifyChannelTypes()
}

func (svc *NotifyService) rebuildLocked() {
	svc.notifier.Clear()
	for _, channel := range svc.data.GetAll() {
		if !channel.Enabled {
			continue
		}
		n, err := channel.ToNotifier()
		if err == nil {
			svc.notifier.Add(n)
		}
	}
}

func (svc *NotifyService) persistLocked() {
	if svc.store != nil {
		if err := svc.store.Save(); err != nil {
			println("Failed to persist notify channels:", err.Error())
		}
	}
}
