package auth

import (
	"errors"
	"testing"
	"time"
)

const testMember = "6f1c2a44-0000-4000-8000-000000000002"

func TestLink_RoundTrip(t *testing.T) {
	token, exp, err := IssueLink("calendar", testMember, testSecret, 10*time.Minute, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if !exp.Equal(testNow.Add(10 * time.Minute)) {
		t.Fatalf("срок ссылки: %v", exp)
	}
	got, err := ParseLink("calendar", token, testSecret, testNow.Add(9*time.Minute))
	if err != nil || got != testMember {
		t.Fatalf("ParseLink: %q, %v", got, err)
	}
}

func TestLink_Expired(t *testing.T) {
	token, _, err := IssueLink("calendar", testMember, testSecret, 10*time.Minute, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseLink("calendar", token, testSecret, testNow.Add(11*time.Minute)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("просроченная ссылка принята: %v", err)
	}
}

// Ссылка на календарь не должна открывать ничего, кроме календаря, а токен
// сессии — работать ссылкой: иначе адрес из истории браузера давал бы весь API.
func TestLink_PurposeIsolated(t *testing.T) {
	link, _, err := IssueLink("calendar", testMember, testSecret, 10*time.Minute, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseLink("other", link, testSecret, testNow); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("ссылка с чужим назначением принята: %v", err)
	}
	if _, err := Parse(link, testSecret, testNow); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("ссылка принята как токен сессии: %v", err)
	}
	session, _, err := Issue(testClaims(), testSecret, time.Hour, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseLink("calendar", session, testSecret, testNow); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("токен сессии принят как ссылка: %v", err)
	}
}

func TestLink_Tampered(t *testing.T) {
	token, _, err := IssueLink("calendar", testMember, testSecret, 10*time.Minute, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseLink("calendar", token, []byte("другой-секрет-0123456789abcdef-long"), testNow); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("чужая подпись принята: %v", err)
	}
	if _, err := ParseLink("calendar", token+"x", testSecret, testNow); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("испорченная подпись принята: %v", err)
	}
	if _, _, err := IssueLink("calendar", testMember, nil, time.Minute, testNow); err == nil {
		t.Fatal("пустой секрет — ошибка")
	}
	if _, _, err := IssueLink("calendar", "", testSecret, time.Minute, testNow); err == nil {
		t.Fatal("ссылка без участника — ошибка")
	}
}
