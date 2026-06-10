// Package contract は契約モジュールの公開 API。
// 他モジュールはこのパッケージ（とイベント）だけに依存し、internal/domain には触れない。
package contract

import (
	"context"
	"errors"
	"log"
	"sort"
	"time"

	"github.com/kato0373i/subscope/backend/internal/contract/internal/domain"
	"github.com/kato0373i/subscope/backend/internal/shared"
	"github.com/kato0373i/subscope/backend/internal/shared/events"
)

// Adjustment は日割り調整明細の再エクスポート。
type Adjustment = domain.Adjustment

// BillingRunItem は Billing Run が起票（予定）した 1 契約ぶんの請求。決済手段は含まない。
type BillingRunItem = domain.BillingItem

// BillingRunResult は Billing Run の実行結果（プレビュー含む）。
type BillingRunResult struct {
	RunID   shared.BillingRunID // asOf から決まる安定 ID（同一日付なら同一）
	AsOf    time.Time
	DryRun  bool             // true なら抽出のみ（イベント発行・日付前進なし）
	Items   []BillingRunItem // DryRun: 起票予定 / 実行時: 実際に起票した請求
	Skipped int              // 冪等性により既起票としてスキップした件数
}

// ContractView は契約の読み取り専用ビュー（外向き読みモデル）。
// HTTP/読み取り層へ domain 集約を露出させないための DTO。
type ContractView struct {
	ID               shared.ContractID
	MemberID         shared.MemberID
	BillingAccountID shared.BillingAccountID
	MonthlyFee       shared.Money
	Status           string
}

// ErrNotFound は契約が見つからない場合に返る。
var ErrNotFound = errors.New("契約が見つかりません")

type Service struct {
	bus       shared.EventBus
	contracts map[shared.ContractID]*domain.Contract
	// billed は (契約ID, 請求期間) ごとの起票済みマーク。Billing Run / 手動トリガの
	// 二重起票を防ぐ冪等性ガード。期間は契約の請求スケジュールから決まる（CurrentBillingPeriod）。
	billed map[string]bool
}

func NewService(bus shared.EventBus) *Service {
	return &Service{
		bus:       bus,
		contracts: make(map[shared.ContractID]*domain.Contract),
		billed:    make(map[string]bool),
	}
}

// List は登録済み契約を ID 昇順で返す（読み取り API 用）。
func (s *Service) List() []ContractView {
	views := make([]ContractView, 0, len(s.contracts))
	for _, c := range s.contracts {
		views = append(views, ContractView{
			ID:               c.ID,
			MemberID:         c.MemberID,
			BillingAccountID: c.BillingAccountID,
			MonthlyFee:       c.MonthlyFee,
			Status:           string(c.Status),
		})
	}
	sort.Slice(views, func(i, j int) bool { return views[i].ID < views[j].ID })
	return views
}

// RegisterContract は契約を登録する（デモ用の簡易入口）。
func (s *Service) RegisterContract(id shared.ContractID, member shared.MemberID, account shared.BillingAccountID, fee shared.Money) {
	s.contracts[id] = domain.New(id, member, account, fee)
}

// RegisterTrial はトライアル付きで契約を登録する（trialing で開始）。
func (s *Service) RegisterTrial(id shared.ContractID, member shared.MemberID, account shared.BillingAccountID, fee shared.Money, trialDays int) {
	s.contracts[id] = domain.NewFull(id, "", member, account, "", fee, domain.CycleMonthly, domain.BillingAnchor(1), domain.TrialPeriod{Days: trialDays})
}

// MarkPastDue は未収発生により active → past_due へ遷移させる（請求オペレーションの入口）。
func (s *Service) MarkPastDue(id shared.ContractID) error {
	c, ok := s.contracts[id]
	if !ok {
		return ErrNotFound
	}
	return c.SetPastDue()
}

// TriggerBilling は単一契約の請求を手動で起こす入口（オペレータの「いま請求する」）。
// Billing Run と同じ冪等な起票経路を通り、対象期間は契約のスケジュールから導出する。
func (s *Service) TriggerBilling(ctx context.Context, id shared.ContractID) error {
	c, ok := s.contracts[id]
	if !ok {
		return nil
	}
	log.Printf("[contract]   請求サイクル到来 contract=%s account=%s", c.ID, c.BillingAccountID)
	_, err := s.issueBilling(ctx, c)
	return err
}

// RunBilling は asOf 時点で請求サイクルが到来した全契約を抽出し、契約ごとに BillingDue を
// 発行して次回請求日を進める。Billing Run（定期請求の自動起票）の入口で、手動トリガに加え
// 将来の cron / 外部スケジューラからも同じメソッドを呼ぶ。
//
// 決済手段には一切触れない：抽出するのは「誰に・いくら・どの期間」だけで、どの手段で回収するかは
// 後段の collection が請求先ごとに解決する。これにより自動課金の決済手段は付け替え可能なまま保たれる。
//
// dryRun=true のときは抽出結果のプレビューのみ返し、イベント発行も日付前進も行わない。
func (s *Service) RunBilling(ctx context.Context, asOf time.Time, dryRun bool) (BillingRunResult, error) {
	run := domain.NewBillingRun(asOf)
	plan := run.Plan(s.sortedContracts())

	result := BillingRunResult{RunID: run.ID(), AsOf: asOf, DryRun: dryRun}
	for _, item := range plan {
		if s.billed[billedKey(item.ContractID, item.Period)] {
			result.Skipped++
			continue
		}
		if dryRun {
			result.Items = append(result.Items, item)
			continue
		}
		issued, err := s.issueBilling(ctx, s.contracts[item.ContractID])
		if err != nil {
			return result, err
		}
		if issued {
			result.Items = append(result.Items, item)
		} else {
			result.Skipped++
		}
	}
	if dryRun {
		log.Printf("[contract] Billing Run（ドライラン）run=%s asOf=%s 対象=%d 件 スキップ=%d 件", result.RunID, asOf.Format("2006-01-02"), len(result.Items), result.Skipped)
	} else {
		log.Printf("[contract] Billing Run run=%s asOf=%s 起票=%d 件 スキップ=%d 件", result.RunID, asOf.Format("2006-01-02"), len(result.Items), result.Skipped)
	}
	return result, nil
}

// issueBilling は 1 契約に対して冪等に BillingDue を発行し、次回請求日を進める。
// 当該期間が起票済みなら何もしない（issued=false）。BillingDue は決済手段を載せない。
func (s *Service) issueBilling(ctx context.Context, c *domain.Contract) (issued bool, err error) {
	period := c.CurrentBillingPeriod()
	key := billedKey(c.ID, period)
	if s.billed[key] {
		return false, nil
	}
	if err := s.bus.Publish(ctx, events.BillingDue{
		ContractID:       c.ID,
		BillingAccountID: c.BillingAccountID,
		Amount:           c.MonthlyFee,
		Period:           period,
	}); err != nil {
		return false, err
	}
	s.billed[key] = true
	c.AdvanceBillingDate()
	return true, nil
}

// sortedContracts は契約を ID 昇順で返す。Billing Run の抽出順を決定的にする。
func (s *Service) sortedContracts() []*domain.Contract {
	out := make([]*domain.Contract, 0, len(s.contracts))
	for _, c := range s.contracts {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// billedKey は (契約ID, 請求期間) の冪等性キーを組み立てる。
func billedKey(id shared.ContractID, period string) string {
	return string(id) + "|" + period
}

// Activate はトライアル終了で契約を有効化し ContractActivated を発行する。
func (s *Service) Activate(ctx context.Context, id shared.ContractID) error {
	c, ok := s.contracts[id]
	if !ok {
		return ErrNotFound
	}
	if err := c.Activate(); err != nil {
		return err
	}
	log.Printf("[contract] 契約を有効化 contract=%s", c.ID)
	return s.bus.Publish(ctx, events.ContractActivated{ContractID: c.ID})
}

// Suspend は契約を利用停止にし ContractSuspended を発行する。
func (s *Service) Suspend(ctx context.Context, id shared.ContractID) error {
	c, ok := s.contracts[id]
	if !ok {
		return ErrNotFound
	}
	if err := c.Suspend(); err != nil {
		return err
	}
	log.Printf("[contract] 契約を利用停止 contract=%s", c.ID)
	return s.bus.Publish(ctx, events.ContractSuspended{ContractID: c.ID})
}

// Cancel は契約を解約し ContractCancelled を発行する。
func (s *Service) Cancel(ctx context.Context, id shared.ContractID) error {
	c, ok := s.contracts[id]
	if !ok {
		return ErrNotFound
	}
	if err := c.Cancel(); err != nil {
		return err
	}
	log.Printf("[contract] 契約を解約 contract=%s", c.ID)
	return s.bus.Publish(ctx, events.ContractCancelled{ContractID: c.ID})
}

// ChangePlan はプランを変更する。変更日の日割り調整を計算し、契約を更新して
// PlanChanged（差額付き）を発行し、計算した調整明細を返す。
func (s *Service) ChangePlan(ctx context.Context, id shared.ContractID, newPlanID shared.PlanID, newFee shared.Money, changeDate time.Time) (Adjustment, error) {
	c, ok := s.contracts[id]
	if !ok {
		return Adjustment{}, ErrNotFound
	}
	adj, err := domain.ProrationPolicy{}.Calculate(c, newFee, changeDate)
	if err != nil {
		return Adjustment{}, err
	}
	oldPlan := c.PlanID
	if err := c.ChangePlan(newPlanID, newFee); err != nil {
		return Adjustment{}, err
	}
	log.Printf("[contract] プラン変更 contract=%s %s→%s 日割り差額=%s", c.ID, oldPlan, newPlanID, adj.Net)
	if err := s.bus.Publish(ctx, events.PlanChanged{
		ContractID:    c.ID,
		OldPlanID:     oldPlan,
		NewPlanID:     newPlanID,
		NetAdjustment: adj.Net,
	}); err != nil {
		return Adjustment{}, err
	}
	return adj, nil
}
