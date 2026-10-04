package domain

import (
	"errors"
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"416-555-0123", "+14165550123", false},
		{"(416) 555 0123", "+14165550123", false},
		{"+44 20 7946 0958", "+442079460958", false},
		{"12345", "", true},
		{"416-555-012a", "", true},
	}
	for _, tt := range tests {
		got, err := NormalizePhone(tt.in)
		if tt.wantErr {
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("NormalizePhone(%q) err = %v, want ErrInvalidInput", tt.in, err)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("NormalizePhone(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
}

func TestSearchQueryNormalize(t *testing.T) {
	q := SearchQuery{Limit: 1000, Offset: -5}.Normalize()
	if q.Limit != MaxSearchLimit || q.Offset != 0 {
		t.Fatalf("got %+v", q)
	}
	if (SearchQuery{}).Normalize().Limit != DefaultSearchLimit {
		t.Fatal("default limit not applied")
	}
	if !(SearchQuery{}).IsEmpty() || (SearchQuery{Name: "a"}).IsEmpty() {
		t.Fatal("IsEmpty wrong")
	}
}

func TestCredentialMethodValid(t *testing.T) {
	if !MethodPassword.Valid() || CredentialMethod("magic").Valid() {
		t.Fatal("Valid() wrong")
	}
}
