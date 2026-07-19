//go:build !cloudcontrol && (!production || debug)

package cloudbootstrap

import (
	"bilibili-ticket-golang/lib/cloudcontrol"
)

// developmentController keeps local development independent of the private
// cloud-control service. Production builds must select provider_private.go.
type developmentController struct{}

func New(cloudcontrol.Config) (cloudcontrol.Controller, error) {
	return developmentController{}, nil
}

func (developmentController) Bootstrap(string) (cloudcontrol.Snapshot, error) {
	return cloudcontrol.Snapshot{}, nil
}

func (developmentController) CheckFeature(string, string) error {
	return nil
}

func (developmentController) ReportError(string, error) error {
	return nil
}

func (developmentController) ReportAction(string) error {
	return nil
}
