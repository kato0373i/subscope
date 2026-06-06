package domain

import (
	"testing"

	"github.com/kato0373i/subscope/backend/internal/shared"
)

func TestMember_New_ValidEmail(t *testing.T) {
	cases := []string{
		"user@example.com",
		"a@b.jp",
	}
	for _, email := range cases {
		if _, err := New("MEM-001", shared.OrgID("ORG-001"), "テスト", email); err != nil {
			t.Fatalf("有効なメールアドレス %q でエラー: %v", email, err)
		}
	}
}

func TestMember_New_SetsStatusActive(t *testing.T) {
	m, err := New("MEM-001", shared.OrgID("ORG-001"), "テスト", "user@example.com")
	if err != nil {
		t.Fatalf("有効な入力でエラー: %v", err)
	}
	if m.Status != StatusActive {
		t.Errorf("期待: StatusActive, 実際: %v", m.Status)
	}
}

func TestMember_New_InvalidEmail(t *testing.T) {
	cases := []string{
		"",            // 空
		"@",           // @ のみ
		"user@",       // ドメイン欠落
		"@domain.com", // ローカル部欠落
		"nodomain",    // @ なし
		"a@@b.com",    // 複数 @
	}
	for _, email := range cases {
		if _, err := New("MEM-001", shared.OrgID("ORG-001"), "テスト", email); err == nil {
			t.Fatalf("無効なメールアドレス %q でエラーが発生しなかった", email)
		}
	}
}
