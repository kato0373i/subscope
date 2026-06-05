package domain

import (
	"testing"
	"time"
)

func TestNewRegistration_Validation(t *testing.T) {
	tests := []struct {
		name    string
		number  string
		wantErr bool
	}{
		{"正常: T+13桁", "T1234567890123", false},
		{"異常: T無し", "1234567890123", true},
		{"異常: 桁数不足", "T123", true},
		{"異常: 空", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRegistration(tt.number, time.Now())
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRegistration(%q) err = %v, wantErr = %v", tt.number, err, tt.wantErr)
			}
		})
	}
}

func TestRegistration_IsValidAt(t *testing.T) {
	from := time.Date(2023, 10, 1, 0, 0, 0, 0, time.UTC)
	reg, err := NewRegistration("T1234567890123", from)
	if err != nil {
		t.Fatal(err)
	}

	if reg.IsValidAt(from.AddDate(0, -1, 0)) {
		t.Error("登録開始前は無効であるべき")
	}
	if !reg.IsValidAt(from.AddDate(1, 0, 0)) {
		t.Error("無期限登録は開始後ずっと有効であるべき")
	}
}
