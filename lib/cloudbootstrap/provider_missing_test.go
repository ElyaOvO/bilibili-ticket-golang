//go:build !cloudcontrol && !production

package cloudbootstrap

import (
	"testing"

	"bilibili-ticket-golang/lib/cloudcontrol"
)

func TestDevelopmentControllerAllowsStartupAndFeatures(t *testing.T) {
	controller, err := New(cloudcontrol.Config{ClientType: cloudcontrol.EmployerClient})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if _, err = controller.Bootstrap("employer_startup"); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if err = controller.CheckFeature("cluster", "submit"); err != nil {
		t.Fatalf("CheckFeature() error = %v", err)
	}
	if err = controller.ReportAction("app_start"); err != nil {
		t.Fatalf("ReportAction() error = %v", err)
	}
	if err = controller.ReportError("test", nil); err != nil {
		t.Fatalf("ReportError() error = %v", err)
	}
}
