package cluster_service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"bilibili-ticket-golang/cluster/domain"
	"bilibili-ticket-golang/cmd/gui/i18n"
)

func (s *ClusterService) ticketSuccessNotification(intent domain.LogicalOrderIntent, result domain.ExecutionResult, records []domain.OrderRecord) string {
	record := s.notificationRecord(intent, result, records)
	user, uid := s.notificationAccount(record)
	return i18n.T("task.notify_success", map[string]interface{}{
		"Project":    record.ProjectName,
		"Screen":     record.ScreenName,
		"Sku":        record.SKUName,
		"Buyer":      notificationBuyers(intent, result),
		"User":       user,
		"UID":        uid,
		"PaymentURL": record.PaymentURL,
	})
}

func (s *ClusterService) notificationRecord(intent domain.LogicalOrderIntent, result domain.ExecutionResult, records []domain.OrderRecord) domain.OrderRecord {
	record := firstNotificationRecord(records)
	if s.repository == nil {
		return record
	}
	ctx := context.Background()
	if record.ProjectName == "" || record.ScreenName == "" || record.SKUName == "" {
		if macros, err := s.repository.ListMacroTasks(ctx); err == nil {
			for _, macro := range macros {
				if macro.ID == intent.MacroTaskID {
					record.ProjectName = macro.ProjectName
					record.ScreenName = macro.ScreenName
					record.SKUName = macro.SKUName
					break
				}
			}
		}
	}
	if record.AccountID == "" && result.AttemptID != "" {
		if attempts, err := s.repository.ListAttempts(ctx); err == nil {
			for _, attempt := range attempts {
				if attempt.ID == result.AttemptID {
					record.AccountID = attempt.AccountID
					break
				}
			}
		}
	}
	return s.hydrateOrderRecordAccount(ctx, record)
}

func firstNotificationRecord(records []domain.OrderRecord) domain.OrderRecord {
	for _, record := range records {
		if record.Status == "" || record.Status == domain.SubOrderSucceeded {
			return record
		}
	}
	if len(records) > 0 {
		return records[0]
	}
	return domain.OrderRecord{}
}

func notificationBuyers(intent domain.LogicalOrderIntent, result domain.ExecutionResult) string {
	buyers := intent.Buyers
	if len(result.SubOrders) > 0 {
		selected := make([]domain.Buyer, 0, len(result.SubOrders))
		seen := make(map[int]bool, len(result.SubOrders))
		for _, child := range result.SubOrders {
			if child.State != domain.SubOrderSucceeded || child.BuyerIndex < 0 || child.BuyerIndex >= len(intent.Buyers) || seen[child.BuyerIndex] {
				continue
			}
			seen[child.BuyerIndex] = true
			selected = append(selected, intent.Buyers[child.BuyerIndex])
		}
		if len(selected) > 0 {
			buyers = selected
		}
	}
	formatted := make([]string, 0, len(buyers))
	for _, buyer := range buyers {
		switch {
		case buyer.Type == 1 && buyer.Tel != "":
			formatted = append(formatted, buyer.Name+" ("+buyer.Tel+")")
		case buyer.BuyerID > 0:
			formatted = append(formatted, fmt.Sprintf("%s (ID: %d)", buyer.Name, buyer.BuyerID))
		default:
			formatted = append(formatted, buyer.Name)
		}
	}
	return strings.Join(formatted, ", ")
}

func (s *ClusterService) notificationAccount(record domain.OrderRecord) (string, string) {
	user := record.AccountName
	uid := strings.TrimPrefix(record.AccountID, "bili-")
	if s.repository != nil && record.AccountID != "" {
		if account, err := s.repository.Account(context.Background(), record.AccountID); err == nil {
			if account.Name != "" {
				user = account.Name
			}
			if cookieUID := account.Credentials.Cookies["DedeUserID"]; cookieUID != "" {
				uid = cookieUID
			} else {
				for _, cookie := range account.Credentials.CookieJar {
					if cookie.Name == "DedeUserID" && cookie.Value != "" {
						uid = cookie.Value
						break
					}
				}
			}
		}
	}
	if user == "" {
		user = record.AccountID
	}
	if _, err := strconv.ParseInt(uid, 10, 64); err != nil {
		uid = strings.TrimPrefix(record.AccountID, "bili-")
	}
	return user, uid
}
