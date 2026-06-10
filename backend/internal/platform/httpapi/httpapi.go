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
		t, err := time.Parse("2006-01-02", req.AsOf)
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
