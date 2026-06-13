package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/kato0373i/subscope/backend/internal/billing"
	"github.com/kato0373i/subscope/backend/internal/collection"
	"github.com/kato0373i/subscope/backend/internal/contract"
	"github.com/kato0373i/subscope/backend/internal/metrics"
	"github.com/kato0373i/subscope/backend/internal/shared"
)

// 以下の小さなインターフェースで各モジュールの公開 Service の必要メソッドだけを受ける。
// これにより httpapi → 各モジュールの一方向依存に保ち、テストではモックを差し込める。

// ContractReader は契約の読み取りとコマンドを提供する。
type ContractReader interface {
	List() []contract.ContractView
	RegisterContract(id shared.ContractID, member shared.MemberID, account shared.BillingAccountID, fee shared.Money)
	TriggerBilling(ctx context.Context, id shared.ContractID) error
	RunBilling(ctx context.Context, asOf time.Time, dryRun bool) (contract.BillingRunResult, error)
}

// InvoiceReader は請求書一覧を提供する。
type InvoiceReader interface {
	ListInvoices() []billing.InvoiceView
}

// CaseReader は回収案件一覧を提供する。
type CaseReader interface {
	ListCases() []collection.CaseView
}

// MemberNamer は会員 ID から表示名を解決する。
type MemberNamer interface {
	Name(id shared.MemberID) (string, bool)
}

// MetricsReader は指標スナップショットを提供する。
type MetricsReader interface {
	Snapshot() metrics.Snapshot
}

// Deps は HTTP 層が依存する各モジュールの公開 API。
type Deps struct {
	Contracts ContractReader
	Invoices  InvoiceReader
	Cases     CaseReader
	Members   MemberNamer
	Metrics   MetricsReader
}

// server はルーティングとハンドラを束ねる。
type server struct {
	deps Deps
}

// New は Deps を結線した HTTP ハンドラを返す。CORS とパニックリカバリを適用済み。
func New(deps Deps) http.Handler {
	s := &server{deps: deps}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /api/contracts", s.handleListContracts)
	mux.HandleFunc("GET /api/contracts/{id}", s.handleGetContract)
	mux.HandleFunc("POST /api/contracts", s.handleRegisterContract)
	mux.HandleFunc("POST /api/contracts/{id}/billing", s.handleTriggerBilling)
	mux.HandleFunc("POST /api/billing-runs", s.handleRunBilling)
	mux.HandleFunc("GET /api/invoices", s.handleListInvoices)
	mux.HandleFunc("GET /api/collection-states", s.handleListCollectionStates)
	mux.HandleFunc("GET /api/metrics", s.handleMetrics)

	return withCORS(withRecover(mux))
}

func (s *server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleListContracts(w http.ResponseWriter, _ *http.Request) {
	views := s.deps.Contracts.List()
	out := make([]contractDTO, 0, len(views))
	for _, v := range views {
		name, _ := s.deps.Members.Name(v.MemberID)
		out = append(out, toContractDTO(v, name))
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGetContract は 1 契約の個票（顧客360）を返す。contract / billing / collection の
// 読み取りを契約単位に合成する（ドメイン変更なし・公開 Service のみ参照）。
func (s *server) handleGetContract(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "契約 ID が必要です")
		return
	}
	cid := shared.ContractID(id)

	// 契約ヘッダ。List を id でフィルタ（インメモリ前提・契約数は小さい）。
	var view contract.ContractView
	found := false
	for _, v := range s.deps.Contracts.List() {
		if v.ID == cid {
			view = v
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "not_found", "契約が見つかりません")
		return
	}
	name, _ := s.deps.Members.Name(view.MemberID)

	// 回収案件を InvoiceID で引けるよう投影（collection-states と同型）。
	caseByInvoice := make(map[shared.InvoiceID]string)
	for _, c := range s.deps.Cases.ListCases() {
		caseByInvoice[c.InvoiceID] = c.Status
	}

	// この契約の請求書を束ね、各行の回収ステータスとサマリを合成する。
	// 金額集計は通貨未設定のゼロから始め、初項で通貨を採用する（addMoneyDTO）。
	rows := make([]invoiceCollectionRow, 0)
	summary := customerSummaryDTO{}
	for _, inv := range s.deps.Invoices.ListInvoices() {
		if inv.ContractID != cid {
			continue
		}
		caseStatus, hasCase := caseByInvoice[inv.ID]
		colStatus := collectionStatusFor(inv.Status, caseStatus, hasCase)
		rows = append(rows, invoiceCollectionRow{
			InvoiceID:        string(inv.ID),
			Amount:           toMoney(inv.Amount),
			InvoiceStatus:    inv.Status,
			CollectionStatus: colStatus,
		})
		summary.InvoiceCount++
		switch colStatus {
		case "paid":
			summary.Paid = addMoneyDTO(summary.Paid, inv.Amount)
		case "in_collection":
			summary.InCollection++
			summary.Outstanding = addMoneyDTO(summary.Outstanding, inv.Amount)
		default:
			summary.Outstanding = addMoneyDTO(summary.Outstanding, inv.Amount)
		}
	}

	// 請求が無く通貨が未確定なら契約の月額通貨でゼロ表示にする。
	if summary.Paid.Currency == "" {
		summary.Paid.Currency = view.MonthlyFee.Currency
	}
	if summary.Outstanding.Currency == "" {
		summary.Outstanding.Currency = view.MonthlyFee.Currency
	}

	writeJSON(w, http.StatusOK, customerDetailDTO{
		Contract: toContractDTO(view, name),
		Invoices: rows,
		Summary:  summary,
	})
}

// addMoneyDTO は集計用に moneyDTO へ shared.Money を加算する。
// acc の通貨が未設定なら m の通貨を採用し（ゼロ初期値の初項を許容）、
// 通貨不一致は加算せず据え置く（単一契約内は同一通貨の前提）。
func addMoneyDTO(acc moneyDTO, m shared.Money) moneyDTO {
	if acc.Currency == "" {
		return moneyDTO{Amount: m.Amount, Currency: m.Currency}
	}
	if acc.Currency != m.Currency {
		return acc
	}
	return moneyDTO{Amount: acc.Amount + m.Amount, Currency: acc.Currency}
}

func (s *server) handleRegisterContract(w http.ResponseWriter, r *http.Request) {
	var req registerContractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "リクエストボディを解釈できません")
		return
	}
	if req.ID == "" || req.MemberID == "" || req.BillingAccountID == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "id / memberId / billingAccountId は必須です")
		return
	}
	if req.MonthlyFee.Currency == "" {
		req.MonthlyFee.Currency = "JPY"
	}
	fee := shared.Money{Amount: req.MonthlyFee.Amount, Currency: req.MonthlyFee.Currency}
	s.deps.Contracts.RegisterContract(
		shared.ContractID(req.ID),
		shared.MemberID(req.MemberID),
		shared.BillingAccountID(req.BillingAccountID),
		fee,
	)
	writeJSON(w, http.StatusCreated, map[string]string{"id": req.ID})
}

func (s *server) handleTriggerBilling(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "契約 ID が必要です")
		return
	}
	if err := s.deps.Contracts.TriggerBilling(r.Context(), shared.ContractID(id)); err != nil {
		if errors.Is(err, contract.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "契約が見つかりません")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"contractId": id})
}

// handleRunBilling は Billing Run（定期請求の自動起票）を起動する。
// ボディ（任意）: {"asOf":"2026-06-10","dryRun":true}。asOf 省略時は現在時刻。
// dryRun=true なら抽出結果のプレビューのみを返す（起票しない）。
func (s *server) handleRunBilling(w http.ResponseWriter, r *http.Request) {
	req := runBillingRequest{}
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "リクエストボディを解釈できません")
			return
		}
	}
	asOf := time.Now()
	if req.AsOf != "" {
		// asOf はドメインが NextBillingDate を算出する時刻系（ローカル）に合わせて解釈する。
		// time.Parse は TZ 無し＝UTC 解釈になり、ローカルが UTC でない環境では請求サイクル
		// 境界の due 判定が最大 TZ オフセットぶんズレうる。両層の基準をローカルに揃える。
		t, err := time.ParseInLocation("2006-01-02", req.AsOf, time.Local)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_as_of", "asOf は YYYY-MM-DD 形式で指定してください")
			return
		}
		asOf = t
	}
	result, err := s.deps.Contracts.RunBilling(r.Context(), asOf, req.DryRun)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toBillingRunDTO(result))
}

func (s *server) handleListInvoices(w http.ResponseWriter, _ *http.Request) {
	views := s.deps.Invoices.ListInvoices()
	out := make([]invoiceDTO, 0, len(views))
	for _, v := range views {
		out = append(out, toInvoiceDTO(v))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleListCollectionStates(w http.ResponseWriter, _ *http.Request) {
	invoices := s.deps.Invoices.ListInvoices()
	cases := s.deps.Cases.ListCases()

	caseByInvoice := make(map[shared.InvoiceID]string, len(cases))
	for _, c := range cases {
		caseByInvoice[c.InvoiceID] = c.Status
	}

	out := make([]collectionStateDTO, 0, len(invoices))
	for _, inv := range invoices {
		caseStatus, hasCase := caseByInvoice[inv.ID]
		out = append(out, collectionStateDTO{
			InvoiceID:  string(inv.ID),
			ContractID: string(inv.ContractID),
			Amount:     toMoney(inv.Amount),
			Status:     collectionStatusFor(inv.Status, caseStatus, hasCase),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, toMetricsDTO(s.deps.Metrics.Snapshot()))
}
