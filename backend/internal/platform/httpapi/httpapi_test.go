package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kato0373i/subscope/backend/internal/billing"
	"github.com/kato0373i/subscope/backend/internal/collection"
	"github.com/kato0373i/subscope/backend/internal/contract"
	"github.com/kato0373i/subscope/backend/internal/dunning"
	"github.com/kato0373i/subscope/backend/internal/metrics"
	"github.com/kato0373i/subscope/backend/internal/platform/httpapi"
	"github.com/kato0373i/subscope/backend/internal/shared"
)

// --- スタブ（各 Reader を最小実装） ---

type stubContracts struct {
	views      []contract.ContractView
	registered []shared.ContractID
	triggered  []shared.ContractID
	triggerErr error
	runResult  contract.BillingRunResult
	runAsOf    time.Time
	runDryRun  bool
}

func (s *stubContracts) List() []contract.ContractView { return s.views }
func (s *stubContracts) RegisterContract(id shared.ContractID, _ shared.MemberID, _ shared.BillingAccountID, _ shared.Money) {
	s.registered = append(s.registered, id)
}
func (s *stubContracts) TriggerBilling(_ context.Context, id shared.ContractID) error {
	s.triggered = append(s.triggered, id)
	return s.triggerErr
}
func (s *stubContracts) RunBilling(_ context.Context, asOf time.Time, dryRun bool) (contract.BillingRunResult, error) {
	s.runAsOf = asOf
	s.runDryRun = dryRun
	return s.runResult, nil
}

type stubInvoices struct{ views []billing.InvoiceView }

func (s *stubInvoices) ListInvoices() []billing.InvoiceView { return s.views }

type stubCases struct{ views []collection.CaseView }

func (s *stubCases) ListCases() []collection.CaseView { return s.views }

type stubMembers struct{ names map[shared.MemberID]string }

func (s *stubMembers) Name(id shared.MemberID) (string, bool) {
	n, ok := s.names[id]
	return n, ok
}

type stubMetrics struct{ snap metrics.Snapshot }

func (s *stubMetrics) Snapshot() metrics.Snapshot { return s.snap }

type stubDunning struct{ views []dunning.CampaignView }

func (s *stubDunning) ListCampaigns() []dunning.CampaignView { return s.views }

func newTestServer(d httpapi.Deps) *httptest.Server {
	return httptest.NewServer(httpapi.New(d))
}

func TestHealthz(t *testing.T) {
	srv := newTestServer(httpapi.Deps{})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestListContracts_ResolvesMemberName(t *testing.T) {
	deps := httpapi.Deps{
		Contracts: &stubContracts{views: []contract.ContractView{
			{ID: "CT-0001", MemberID: "MEM-0001", BillingAccountID: "BA-0001", MonthlyFee: shared.JPY(3000), Status: "active"},
		}},
		Members: &stubMembers{names: map[shared.MemberID]string{"MEM-0001": "山田 太郎"}},
	}
	srv := newTestServer(deps)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/contracts")
	if err != nil {
		t.Fatalf("GET /api/contracts: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got []struct {
		ID         string `json:"id"`
		MemberName string `json:"memberName"`
		MonthlyFee struct {
			Amount   int64  `json:"amount"`
			Currency string `json:"currency"`
		} `json:"monthlyFee"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].MemberName != "山田 太郎" {
		t.Errorf("memberName = %q, want 山田 太郎", got[0].MemberName)
	}
	if got[0].MonthlyFee.Amount != 3000 || got[0].MonthlyFee.Currency != "JPY" {
		t.Errorf("monthlyFee = %+v, want {3000 JPY}", got[0].MonthlyFee)
	}
}

func TestCollectionStates_StatusComposition(t *testing.T) {
	deps := httpapi.Deps{
		Invoices: &stubInvoices{views: []billing.InvoiceView{
			{ID: "INV-0001", ContractID: "CT-0001", Amount: shared.JPY(3000), Status: "paid"},
			{ID: "INV-0002", ContractID: "CT-0002", Amount: shared.JPY(5000), Status: "issued"},
			{ID: "INV-0003", ContractID: "CT-0003", Amount: shared.JPY(3000), Status: "issued"},
			{ID: "INV-0004", ContractID: "CT-0004", Amount: shared.JPY(8000), Status: "issued"},
		}},
		Cases: &stubCases{views: []collection.CaseView{
			{InvoiceID: "INV-0002", Status: "in_progress"},
			{InvoiceID: "INV-0003", Status: "written_off"},
		}},
	}
	srv := newTestServer(deps)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/collection-states")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	var got []struct {
		InvoiceID string `json:"invoiceId"`
		Status    string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := map[string]string{
		"INV-0001": "paid",          // invoice paid 優先
		"INV-0002": "in_collection", // case in_progress
		"INV-0003": "written_off",   // case written_off
		"INV-0004": "issued",        // case 無し
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for _, g := range got {
		if want[g.InvoiceID] != g.Status {
			t.Errorf("%s status = %q, want %q", g.InvoiceID, g.Status, want[g.InvoiceID])
		}
	}
}

func TestGetContract_Composition(t *testing.T) {
	deps := httpapi.Deps{
		Contracts: &stubContracts{views: []contract.ContractView{
			{ID: "CT-0001", MemberID: "MEM-0001", BillingAccountID: "BA-0001", MonthlyFee: shared.JPY(3000), Status: "active"},
			{ID: "CT-0002", MemberID: "MEM-0002", BillingAccountID: "BA-0002", MonthlyFee: shared.JPY(5000), Status: "active"},
		}},
		Invoices: &stubInvoices{views: []billing.InvoiceView{
			{ID: "INV-0001", ContractID: "CT-0001", Amount: shared.JPY(3000), Status: "paid"},
			{ID: "INV-0002", ContractID: "CT-0001", Amount: shared.JPY(3000), Status: "issued"},
			{ID: "INV-0003", ContractID: "CT-0001", Amount: shared.JPY(3000), Status: "issued"},
			{ID: "INV-0099", ContractID: "CT-0002", Amount: shared.JPY(5000), Status: "issued"}, // 別契約は混ざらない
		}},
		Cases: &stubCases{views: []collection.CaseView{
			{InvoiceID: "INV-0002", Status: "in_progress"}, // → in_collection
		}},
		Members: &stubMembers{names: map[shared.MemberID]string{"MEM-0001": "山田 太郎"}},
	}
	srv := newTestServer(deps)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/contracts/CT-0001")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got struct {
		Contract struct {
			ID         string `json:"id"`
			MemberName string `json:"memberName"`
		} `json:"contract"`
		Invoices []struct {
			InvoiceID        string `json:"invoiceId"`
			CollectionStatus string `json:"collectionStatus"`
		} `json:"invoices"`
		Summary struct {
			InvoiceCount int `json:"invoiceCount"`
			Paid         struct {
				Amount   int64  `json:"amount"`
				Currency string `json:"currency"`
			} `json:"paid"`
			Outstanding struct {
				Amount   int64  `json:"amount"`
				Currency string `json:"currency"`
			} `json:"outstanding"`
			InCollection int `json:"inCollection"`
		} `json:"summary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got.Contract.MemberName != "山田 太郎" {
		t.Errorf("memberName = %q, want 山田 太郎", got.Contract.MemberName)
	}
	if len(got.Invoices) != 3 {
		t.Fatalf("invoices len = %d, want 3 (別契約 INV-0099 は除外)", len(got.Invoices))
	}
	colByID := map[string]string{}
	for _, r := range got.Invoices {
		colByID[r.InvoiceID] = r.CollectionStatus
	}
	if colByID["INV-0001"] != "paid" || colByID["INV-0002"] != "in_collection" || colByID["INV-0003"] != "issued" {
		t.Errorf("collectionStatus 合成が想定外: %v", colByID)
	}
	if got.Summary.InvoiceCount != 3 {
		t.Errorf("invoiceCount = %d, want 3", got.Summary.InvoiceCount)
	}
	if got.Summary.Paid.Amount != 3000 || got.Summary.Paid.Currency != "JPY" {
		t.Errorf("paid = %d %s, want 3000 JPY", got.Summary.Paid.Amount, got.Summary.Paid.Currency)
	}
	if got.Summary.Outstanding.Amount != 6000 || got.Summary.Outstanding.Currency != "JPY" {
		t.Errorf("outstanding = %d %s, want 6000 JPY", got.Summary.Outstanding.Amount, got.Summary.Outstanding.Currency)
	}
	if got.Summary.InCollection != 1 {
		t.Errorf("inCollection = %d, want 1", got.Summary.InCollection)
	}
}

func TestGetContract_NotFound(t *testing.T) {
	srv := newTestServer(httpapi.Deps{
		Contracts: &stubContracts{},
		Members:   &stubMembers{names: map[shared.MemberID]string{}},
	})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/contracts/UNKNOWN")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestListDunningCampaigns(t *testing.T) {
	deps := httpapi.Deps{
		Dunning: &stubDunning{views: []dunning.CampaignView{
			// 意図的に逆順で渡し、ハンドラが CampaignID 昇順へ整列することを確認する。
			{CampaignID: "DUN-0002", InvoiceID: "INV-0002", Account: "BA-0002", Status: "resolved", StepsTriggered: 1, StepsTotal: 3, NextChannel: "sms"},
			{CampaignID: "DUN-0001", InvoiceID: "INV-0001", Account: "BA-0001", Status: "active", StepsTriggered: 2, StepsTotal: 3, NextChannel: "letter"},
		}},
	}
	srv := newTestServer(deps)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/dunning-campaigns")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got []struct {
		CampaignID     string `json:"campaignId"`
		Status         string `json:"status"`
		StepsTriggered int    `json:"stepsTriggered"`
		StepsTotal     int    `json:"stepsTotal"`
		NextChannel    string `json:"nextChannel"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].CampaignID != "DUN-0001" || got[1].CampaignID != "DUN-0002" {
		t.Errorf("CampaignID 昇順整列されていない: %s, %s", got[0].CampaignID, got[1].CampaignID)
	}
	if got[0].Status != "active" || got[0].StepsTriggered != 2 || got[0].StepsTotal != 3 || got[0].NextChannel != "letter" {
		t.Errorf("DUN-0001 の写像が想定外: %+v", got[0])
	}
}

func TestRegisterContract(t *testing.T) {
	sc := &stubContracts{}
	srv := newTestServer(httpapi.Deps{Contracts: sc})
	defer srv.Close()

	body := `{"id":"CT-9999","memberId":"MEM-1","billingAccountId":"BA-1","monthlyFee":{"amount":3000,"currency":"JPY"}}`
	resp, err := http.Post(srv.URL+"/api/contracts", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	if len(sc.registered) != 1 || sc.registered[0] != "CT-9999" {
		t.Errorf("registered = %v, want [CT-9999]", sc.registered)
	}
}

func TestRegisterContract_MissingField(t *testing.T) {
	srv := newTestServer(httpapi.Deps{Contracts: &stubContracts{}})
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/contracts", "application/json", strings.NewReader(`{"id":"CT-1"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestTriggerBilling_NotFound(t *testing.T) {
	srv := newTestServer(httpapi.Deps{Contracts: &stubContracts{triggerErr: contract.ErrNotFound}})
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/contracts/CT-x/billing", "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestRunBilling_DryRunPreview(t *testing.T) {
	sc := &stubContracts{runResult: contract.BillingRunResult{
		RunID:  "BR-20260610",
		AsOf:   time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		DryRun: true,
		Items: []contract.BillingRunItem{
			{ContractID: "CT-0001", BillingAccountID: "BA-0001", Amount: shared.JPY(3000), Period: "2026-06"},
		},
		Skipped: 1,
	}}
	srv := newTestServer(httpapi.Deps{Contracts: sc})
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/billing-runs", "application/json", strings.NewReader(`{"asOf":"2026-06-10","dryRun":true}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	// 渡された引数が正しくサービスへ伝わること。
	if !sc.runDryRun {
		t.Error("dryRun が false で渡された, want true")
	}
	if sc.runAsOf.Format("2006-01-02") != "2026-06-10" {
		t.Errorf("asOf = %s, want 2026-06-10", sc.runAsOf.Format("2006-01-02"))
	}
	// asOf はドメインの時刻系（ローカル）に合わせて解釈する。UTC 固定だと非 UTC 環境で
	// 請求サイクル境界の due 判定がズレるため、ローカルの午前0時に正規化されること。
	if sc.runAsOf.Location() != time.Local || sc.runAsOf.Hour() != 0 {
		t.Errorf("asOf = %v (loc=%v), want ローカル午前0時", sc.runAsOf, sc.runAsOf.Location())
	}

	var got struct {
		RunID   string `json:"runId"`
		DryRun  bool   `json:"dryRun"`
		Skipped int    `json:"skipped"`
		Items   []struct {
			ContractID string `json:"contractId"`
			Period     string `json:"period"`
			Amount     struct {
				Amount int64 `json:"amount"`
			} `json:"amount"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.RunID != "BR-20260610" || !got.DryRun || got.Skipped != 1 {
		t.Errorf("result = %+v", got)
	}
	if len(got.Items) != 1 || got.Items[0].ContractID != "CT-0001" || got.Items[0].Period != "2026-06" {
		t.Fatalf("items = %+v", got.Items)
	}
	if got.Items[0].Amount.Amount != 3000 {
		t.Errorf("amount = %d, want 3000", got.Items[0].Amount.Amount)
	}
}

func TestRunBilling_DefaultsAsOfToNow(t *testing.T) {
	sc := &stubContracts{}
	srv := newTestServer(httpapi.Deps{Contracts: sc})
	defer srv.Close()

	// ボディなしでも 200 で、asOf が現在時刻にフォールバックする。
	resp, err := http.Post(srv.URL+"/api/billing-runs", "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if sc.runAsOf.IsZero() {
		t.Error("asOf が現在時刻にフォールバックしていない")
	}
	if sc.runDryRun {
		t.Error("dryRun は既定で false のはず")
	}
}

func TestMetrics_ReturnsSnapshot(t *testing.T) {
	deps := httpapi.Deps{Metrics: &stubMetrics{snap: metrics.Snapshot{
		ActiveContracts: 3,
		InvoicesIssued:  2,
		BilledTotal:     shared.JPY(6000),
	}}}
	srv := newTestServer(deps)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/metrics")
	if err != nil {
		t.Fatalf("GET /api/metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got struct {
		ActiveContracts int `json:"activeContracts"`
		InvoicesIssued  int `json:"invoicesIssued"`
		BilledTotal     struct {
			Amount   int64  `json:"amount"`
			Currency string `json:"currency"`
		} `json:"billedTotal"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ActiveContracts != 3 || got.InvoicesIssued != 2 {
		t.Errorf("snapshot = %+v, want activeContracts=3 invoicesIssued=2", got)
	}
	if got.BilledTotal.Amount != 6000 || got.BilledTotal.Currency != "JPY" {
		t.Errorf("billedTotal = %+v, want {6000 JPY}", got.BilledTotal)
	}
}

func TestCORSPreflight(t *testing.T) {
	srv := newTestServer(httpapi.Deps{})
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/api/contracts", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Allow-Origin = %q, want *", got)
	}
}
