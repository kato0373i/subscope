package domain

import (
	"testing"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

func TestNormalizeName_AbsorbsVariations(t *testing.T) {
	cases := map[string]string{
		" ヤマダ タロウ ": "ヤマダタロウ",
		"ヤマダ　タロウ":   "ヤマダタロウ", // 全角空白
		"abc inc":   "ABCINC",
	}
	for in, want := range cases {
		if got := NormalizeName(in); got != want {
			t.Errorf("NormalizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

// 請求先 ID ＋ 金額の単一一致で自動消込する。
func TestMatch_SingleByAccount(t *testing.T) {
	dep := NewBankDeposit("DEP-1", "REF-1", "BA-1", "ヤマダ", shared.JPY(3000))
	cands := []Candidate{
		{Invoice: "INV-1", Account: "BA-1", Outstanding: shared.JPY(3000)},
		{Invoice: "INV-2", Account: "BA-2", Outstanding: shared.JPY(3000)},
	}
	allocs, matched := Match(dep, cands)
	if !matched {
		t.Fatal("matched = false, want true")
	}
	if len(allocs) != 1 || allocs[0].Invoice != "INV-1" {
		t.Fatalf("allocations = %+v, want 単一 INV-1", allocs)
	}
}

// 団体一括：同一請求先の複数請求の残額合計＝入金額 → 各請求へ按分。
func TestMatch_ApportionByAccountSum(t *testing.T) {
	dep := NewBankDeposit("DEP-1", "REF-1", "BA-1", "協会", shared.JPY(5000))
	cands := []Candidate{
		{Invoice: "INV-1", Account: "BA-1", Outstanding: shared.JPY(2000)},
		{Invoice: "INV-2", Account: "BA-1", Outstanding: shared.JPY(3000)},
	}
	allocs, matched := Match(dep, cands)
	if !matched {
		t.Fatal("matched = false, want true")
	}
	if len(allocs) != 2 {
		t.Fatalf("allocations = %d, want 2（按分）", len(allocs))
	}
}

// 名義（正規化）＋金額で照合する（請求先 ID が無い入金）。
func TestMatch_ByNormalizedName(t *testing.T) {
	dep := NewBankDeposit("DEP-1", "REF-1", "", "ヤマダ　タロウ", shared.JPY(3000))
	cands := []Candidate{
		{Invoice: "INV-1", PayerName: "ヤマダ タロウ", Outstanding: shared.JPY(3000)},
	}
	allocs, matched := Match(dep, cands)
	if !matched || len(allocs) != 1 {
		t.Fatalf("名義照合に失敗 allocs=%+v matched=%v", allocs, matched)
	}
}

// 金額が一致しない（合計にも満たない）入金は自動消込せず matched=false。
func TestMatch_UnmatchedOnAmountMismatch(t *testing.T) {
	dep := NewBankDeposit("DEP-1", "REF-1", "BA-1", "ヤマダ", shared.JPY(2500))
	cands := []Candidate{
		{Invoice: "INV-1", Account: "BA-1", Outstanding: shared.JPY(3000)},
	}
	if _, matched := Match(dep, cands); matched {
		t.Error("金額不一致は matched=false であるべき")
	}
}

// 同額候補が複数ある曖昧ケースは自動消込せず matched=false（手動へ）。
func TestMatch_UnmatchedOnAmbiguousSameAmount(t *testing.T) {
	dep := NewBankDeposit("DEP-1", "REF-1", "BA-1", "協会", shared.JPY(3000))
	cands := []Candidate{
		{Invoice: "INV-1", Account: "BA-1", Outstanding: shared.JPY(3000)},
		{Invoice: "INV-2", Account: "BA-1", Outstanding: shared.JPY(3000)},
	}
	if _, matched := Match(dep, cands); matched {
		t.Error("同額候補が複数なら matched=false であるべき")
	}
}

// 団体一括の候補に通貨混在がある場合は matched=false（誤合算を防ぐ）。
func TestMatch_UnmatchedOnMixedCurrencies(t *testing.T) {
	dep := NewBankDeposit("DEP-1", "REF-1", "BA-1", "協会", shared.JPY(5000))
	cands := []Candidate{
		{Invoice: "INV-1", Account: "BA-1", Outstanding: shared.JPY(2000)},
		{Invoice: "INV-2", Account: "BA-1", Outstanding: shared.Money{Amount: 3000, Currency: "USD"}},
	}
	if _, matched := Match(dep, cands); matched {
		t.Error("通貨混在のグループは matched=false であるべき")
	}
}

// 該当する請求先が無い入金は matched=false。
func TestMatch_UnmatchedOnNoCandidate(t *testing.T) {
	dep := NewBankDeposit("DEP-1", "REF-1", "BA-X", "謎", shared.JPY(3000))
	cands := []Candidate{
		{Invoice: "INV-1", Account: "BA-1", Outstanding: shared.JPY(3000)},
	}
	if _, matched := Match(dep, cands); matched {
		t.Error("該当請求先なしは matched=false であるべき")
	}
}
