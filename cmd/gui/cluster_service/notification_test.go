package cluster_service

import (
	"context"
	"testing"

	"bilibili-ticket-golang/cluster/domain"
	"bilibili-ticket-golang/cmd/gui/i18n"
)

func TestTicketSuccessNotificationMatchesOriginalTemplate(t *testing.T) {
	i18n.SetLocale("zh-CN")
	service := testClusterService(t)
	if err := service.repository.PutAccount(context.Background(), domain.Account{
		ID: "bili-42", Name: "哔哩用户", Enabled: true,
		Credentials: domain.Credentials{Cookies: map[string]string{"DedeUserID": "42"}},
	}, nil); err != nil {
		t.Fatal(err)
	}
	intent := domain.LogicalOrderIntent{Buyers: []domain.Buyer{
		{Name: "张三", Tel: "13800000000", Type: 1},
		{Name: "李四", BuyerID: 9, Type: 2},
	}}
	result := domain.ExecutionResult{SubOrders: []domain.SubOrderResult{
		{BuyerIndex: 0, State: domain.SubOrderFailed},
		{BuyerIndex: 1, State: domain.SubOrderSucceeded},
	}}
	records := []domain.OrderRecord{{
		Status: domain.SubOrderSucceeded, ProjectName: "项目", ScreenName: "场次",
		SKUName: "票种", AccountID: "bili-42", AccountName: "哔哩用户", PaymentURL: "https://pay.example/order",
	}}
	got := service.ticketSuccessNotification(intent, result, records)
	want := "抢票成功！\n项目：项目\n场次：场次\n票种：票种\n购票人：李四 (ID: 9)\n购票用户：哔哩用户(42)\n支付链接：https://pay.example/order"
	if got != want {
		t.Fatalf("notification mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestNotificationBuyerFormattingMatchesOriginal(t *testing.T) {
	intent := domain.LogicalOrderIntent{Buyers: []domain.Buyer{
		{Name: "普通用户", Tel: "13800000000", Type: 1},
		{Name: "实名用户", BuyerID: 7, Type: 2},
	}}
	got := notificationBuyers(intent, domain.ExecutionResult{})
	want := "普通用户 (13800000000), 实名用户 (ID: 7)"
	if got != want {
		t.Fatalf("buyers=%q, want %q", got, want)
	}
}
