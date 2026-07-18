// Package cloudcontrol defines the public boundary between application code
// and the private cloud-control implementation.
package cloudcontrol

import (
	"errors"

	"bilibili-ticket-golang/lib/reporting"
)

var (
	ErrMachineBanned = errors.New("machine is banned")
	ErrUnavailable   = errors.New("cloud-control is unavailable")
)

type Snapshot struct {
	MachineID string
}

type ClientType uint8

const (
	EmployerClient ClientType = 1
	WorkerClient   ClientType = 2
)

func (c ClientType) Valid() bool {
	return c == EmployerClient || c == WorkerClient
}

type Config struct {
	DSN                 string
	CapabilityPublicKey string
	Timeout             string
	SkipSSLCheck        string
	ClientType          ClientType
}

// Controller is injected at the application composition root. Business
// packages must depend on this contract, never on the transport implementation.
type Controller interface {
	reporting.Reporter
	Bootstrap(checkpoint string) (Snapshot, error)
	CheckFeature(feature, checkpoint string) error
}
