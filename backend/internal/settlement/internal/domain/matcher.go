package domain

import (
	"strings"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

// Candidate は消込先の候補となる未消込の請求（settlement のローカル投影）。
type Candidate struct {
	Invoice     shared.InvoiceID
	Account     shared.BillingAccountID
	PayerName   string
	Outstanding shared.Money
}

// NormalizeName は振込人名義の表記揺れを吸収する。
// 空白（半角・全角）の除去、トリム、大文字化を行い、自動照合の精度を上げる。
func NormalizeName(s string) string {
	s = strings.ReplaceAll(s, "　", "") // 全角空白
	s = strings.ReplaceAll(s, " ", "")
	return strings.ToUpper(strings.TrimSpace(s))
}

// Match は入金に対する自動消込の充当案を返す。
// 照合キーは「請求先 ID（あれば優先）／なければ正規化した名義」＋金額。
// 単一一致（ある請求の残額＝入金額）と、団体一括（グループ残額の合計＝入金額での按分）に対応する。
// いずれにも一致しなければ matched=false を返し、呼び出し側は手動消込へ回す。
func Match(deposit *BankDeposit, candidates []Candidate) (allocations []Allocation, matched bool) {
	group := filterGroup(deposit, candidates)
	if len(group) == 0 {
		return nil, false
	}

	// 1) 単一一致：候補のいずれかの残額が入金額と一致 → その請求へ全額充当。
	for _, c := range group {
		if c.Outstanding.Currency == deposit.Amount.Currency && c.Outstanding.Amount == deposit.Amount.Amount {
			return []Allocation{{Invoice: c.Invoice, Amount: c.Outstanding}}, true
		}
	}

	// 2) 団体一括：グループの残額合計が入金額と一致 → 各請求へ残額全額を按分。
	var sum int64
	for _, c := range group {
		sum += c.Outstanding.Amount
	}
	if sum == deposit.Amount.Amount {
		allocs := make([]Allocation, 0, len(group))
		for _, c := range group {
			allocs = append(allocs, Allocation{Invoice: c.Invoice, Amount: c.Outstanding})
		}
		return allocs, true
	}

	return nil, false
}

// filterGroup は入金の照合キーに一致する候補を返す。
// 請求先 ID を持つ入金は ID で、持たない入金は正規化名義で束ねる。
func filterGroup(deposit *BankDeposit, candidates []Candidate) []Candidate {
	var group []Candidate
	if deposit.Account != "" {
		for _, c := range candidates {
			if c.Account == deposit.Account {
				group = append(group, c)
			}
		}
		return group
	}
	key := NormalizeName(deposit.PayerName)
	if key == "" {
		return nil
	}
	for _, c := range candidates {
		if NormalizeName(c.PayerName) == key {
			group = append(group, c)
		}
	}
	return group
}
