package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi/maxtest"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testBotToken = "test-bot-token"

var testNow = time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

type env struct {
	t    *testing.T
	pool *pgxpool.Pool
	st   *store.Store
	srv  *Server
	h    http.Handler
	now  time.Time
	// fake — чат бота: уведомления семье, которые шлёт API.
	fake *maxtest.Fake
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool := dbtest.Open(t)
	dbtest.Fictional(t, pool)
	e := &env{t: t, pool: pool, st: store.New(pool), now: testNow, fake: &maxtest.Fake{}}
	e.srv = newServer(Deps{
		Store: e.st,
		Max:   e.fake,
		Config: config.Config{AppEnv: "test", DevUnsignedInitData: true,
			JWTSecret: "test-jwt-secret-0123456789abcdef!", JWTTTL: time.Hour, MaxBotToken: testBotToken,
			MaxBotName: "test_bot", MaxBotID: 42, ReminderHour: 10},
		Now: func() time.Time { return e.now },
	})
	// Служебная ручка только для тестов: кто я по мнению authed.
	e.srv.mux.HandleFunc("GET /test/me", e.srv.authed(func(w http.ResponseWriter, r *http.Request) error {
		writeJSON(w, http.StatusOK, map[string]string{"member_id": me(r).MemberID})
		return nil
	}))
	e.h = e.srv.handler([]string{"https://traektoria.website.yandexcloud.net"})
	return e
}

// signedInitData подписывает строку так, как это делает MAX (dev.max.ru).
func signedInitData(userID int64, name string, authDate time.Time) string {
	user, _ := json.Marshal(map[string]any{"id": userID, "first_name": name})
	pairs := map[string]string{"auth_date": fmt.Sprint(authDate.Unix()), "user": string(user), "query_id": "q-1"}
	keys := make([]string, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, len(keys))
	for i, k := range keys {
		lines[i] = k + "=" + pairs[k]
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(testBotToken))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(lines, "\n")))
	v := url.Values{}
	for k, val := range pairs {
		v.Set(k, val)
	}
	v.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return v.Encode()
}

// stubInitData — строка заглушки браузера (apps/web/src/bridge/stub.ts).
func stubInitData(userID int64, name string) string {
	user, _ := json.Marshal(map[string]any{"id": userID, "first_name": name})
	return url.Values{"user": {string(user)}, "auth_date": {fmt.Sprint(testNow.Unix())},
		"hash": {"dev-stub-unsigned"}}.Encode()
}

type resp struct {
	code int
	body map[string]any
	raw  []byte
	hdr  http.Header
}

func (e *env) do(method, path, token string, body any) resp {
	e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			e.t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	r := resp{code: rec.Code, raw: rec.Body.Bytes(), hdr: rec.Header()}
	_ = json.Unmarshal(r.raw, &r.body)
	return r
}

func (r resp) errCode() string {
	e, _ := r.body["error"].(map[string]any)
	c, _ := e["code"].(string)
	return c
}

func ptr[T any](v T) *T { return &v }

// family — траектория демо-персонажей: Артём (ученик, создатель) и, по
// желанию, Ольга (родитель).
type family struct {
	trajectoryID string
	kid, parent  store.Member
}

func (e *env) kidCreator() family {
	e.t.Helper()
	ctx := context.Background()
	uid, err := e.st.UpsertUser(ctx, 900000001, "Артём")
	if err != nil {
		e.t.Fatal(err)
	}
	m, err := e.st.CreateTrajectory(ctx, store.NewTrajectory{
		CreatorUserID: uid, Role: "kid", StudentName: "Артём", Grade: 9, RegionCode: "16",
		TZ: "Europe/Moscow", DirectionID: ptr("napr-09-03-04"), GoalStatus: "known",
		SubjectCodes: []string{"inf", "math"}, UniversityIDs: []string{"innopolis", "hse", "kfu"},
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return family{trajectoryID: m.TrajectoryID, kid: m}
}

func (e *env) withParent(f family) family {
	e.t.Helper()
	ctx := context.Background()
	uid, _ := e.st.UpsertUser(ctx, 900000002, "Ольга")
	m, err := e.st.AddMember(ctx, f.trajectoryID, uid, "parent")
	if err != nil {
		e.t.Fatal(err)
	}
	f.parent = m
	return f
}

// login проходит POST /session за пользователя и возвращает токен.
func (e *env) login(maxUserID int64, name string) string {
	e.t.Helper()
	r := e.do("POST", "/api/v1/session", "", map[string]any{"init_data": signedInitData(maxUserID, name, e.now)})
	if r.code != http.StatusOK {
		e.t.Fatalf("вход %d: %d %s", maxUserID, r.code, r.raw)
	}
	return r.body["token"].(string)
}

// track кладёт пункт в трекер напрямую (ручки трекера проверяются отдельно).
func (e *env) track(f family, profileID string, registered bool) string {
	e.t.Helper()
	var id string
	var regBy any
	if registered {
		regBy = f.kid.MemberID
		if f.parent.MemberID != "" {
			regBy = f.parent.MemberID
		}
	}
	err := e.pool.QueryRow(context.Background(), `
		INSERT INTO tracker_items (trajectory_id, olympiad_profile_id, added_by_member_id, registered_at, registered_by_member_id)
		VALUES ($1, $2, $3, CASE WHEN $4 THEN now() END, $5) RETURNING id::text`,
		f.trajectoryID, profileID, f.kid.MemberID, registered, regBy).Scan(&id)
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}
