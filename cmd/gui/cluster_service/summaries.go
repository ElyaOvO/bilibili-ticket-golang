package cluster_service

import (
	"context"
	"strings"
	"time"

	"bilibili-ticket-golang/cluster/domain"
	"bilibili-ticket-golang/lib/global"
)

// ListAccountSummaries returns only the account data needed by account pickers
// and the account management page.
func (s *ClusterService) ListAccountSummaries() ([]AccountSummary, error) {
	accounts, err := s.repository.ListAccounts(context.Background())
	if err != nil {
		return nil, err
	}
	return summarizeAccounts(accounts), nil
}

// GetBuyerListSnapshot returns buyer mappings and their account labels without
// loading workers, tasks, intents, or attempt history.
func (s *ClusterService) GetBuyerListSnapshot() (BuyerListSnapshot, error) {
	ctx := context.Background()
	accounts, err := s.repository.ListAccounts(ctx)
	if err != nil {
		return BuyerListSnapshot{}, err
	}
	buyers, err := s.listBuyerSummaries(ctx, accounts)
	if err != nil {
		return BuyerListSnapshot{}, err
	}
	return BuyerListSnapshot{
		Buyers:   buyers,
		Accounts: summarizeAccounts(accounts),
	}, nil
}

// GetWorkerListSnapshot reads persisted workers and cached health metadata. It
// never performs network I/O.
func (s *ClusterService) GetWorkerListSnapshot() (WorkerListSnapshot, error) {
	workers, err := s.repository.ListWorkers(context.Background())
	if err != nil {
		return WorkerListSnapshot{}, err
	}
	return WorkerListSnapshot{
		Workers:         s.summarizeWorkers(workers),
		EmployerVersion: global.GitCommit,
	}, nil
}

// ListTaskGroups returns sidebar data without constructing a full snapshot.
func (s *ClusterService) ListTaskGroups() ([]domain.TaskGroup, error) {
	return s.repository.ListTaskGroups(context.Background())
}

func summarizeAccounts(accounts []domain.Account) []AccountSummary {
	result := make([]AccountSummary, 0, len(accounts))
	now := time.Now()
	for _, account := range accounts {
		summary := AccountSummary{
			ID:                account.ID,
			Name:              account.Name,
			Tags:              normalizeAccountTags(account.Tags),
			Enabled:           account.Enabled,
			VipStatus:         account.VipStatus,
			CredentialVersion: account.Credentials.Version,
		}
		if !account.CooldownUntil.IsZero() {
			cooldown := account.CooldownUntil
			summary.CooldownUntil = &cooldown
			if account.CooldownUntil.After(now) {
				summary.CooldownReason = "账号风控触发，冷却 5 分钟"
			}
		}
		result = append(result, summary)
	}
	return result
}

func (s *ClusterService) listBuyerSummaries(ctx context.Context, accounts []domain.Account) ([]BuyerWithAccounts, error) {
	buyers, err := s.repository.ListLogicalBuyers(ctx)
	if err != nil {
		return nil, err
	}
	mappings, err := s.repository.ListBuyerMappings(ctx)
	if err != nil {
		return nil, err
	}

	accountByID := make(map[string]domain.Account, len(accounts))
	for _, account := range accounts {
		accountByID[account.ID] = account
	}
	buyerAccounts := make(map[string][]BuyerAccountBadge)
	for _, mapping := range mappings {
		account := accountByID[mapping.AccountID]
		buyerAccounts[mapping.LogicalBuyerID] = append(buyerAccounts[mapping.LogicalBuyerID], BuyerAccountBadge{
			AccountID:   mapping.AccountID,
			AccountName: account.Name,
			UID:         strings.TrimPrefix(mapping.AccountID, "bili-"),
		})
	}

	result := make([]BuyerWithAccounts, len(buyers))
	for i, buyer := range buyers {
		mappedAccounts := buyerAccounts[buyer.LogicalID]
		if mappedAccounts == nil {
			mappedAccounts = make([]BuyerAccountBadge, 0)
		}
		result[i] = BuyerWithAccounts{Buyer: buyer, Accounts: mappedAccounts}
	}
	return result, nil
}

func (s *ClusterService) summarizeWorkers(workers []domain.WorkerNode) []WorkerSummary {
	result := make([]WorkerSummary, 0, len(workers))
	now := time.Now()
	for _, node := range workers {
		summary := WorkerSummary{
			ID:               node.ID,
			Name:             node.Name,
			Address:          node.Address,
			Type:             node.Type,
			Tags:             workerEffectiveTags(node),
			Enabled:          node.Enabled,
			SkipVersionCheck: node.SkipVersionCheck,
			Healthy:          s.client.IsHealthy(node.ID),
		}
		if heartbeat, ok := s.client.LastHeartbeat(node.ID); ok {
			summary.LastHeartbeatAt = &heartbeat
			summary.LastHeartbeatLatency = now.Sub(heartbeat).Milliseconds()
		}
		if !summary.Healthy {
			info := s.dispatcher.WorkerCooldown(node.ID)
			if info.CooledDown {
				summary.Cooldown = WorkerCooldownInfo{
					CooledDown:      true,
					CooldownEnd:     info.CooldownEnd,
					StartedAt:       info.StartedAt,
					Reason:          info.Reason,
					RemainingMs:     max(0, info.CooldownEnd.Sub(now).Milliseconds()),
					TotalDurationMs: info.TotalDurationMs,
				}
			}
		}
		if health, ok := s.client.CachedHealth(node.ID); ok {
			summary.ActiveAttemptID = health.ActiveAttemptID
			summary.Version = health.Version
			summary.BilibiliOffsetMs = health.BilibiliOffsetMs
			summary.NtpOffsetMs = health.NtpOffsetMs
			summary.VersionBlocked = !health.ProtocolVersionOK
		}
		result = append(result, summary)
	}
	return result
}
