// Package httpapi は各モジュールの公開 Service を REST で公開する HTTP 層。
// ハンドラは Service の公開 API のみを呼び、domain 集約・内部 map には触れない。
// 複数モジュールの読み取りを合成して DTO を組み立てるのは読み取り層の責務として許容する。
package httpapi

import (
	"github.com/kato0373i/subscope/backend/internal/billing"
	"github.com/kato0373i/subscope/backend/internal/contract"
	"github.com/kato0373i/subscope/backend/internal/metrics"
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
