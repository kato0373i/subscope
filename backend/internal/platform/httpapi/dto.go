// Package httpapi は各モジュールの公開 Service を REST で公開する HTTP 層。
// ハンドラは Service の公開 API のみを呼び、domain 集約・内部 map には触れない。
// 複数モジュールの読み取りを合成して DTO を組み立てるのは読み取り層の責務として許容する。
package httpapi

import (
	"github.com/kato0373i/subscope/backend/internal/billing"
	"github.com/kato0373i/subscope/backend/internal/contract"
	"github.com/kato0373i/subscope/backend/internal/dunning"
	"github.com/kato0373i/subscope/backend/internal/metrics"
	"github.com/kato0373i/subscope/backend/internal/settlement"
	"github.com/kato0373i/subscope/backend/internal/shared"
)

// moneyDTO は frontend types.ts の Money（{amount, currency}）に対応。
type moneyDTO struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

func toMoney(m shared.Money) moneyDTO {
	return moneyDTO{Amount: m.Amount, Currency: m.Currency}
}

// contractDTO は frontend types.ts の Contract に対応。
type contractDTO struct {
	ID               string   `json:"id"`
	MemberName       string   `json:"memberName"`
	BillingAccountID string   `json:"billingAccountId"`
	MonthlyFee       moneyDTO `json:"monthlyFee"`
	Status           string   `json:"status"`
}

// collectionStateDTO は frontend types.ts の CollectionState に対応。
type collectionStateDTO struct {
	InvoiceID  string   `json:"invoiceId"`
	ContractID string   `json:"contractId"`
	Amount     moneyDTO `json:"amount"`
	Status     string   `json:"status"`
}

// invoiceDTO は請求書一覧の 1 行（読み取り API 用）。
type invoiceDTO struct {
	ID               string   `json:"id"`
	ContractID       string   `json:"contractId"`
	BillingAccountID string   `json:"billingAccountId"`
	Amount           moneyDTO `json:"amount"`
	Status           string   `json:"status"`
}

// customerDetailDTO は顧客個票（顧客360）の合成レスポンス。
// contract / billing / collection の読み取りを契約単位に束ねる。
type customerDetailDTO struct {
	Contract contractDTO            `json:"contract"`
	Invoices []invoiceCollectionRow `json:"invoices"`
	Summary  customerSummaryDTO     `json:"summary"`
}

// invoiceCollectionRow は請求書 1 行に回収ステータスを合成したもの。
// invoiceStatus は billing 由来の生ステータス、collectionStatus は
// billing×collection を合成した画面用ステータス（既存 collectionStatusFor）。
type invoiceCollectionRow struct {
	InvoiceID        string   `json:"invoiceId"`
	Amount           moneyDTO `json:"amount"`
	InvoiceStatus    string   `json:"invoiceStatus"`
	CollectionStatus string   `json:"collectionStatus"`
}

// customerSummaryDTO は個票上部に出す集計。outstanding=未入金合計、paid=入金済合計。
type customerSummaryDTO struct {
	InvoiceCount int      `json:"invoiceCount"`
	Paid         moneyDTO `json:"paid"`         // collectionStatus == "paid" の合計
	Outstanding  moneyDTO `json:"outstanding"`  // paid 以外の合計（債権残）
	InCollection int      `json:"inCollection"` // collectionStatus == "in_collection" の件数
}

// dunningCampaignDTO は frontend types.ts の DunningCampaign に対応。
type dunningCampaignDTO struct {
	CampaignID     string `json:"campaignId"`
	InvoiceID      string `json:"invoiceId"`
	Account        string `json:"account"`
	Status         string `json:"status"` // active / resolved / completed
	StepsTriggered int    `json:"stepsTriggered"`
	StepsTotal     int    `json:"stepsTotal"`
	NextChannel    string `json:"nextChannel"` // 完了なら ""
}

func toDunningCampaignDTO(v dunning.CampaignView) dunningCampaignDTO {
	return dunningCampaignDTO{
		CampaignID:     string(v.CampaignID),
		InvoiceID:      string(v.InvoiceID),
		Account:        string(v.Account),
		Status:         v.Status,
		StepsTriggered: v.StepsTriggered,
		StepsTotal:     v.StepsTotal,
		NextChannel:    v.NextChannel,
	}
}

// settlementDTO は frontend types.ts の Settlement に対応（消込実績の 1 行）。
type settlementDTO struct {
	SettlementID string   `json:"settlementId"`
	InvoiceID    string   `json:"invoiceId"`
	Amount       moneyDTO `json:"amount"`
	Reconciled   moneyDTO `json:"reconciled"`
	FullyApplied bool     `json:"fullyApplied"`
}

func toSettlementDTO(v settlement.SettlementView) settlementDTO {
	return settlementDTO{
		SettlementID: string(v.SettlementID),
		InvoiceID:    string(v.InvoiceID),
		Amount:       toMoney(v.Amount),
		Reconciled:   toMoney(v.Reconciled),
		FullyApplied: v.FullyApplied,
	}
}

// outstandingDTO は frontend types.ts の OutstandingInvoice に対応（未消込の請求）。
type outstandingDTO struct {
	InvoiceID   string   `json:"invoiceId"`
	Account     string   `json:"account"`
	PayerName   string   `json:"payerName"`
	Outstanding moneyDTO `json:"outstanding"`
}

func toOutstandingDTO(v settlement.OutstandingView) outstandingDTO {
	return outstandingDTO{
		InvoiceID:   string(v.InvoiceID),
		Account:     string(v.Account),
		PayerName:   v.PayerName,
		Outstanding: toMoney(v.Outstanding),
	}
}

// importDepositsRequest は POST /api/bank-deposits のリクエストボディ。
type importDepositsRequest struct {
	Deposits []depositInputDTO `json:"deposits"`
}

// depositInputDTO は銀行入金取込の 1 レコード。
type depositInputDTO struct {
	Reference string   `json:"reference"`
	Account   string   `json:"account"`
	PayerName string   `json:"payerName"`
	Amount    moneyDTO `json:"amount"`
}

// manualReconcileRequest は POST /api/settlements/manual のリクエストボディ。
type manualReconcileRequest struct {
	InvoiceID string   `json:"invoiceId"`
	Amount    moneyDTO `json:"amount"`
}

// metricsDTO は metrics.Snapshot の外向き表現。
type metricsDTO struct {
	ActiveContracts  int      `json:"activeContracts"`
	ChurnedContracts int      `json:"churnedContracts"`
	InvoicesIssued   int      `json:"invoicesIssued"`
	InvoicesPaid     int      `json:"invoicesPaid"`
	BilledTotal      moneyDTO `json:"billedTotal"`
	RecoveredTotal   moneyDTO `json:"recoveredTotal"`
	WrittenOffTotal  moneyDTO `json:"writtenOffTotal"`
	RefundTotal      moneyDTO `json:"refundTotal"`
}

// registerContractRequest は POST /api/contracts のリクエストボディ。
type registerContractRequest struct {
	ID               string   `json:"id"`
	MemberID         string   `json:"memberId"`
	BillingAccountID string   `json:"billingAccountId"`
	MonthlyFee       moneyDTO `json:"monthlyFee"`
}

// runBillingRequest は POST /api/billing-runs のリクエストボディ（全フィールド任意）。
type runBillingRequest struct {
	AsOf   string `json:"asOf"`   // YYYY-MM-DD。省略時は現在時刻。
	DryRun bool   `json:"dryRun"` // true なら抽出のみ（起票しない）。
}

// billingRunItemDTO は Billing Run が起票（予定）した 1 件。決済手段は持たない（債権≠決済手段）。
type billingRunItemDTO struct {
	ContractID       string   `json:"contractId"`
	BillingAccountID string   `json:"billingAccountId"`
	Amount           moneyDTO `json:"amount"`
	Period           string   `json:"period"`
}

// billingRunResultDTO は POST /api/billing-runs のレスポンス。
type billingRunResultDTO struct {
	RunID   string              `json:"runId"`
	AsOf    string              `json:"asOf"`
	DryRun  bool                `json:"dryRun"`
	Items   []billingRunItemDTO `json:"items"`
	Skipped int                 `json:"skipped"`
}

func toBillingRunDTO(r contract.BillingRunResult) billingRunResultDTO {
	items := make([]billingRunItemDTO, 0, len(r.Items))
	for _, it := range r.Items {
		items = append(items, billingRunItemDTO{
			ContractID:       string(it.ContractID),
			BillingAccountID: string(it.BillingAccountID),
			Amount:           toMoney(it.Amount),
			Period:           it.Period,
		})
	}
	return billingRunResultDTO{
		RunID:   string(r.RunID),
		AsOf:    r.AsOf.Format("2006-01-02"),
		DryRun:  r.DryRun,
		Items:   items,
		Skipped: r.Skipped,
	}
}

func toContractDTO(v contract.ContractView, memberName string) contractDTO {
	return contractDTO{
		ID:               string(v.ID),
		MemberName:       memberName,
		BillingAccountID: string(v.BillingAccountID),
		MonthlyFee:       toMoney(v.MonthlyFee),
		Status:           v.Status,
	}
}

func toInvoiceDTO(v billing.InvoiceView) invoiceDTO {
	return invoiceDTO{
		ID:               string(v.ID),
		ContractID:       string(v.ContractID),
		BillingAccountID: string(v.BillingAccountID),
		Amount:           toMoney(v.Amount),
		Status:           v.Status,
	}
}

func toMetricsDTO(s metrics.Snapshot) metricsDTO {
	return metricsDTO{
		ActiveContracts:  s.ActiveContracts,
		ChurnedContracts: s.ChurnedContracts,
		InvoicesIssued:   s.InvoicesIssued,
		InvoicesPaid:     s.InvoicesPaid,
		BilledTotal:      toMoney(s.BilledTotal),
		RecoveredTotal:   toMoney(s.RecoveredTotal),
		WrittenOffTotal:  toMoney(s.WrittenOffTotal),
		RefundTotal:      toMoney(s.RefundTotal),
	}
}

// 合成ロジックで参照するステータス文字列。billing/collection の domain 定数値と一致させる
// （internal/domain は import 不可のため API 契約として文字列で固定する）。
const (
	statusInvoicePaid = "paid"
	statusCaseWritten = "written_off"
	statusCaseInProg  = "in_progress"
	statusCaseEscal   = "escalated"
)

// collectionStatusFor は invoice と case を合成して frontend の CollectionStatus を決める。
// 優先順: paid > written_off > in_collection > issued。
// partially_paid は現状ドメインに部分入金概念が無いため生成しない（#11/#44 で対応予定）。
func collectionStatusFor(invoiceStatus, caseStatus string, hasCase bool) string {
	if invoiceStatus == statusInvoicePaid {
		return "paid"
	}
	if hasCase {
		switch caseStatus {
		case statusCaseWritten:
			return "written_off"
		case statusCaseInProg, statusCaseEscal:
			return "in_collection"
		}
	}
	return "issued"
}
