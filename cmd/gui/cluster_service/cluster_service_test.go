package cluster_service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"bilibili-ticket-golang/cluster/dispatcher"
	"bilibili-ticket-golang/cluster/domain"
	clusterstorage "bilibili-ticket-golang/cluster/storage"
)

func testClusterService(t *testing.T) *ClusterService {
	t.Helper()
	repository, err := clusterstorage.Open(filepath.Join(t.TempDir(), "employer.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	return NewClusterService(repository)
}

func document(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestClusterEventLogPageUsesServerSideSearchAndPagination(t *testing.T) {
	service := testClusterService(t)
	service.recordEvent(ClusterEvent{Time: time.UnixMilli(1), WorkerID: "alpha", Message: "old"})
	service.recordEvent(ClusterEvent{Time: time.UnixMilli(2), WorkerID: "beta", Message: "middle"})
	service.recordEvent(ClusterEvent{Time: time.UnixMilli(3), WorkerID: "alpha", Message: "new"})

	page := service.GetClusterEventLogPage(2, 1, "alpha")
	if page.Total != 2 || page.Page != 2 || page.PageSize != 1 || len(page.Events) != 1 || page.Events[0].Message != "old" {
		t.Fatalf("unexpected event page: %#v", page)
	}
}

func TestSaveOrderRecordsPersistsEachSubOrderState(t *testing.T) {
	service := testClusterService(t)
	intent := domain.LogicalOrderIntent{ID: "intent", MacroTaskID: "macro"}
	result := domain.ExecutionResult{
		AttemptID: "attempt", State: domain.AttemptPartial, Partial: true,
		SubOrders: []domain.SubOrderResult{
			{BuyerIndex: 0, BuyerID: 7, BuyerName: "A", State: domain.SubOrderSucceeded, OrderID: "order-1"},
			{BuyerIndex: 1, BuyerID: 8, BuyerName: "B", State: domain.SubOrderFailed, Code: 100016},
		},
	}
	records, err := service.saveOrderRecords(intent, result)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].Status != domain.SubOrderSucceeded || records[1].Status != domain.SubOrderFailed || records[1].BuyerID != 8 {
		t.Fatalf("unexpected records: %#v", records)
	}
	stored, err := service.repository.ListOrderRecords(context.Background(), 10)
	if err != nil || len(stored) != 2 {
		t.Fatalf("stored records: %#v, err=%v", stored, err)
	}
}

func TestClusterServiceValidatesRunnableMacroAndPurchaseShape(t *testing.T) {
	service := testClusterService(t)
	if err := service.SaveTaskGroup(`{"id":"group","name":"test"}`); err != nil {
		t.Fatal(err)
	}
	macro := domain.MacroTask{ID: "macro", TaskGroupID: "group", ProjectID: 1, ScreenID: 2, SKUID: 3, EventDay: "2026-07-01", EventDayConfirmed: true, OrderCapacity: 2}
	if err := service.SaveMacro(document(t, macro)); err == nil {
		t.Fatal("expected missing execution window to be rejected")
	}
	macro.StartAt = time.Now().Add(time.Minute)
	macro.Deadline = macro.StartAt.Add(time.Hour)
	if err := service.SaveMacro(document(t, macro)); err != nil {
		t.Fatal(err)
	}
	if err := service.StartMacro(macro.ID); err == nil {
		t.Fatal("starting a macro without purchase groups must fail")
	}
	tooLarge := domain.PurchaseGroup{MacroTaskID: macro.ID, Buyers: []domain.Buyer{{LogicalID: "a"}, {LogicalID: "b"}, {LogicalID: "c"}}}
	if err := service.SavePurchaseGroup(document(t, tooLarge)); err == nil {
		t.Fatal("expected oversized purchase group to be rejected")
	}
	duplicate := domain.PurchaseGroup{MacroTaskID: macro.ID, Buyers: []domain.Buyer{{LogicalID: "a"}, {LogicalID: "a"}}}
	if err := service.SavePurchaseGroup(document(t, duplicate)); err == nil {
		t.Fatal("expected duplicate logical buyer to be rejected")
	}
	valid := domain.PurchaseGroup{MacroTaskID: macro.ID, Buyers: []domain.Buyer{{LogicalID: "a"}, {LogicalID: "b"}}, AllowSplit: true}
	if err := service.SavePurchaseGroup(document(t, valid)); err != nil {
		t.Fatal(err)
	}
	if err := service.StartMacro(macro.ID); err == nil {
		t.Fatal("starting without an eligible account and worker must not silently succeed")
	}
	snapshot, err := service.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Macros) != 1 || len(snapshot.Macros[0].PurchaseGroups) != 1 || len(snapshot.Macros[0].PurchaseGroups[0].Buyers) != 2 {
		t.Fatalf("purchase groups missing from macro summary: %#v", snapshot.Macros)
	}
	macro.Priority = 9
	if err := service.SaveMacro(document(t, macro)); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteMacro(macro.ID); err != nil {
		t.Fatal(err)
	}
	snapshot, err = service.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Macros) != 0 || len(snapshot.Attempts) != 0 {
		t.Fatalf("macro cascade was not removed: %#v", snapshot)
	}
}

func TestClusterSnapshotUsesEmptyBuyerArray(t *testing.T) {
	service := testClusterService(t)
	snapshot, err := service.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Buyers == nil {
		t.Fatal("empty buyers must be represented as an empty array, not null")
	}
}

func TestClusterServiceEditsAndDeletesPurchaseGroups(t *testing.T) {
	service := testClusterService(t)
	if err := service.SaveTaskGroup(`{"id":"group","name":"test"}`); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	macro := domain.MacroTask{ID: "macro", TaskGroupID: "group", ProjectID: 1, ScreenID: 2, SKUID: 3, EventDay: "2026-07-01", EventDayConfirmed: true, OrderCapacity: 4, StartAt: now.Add(time.Minute), Deadline: now.Add(time.Hour)}
	if err := service.SaveMacro(document(t, macro)); err != nil {
		t.Fatal(err)
	}
	createdAt := now.Add(-time.Minute)
	group := domain.PurchaseGroup{ID: "purchase", MacroTaskID: macro.ID, Buyers: []domain.Buyer{{LogicalID: "a", Name: "A"}}, CreatedAt: createdAt}
	if err := service.SavePurchaseGroup(document(t, group)); err != nil {
		t.Fatal(err)
	}
	if err := service.SavePurchaseGroup(`{"id":"purchase","macroTaskId":"macro","buyers":[{"logicalId":"b","name":"B"}],"allowSplit":true,"weight":"3","priority":"2"}`); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	saved := snapshot.Macros[0].PurchaseGroups
	if len(saved) != 1 || saved[0].Buyers[0].LogicalID != "b" || !saved[0].AllowSplit || saved[0].Weight != 3 || saved[0].Priority != 2 || !saved[0].CreatedAt.Equal(createdAt) {
		t.Fatalf("purchase group was not updated in place: %#v", saved)
	}
	if err := service.DeletePurchaseGroup(macro.ID, group.ID); err != nil {
		t.Fatal(err)
	}
	snapshot, err = service.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Macros[0].PurchaseGroups) != 0 {
		t.Fatalf("purchase group was not deleted: %#v", snapshot.Macros[0].PurchaseGroups)
	}
}

func TestClusterServicePlansTaskGroupAndRequiresHealthyWorker(t *testing.T) {
	service := testClusterService(t)
	ctx := context.Background()
	// Add a local worker so StartTaskGroup has something to dispatch to.
	worker := domain.WorkerNode{ID: "w", Name: "test-worker", Type: domain.WorkerTypeLocal, Enabled: true}
	if err := service.repository.PutWorker(ctx, worker); err != nil {
		t.Fatal(err)
	}
	// Add an account mapped to buyer "a".
	account := domain.Account{ID: "acct", Enabled: true, Credentials: domain.Credentials{Version: 1}}
	if err := service.repository.PutAccount(ctx, account, nil); err != nil {
		t.Fatal(err)
	}
	// Map buyer "a" to account "acct".
	mapping := domain.AccountBuyerMapping{AccountID: "acct", LogicalBuyerID: "a", BuyerID: 1}
	if err := service.repository.PutBuyerMapping(ctx, mapping); err != nil {
		t.Fatal(err)
	}
	if err := service.SaveTaskGroup(`{"id":"group","name":"test","accountIds":["acct"]}`); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	macro := domain.MacroTask{ID: "macro", TaskGroupID: "group", ProjectID: 1, ScreenID: 2, SKUID: 3, EventDay: "2026-07-01", EventDayConfirmed: true, OrderCapacity: 4, StartAt: now.Add(time.Minute), Deadline: now.Add(time.Hour)}
	if err := service.SaveMacro(document(t, macro)); err != nil {
		t.Fatal(err)
	}
	group := domain.PurchaseGroup{ID: "purchase", MacroTaskID: macro.ID, Buyers: []domain.Buyer{{LogicalID: "a", Name: "A"}}, CreatedAt: now}
	if err := service.SavePurchaseGroup(document(t, group)); err != nil {
		t.Fatal(err)
	}
	if err := service.StartTaskGroup("group", `["w"]`); err == nil {
		t.Fatal("task group start must not silently succeed without a healthy worker client")
	}
	intents, err := service.repository.ListIntents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(intents) != 1 || intents[0].MacroTaskID != macro.ID || intents[0].Armed || !intents[0].Terminal || intents[0].FailureReason != domain.FailureStopped {
		t.Fatalf("task group start rollback did not persist a stopped intent: %#v", intents)
	}
}

func TestClusterServiceRejectsOverlappingTaskGroupResources(t *testing.T) {
	service := testClusterService(t)
	ctx := context.Background()
	if err := service.SaveTaskGroup(`{"id":"group-a","name":"A"}`); err != nil {
		t.Fatal(err)
	}
	if err := service.SaveTaskGroup(`{"id":"group-b","name":"B","accountIds":["account"],"primaryWorkerIds":["w"]}`); err != nil {
		t.Fatal(err)
	}
	if err := service.repository.PutWorker(ctx, domain.WorkerNode{ID: "w", Name: "worker", Type: domain.WorkerTypeLocal, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := service.dispatcher.ReserveWorkerPools("group-a", []string{"w"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := service.StartTaskGroup("group-b", ""); err == nil {
		t.Fatal("starting a task group with reserved resources owned by another group must be rejected")
	}
}

func TestForceStopTaskGroupClearsInMemoryStoppingAttempt(t *testing.T) {
	service := testClusterService(t)
	ctx := context.Background()
	taskGroup := domain.TaskGroup{ID: "group", Name: "test"}
	if err := service.repository.PutTaskGroup(ctx, taskGroup); err != nil {
		t.Fatal(err)
	}
	macro := domain.MacroTask{ID: "macro", TaskGroupID: taskGroup.ID, EventDay: "2026-07-01", OrderCapacity: 1}
	if err := service.repository.PutMacroTask(ctx, macro); err != nil {
		t.Fatal(err)
	}
	intent, err := domain.NewIntent("intent", macro, domain.PhasePunctual, []domain.Buyer{{LogicalID: "buyer"}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := service.repository.PutIntent(ctx, intent); err != nil {
		t.Fatal(err)
	}
	service.dispatcher.Add(dispatcher.IntentPlan{TaskGroup: taskGroup, Macro: macro, Intent: intent})
	attempt := domain.ExecutionAttempt{ID: "attempt", IntentID: intent.ID, AccountID: "account", WorkerID: "missing-worker", State: domain.AttemptRunning}
	if err := service.repository.PutAttempt(ctx, attempt); err != nil {
		t.Fatal(err)
	}
	if err := service.dispatcher.RestoreAttempt(attempt); err != nil {
		t.Fatal(err)
	}

	if err := service.ForceStopTaskGroup(taskGroup.ID); err != nil {
		t.Fatal(err)
	}
	if service.taskGroupActive(ctx, taskGroup.ID) {
		t.Fatal("force-stopped task group is still active")
	}
	stored, err := service.repository.ListAttempts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0].State != domain.AttemptStopped {
		t.Fatalf("force-stopped attempt was not persisted: %#v", stored)
	}
}

func TestRecoveryStopsOnlyStaleLocalAttempts(t *testing.T) {
	service := testClusterService(t)
	ctx := context.Background()
	if err := service.repository.PutTaskGroup(ctx, domain.TaskGroup{ID: "group"}); err != nil {
		t.Fatal(err)
	}
	macro := domain.MacroTask{ID: "macro", TaskGroupID: "group", EventDay: "2026-07-01", OrderCapacity: 1}
	if err := service.repository.PutMacroTask(ctx, macro); err != nil {
		t.Fatal(err)
	}
	localIntent, err := domain.NewIntent("local-intent", macro, domain.PhasePunctual, []domain.Buyer{{LogicalID: "local-buyer"}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	remoteIntent, err := domain.NewIntent("remote-intent", macro, domain.PhasePunctual, []domain.Buyer{{LogicalID: "remote-buyer"}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, intent := range []domain.LogicalOrderIntent{localIntent, remoteIntent} {
		if err := service.repository.PutIntent(ctx, intent); err != nil {
			t.Fatal(err)
		}
	}
	intents := map[string]domain.LogicalOrderIntent{localIntent.ID: localIntent, remoteIntent.ID: remoteIntent}
	attempts := []domain.ExecutionAttempt{
		{ID: "local-attempt", IntentID: localIntent.ID, WorkerID: "local", State: domain.AttemptRunning},
		{ID: "remote-attempt", IntentID: remoteIntent.ID, WorkerID: "remote", State: domain.AttemptRunning},
	}
	for _, attempt := range attempts {
		if err := service.repository.PutAttempt(ctx, attempt); err != nil {
			t.Fatal(err)
		}
	}
	workers := []domain.WorkerNode{
		{ID: "local", Type: domain.WorkerTypeLocal},
		{ID: "remote", Type: domain.WorkerTypeRemote},
	}

	if err := service.stopStaleLocalAttempts(ctx, workers, intents, attempts); err != nil {
		t.Fatal(err)
	}
	if attempts[0].State != domain.AttemptStopped || intents[localIntent.ID].Armed || !intents[localIntent.ID].Terminal {
		t.Fatalf("local recovery state was not stopped: attempt=%#v intent=%#v", attempts[0], intents[localIntent.ID])
	}
	if attempts[1].State != domain.AttemptRunning || !intents[remoteIntent.ID].Armed || intents[remoteIntent.ID].Terminal {
		t.Fatalf("remote recovery state was changed: attempt=%#v intent=%#v", attempts[1], intents[remoteIntent.ID])
	}
	stored, err := service.repository.ListAttempts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	states := make(map[string]domain.AttemptState, len(stored))
	for _, attempt := range stored {
		states[attempt.ID] = attempt.State
	}
	if states["local-attempt"] != domain.AttemptStopped || states["remote-attempt"] != domain.AttemptRunning {
		t.Fatalf("unexpected persisted recovery states: %#v", states)
	}
}

func TestClusterServiceDeletesIdleResources(t *testing.T) {
	service := testClusterService(t)
	ctx := context.Background()
	if err := service.repository.PutAccount(ctx, domain.Account{ID: "account", Enabled: true}, nil); err != nil {
		t.Fatal(err)
	}
	if err := service.repository.PutWorker(ctx, domain.WorkerNode{ID: "remote", Address: "worker:18080", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := service.repository.PutWorkerTLS(ctx, "remote", domain.WorkerTLSConfig{
		CACertPEM:     []byte("test-ca"),
		ClientCertPEM: []byte("test-cert"),
		ClientKeyPEM:  []byte("test-key"),
	}); err != nil {
		t.Fatal(err)
	}
	beforeDelete, err := service.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(beforeDelete.Accounts) != 1 || beforeDelete.Accounts[0].CooldownUntil != nil {
		t.Fatalf("zero cooldown must be omitted: %#v", beforeDelete.Accounts)
	}
	if err := service.DeleteAccount("account"); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteWorker("remote"); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Accounts) != 0 || len(snapshot.Workers) != 0 {
		t.Fatalf("resources were not deleted: %#v", snapshot)
	}
	if err := service.DeleteWorker("local"); err == nil {
		t.Fatal("local worker deletion must be rejected")
	}
}
