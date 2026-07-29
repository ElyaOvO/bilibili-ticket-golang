package executor

import (
	"testing"
	"time"

	"bilibili-ticket-golang/lib/biliutils/token"
	"bilibili-ticket-golang/lib/models/bili/api"
	response "bilibili-ticket-golang/lib/models/bili/response"
)

type hotTokenGeneratorStub struct{}

func (hotTokenGeneratorStub) GenerateTokenPrepareStage() string         { return "hot-prepare" }
func (hotTokenGeneratorStub) GenerateTokenCreateStage(time.Time) string { return "hot-create" }
func (hotTokenGeneratorStub) IsHotProject() bool                        { return true }

func TestEmitHotProjectMismatchEvent(t *testing.T) {
	backend := &BilibiliBackend{}
	var event Event
	backend.SetEventSink(func(received Event) { event = received })
	backend.emitHotProjectMismatch(false, true, "restart_with_hot_project_flow")

	if event.Stage != "hot_project_mismatch" ||
		event.Message != "projectInfoHotProject=false confirmInfoHotProject=true action=restart_with_hot_project_flow" ||
		event.Code != 0 || event.Retryable {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func TestPrepareWithConfirmHotProjectRestartsFalseToTrue(t *testing.T) {
	calls := 0
	var mismatchAction string
	tokens, confirm, generator, err := prepareWithConfirmHotProject(
		token.NewNormalTokenGenerator(),
		func() token.ICTokenGenerator { return hotTokenGeneratorStub{} },
		func(generator token.ICTokenGenerator) (*response.RequestTokenAndPToken, *api.ConfirmStruct, error) {
			calls++
			return &response.RequestTokenAndPToken{RequestToken: generator.GenerateTokenPrepareStage()},
				&api.ConfirmStruct{HotProject: true}, nil
		},
		func(currentHot, confirmHot bool, action string) {
			if currentHot || !confirmHot {
				t.Fatalf("unexpected mismatch transition: %t -> %t", currentHot, confirmHot)
			}
			mismatchAction = action
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || !generator.IsHotProject() || !confirm.HotProject || tokens.RequestToken != "hot-prepare" || mismatchAction != "restart_with_hot_project_flow" {
		t.Fatalf("calls=%d hot=%v confirmHot=%v token=%q action=%q", calls, generator.IsHotProject(), confirm.HotProject, tokens.RequestToken, mismatchAction)
	}
}

func TestPrepareWithConfirmHotProjectContinuesTrueToFalse(t *testing.T) {
	calls := 0
	var mismatchAction string
	tokens, confirm, generator, err := prepareWithConfirmHotProject(
		hotTokenGeneratorStub{},
		func() token.ICTokenGenerator {
			t.Fatal("true-to-false must not restart preparation")
			return nil
		},
		func(generator token.ICTokenGenerator) (*response.RequestTokenAndPToken, *api.ConfirmStruct, error) {
			calls++
			return &response.RequestTokenAndPToken{RequestToken: generator.GenerateTokenPrepareStage()},
				&api.ConfirmStruct{HotProject: false}, nil
		},
		func(currentHot, confirmHot bool, action string) {
			if !currentHot || confirmHot {
				t.Fatalf("unexpected mismatch transition: %t -> %t", currentHot, confirmHot)
			}
			mismatchAction = action
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || generator.IsHotProject() || confirm.HotProject || tokens.RequestToken != "hot-prepare" || mismatchAction != "continue_with_normal_flow" {
		t.Fatalf("calls=%d hot=%v confirmHot=%v token=%q action=%q", calls, generator.IsHotProject(), confirm.HotProject, tokens.RequestToken, mismatchAction)
	}
}
