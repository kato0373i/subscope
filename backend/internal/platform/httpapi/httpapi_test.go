package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kato0373i/subscope/backend/internal/billing"
	"github.com/kato0373i/subscope/backend/internal/collection"
	"github.com/kato0373i/subscope/backend/internal/contract"
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
}

func (s *stubContracts) List() []contract.ContractView { return s.views }
func (s *stubContracts) RegisterContract(id shared.ContractID, _ shared.MemberID, _ shared.BillingAccountID, _ shared.Money) {
	s.registered = append(s.registered, id)
}
func (s *stubContracts) TriggerBilling(_ context.Context, id shared.ContractID) error {
	s.triggered = append(s.triggered, id)
	return s.triggerErr
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
