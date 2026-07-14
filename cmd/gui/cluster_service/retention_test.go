package cluster_service

import (
	"fmt"
	"testing"
	"time"
)

func TestCompletedHistoryCachesAreBounded(t *testing.T) {
	service := &ClusterService{
		deployJobs:       make(map[string]*RemoteWorkerDeployJob),
		buyerSyncBatches: make(map[string]*BuyerSyncBatch),
	}
	base := time.Now().Add(-time.Hour)
	for i := 0; i < maxRetainedDeployJobs+10; i++ {
		id := fmt.Sprintf("deploy-%d", i)
		finished := base.Add(time.Duration(i) * time.Second)
		service.deployJobs[id] = &RemoteWorkerDeployJob{ID: id, FinishedAt: &finished}
	}
	service.pruneDeployJobsLocked()
	if len(service.deployJobs) != maxRetainedDeployJobs {
		t.Fatalf("deploy jobs=%d", len(service.deployJobs))
	}

	for i := 0; i < maxRetainedBuyerSyncBatches+10; i++ {
		id := fmt.Sprintf("batch-%d", i)
		service.buyerSyncBatches[id] = &BuyerSyncBatch{
			ID: id, State: BuyerSyncSuccess, UpdatedAt: base.Add(time.Duration(i) * time.Second),
		}
	}
	service.pruneBuyerSyncBatchesLocked()
	if len(service.buyerSyncBatches) != maxRetainedBuyerSyncBatches {
		t.Fatalf("buyer batches=%d", len(service.buyerSyncBatches))
	}
}

func TestOpenedPaymentWindowCacheIsBounded(t *testing.T) {
	service := &ClusterService{openedPaymentWindows: make(map[string]bool)}
	for i := 0; i < maxOpenedPaymentWindows+10; i++ {
		if !service.markPaymentWindowOpened(fmt.Sprintf("order-%d", i)) {
			t.Fatal("new order was treated as duplicate")
		}
	}
	if len(service.openedPaymentWindows) != maxOpenedPaymentWindows {
		t.Fatalf("opened payment windows=%d", len(service.openedPaymentWindows))
	}
}
