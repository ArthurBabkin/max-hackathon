package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSession_SignedInitDataIssuesToken(t *testing.T) {
	e := newEnv(t)
	f := e.kidCreator()
	r := e.do("POST", "/api/v1/session", "", map[string]any{
		"init_data": signedInitData(900000001, "Тёма", e.now), "start_param": "inv_abc"})
	if r.code != http.StatusOK {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	s := r.body["session"].(map[string]any)
	user := s["user"].(map[string]any)
	if user["first_name"] != "Тёма" || user["max_user_id"].(float64) != 900000001 {
		t.Fatalf("user: %v (имя обновляется из initData)", user)
	}
	member := s["member"].(map[string]any)
	if member["id"] != f.kid.MemberID || member["role"] != "kid" || member["is_creator"] != true {
		t.Fatalf("member: %v", member)
	}
	tr := s["trajectory"].(map[string]any)
	if tr["region_name"] != "Республика Татарстан" || tr["direction_name"] != "Программная инженерия" ||
		tr["has_kid"] != true || tr["members_count"].(float64) != 1 {
		t.Fatalf("trajectory: %v", tr)
	}
	perms := s["permissions"].(map[string]any)
	if perms["add_to_tracker"] != true || perms["propose"] != false || perms["delete_trajectory"] != true {
		t.Fatalf("permissions: %v", perms)
	}
	if s["start_param"] != "inv_abc" {
		t.Fatalf("start_param: %v", s["start_param"])
	}
	exp, err := time.Parse(time.RFC3339, r.body["expires_at"].(string))
	if err != nil || !exp.Equal(e.now.Add(time.Hour)) {
		t.Fatalf("expires_at: %v %v", r.body["expires_at"], err)
	}
}

func TestSession_Rejections(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()

	if r := e.do("POST", "/session", "", map[string]any{"init_data": signedInitData(900000001, "Артём", e.now.Add(-2*time.Hour))}); r.code != 401 || r.errCode() != "UNAUTHORIZED" {
		t.Errorf("устаревшая подпись: %d %s", r.code, r.raw)
	}
	if r := e.do("POST", "/session", "", map[string]any{"init_data": stubInitData(12345, "Чужой")}); r.code != 401 {
		t.Errorf("неподписанная строка вне дев-диапазона: %d %s", r.code, r.raw)
	}
	if r := e.do("POST", "/session", "", map[string]any{}); r.code != 400 || r.errCode() != "BAD_REQUEST" {
		t.Errorf("без init_data: %d %s", r.code, r.raw)
	}
	r := e.do("POST", "/session", "", map[string]any{"init_data": signedInitData(900000077, "Новичок", e.now)})
	if r.code != 404 || r.errCode() != "NOT_FOUND" {
		t.Errorf("без траектории ждём 404: %d %s", r.code, r.raw)
	}
	var users int
	_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM users WHERE max_user_id = 900000077").Scan(&users)
	if users != 0 {
		t.Error("открытие мини-приложения без траектории не должно заводить пользователя")
	}
}

func TestSession_DevStubAcceptedOutsideProduction(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	r := e.do("POST", "/session", "", map[string]any{"init_data": stubInitData(900000001, "Артём")})
	if r.code != http.StatusOK {
		t.Fatalf("строка заглушки в дев-режиме: %d %s", r.code, r.raw)
	}
}

func TestAuthed_RemovedMemberLosesAccessWithLiveToken(t *testing.T) {
	e := newEnv(t)
	f := e.withParent(e.kidCreator())
	token := e.login(900000002, "Ольга")

	if r := e.do("GET", "/api/v1/test/me", "мусор", nil); r.code != 401 || r.errCode() != "UNAUTHORIZED" {
		t.Fatalf("мусорный токен: %d %s", r.code, r.raw)
	}
	if r := e.do("GET", "/api/v1/test/me", "", nil); r.code != 401 {
		t.Fatalf("без токена: %d", r.code)
	}
	if r := e.do("GET", "/api/v1/test/me", token, nil); r.code != 200 || r.body["member_id"] != f.parent.MemberID {
		t.Fatalf("живой участник: %d %s", r.code, r.raw)
	}
	_, _ = e.pool.Exec(context.Background(), "UPDATE members SET removed_at = now() WHERE id = $1", f.parent.MemberID)
	if r := e.do("GET", "/api/v1/test/me", token, nil); r.code != 401 {
		t.Fatalf("удалённый участник с живым токеном должен получить 401, получил %d", r.code)
	}
	r := e.do("POST", "/api/v1/session", "", map[string]any{"init_data": signedInitData(900000002, "Ольга", e.now)})
	if r.code != 404 {
		t.Fatalf("после удаления новый вход — 404 «пройдите онбординг», получили %d", r.code)
	}
}

func TestAuthed_ExpiredTokenIsUnauthorized(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	token := e.login(900000001, "Артём")
	e.now = e.now.Add(61 * time.Minute)
	if r := e.do("GET", "/test/me", token, nil); r.code != 401 {
		t.Fatalf("токен старше часа: %d", r.code)
	}
}

func TestRouter_PrefixUnknownRouteAndCORS(t *testing.T) {
	e := newEnv(t)
	if r := e.do("GET", "/api/v1/health", "", nil); r.code != 200 {
		t.Fatalf("health с префиксом: %d", r.code)
	}
	if r := e.do("GET", "/api/v1/nope", "", nil); r.code != 404 || r.errCode() != "NOT_FOUND" {
		t.Fatalf("неизвестный адрес — 404 в формате контракта: %d %s", r.code, r.raw)
	}

	pre := httptest.NewRequest("OPTIONS", "/api/v1/home", nil)
	pre.Header.Set("Origin", "https://traektoria.website.yandexcloud.net")
	pre.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, pre)
	if rec.Code != 204 || rec.Header().Get("Access-Control-Allow-Origin") != "https://traektoria.website.yandexcloud.net" ||
		!strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatalf("preflight: %d %v", rec.Code, rec.Header())
	}
	evil := httptest.NewRequest("OPTIONS", "/api/v1/home", nil)
	evil.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	e.h.ServeHTTP(rec, evil)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("чужой источник не должен получать CORS-разрешение")
	}
}
