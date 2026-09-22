package config

import (
	"strings"
	"testing"
	"time"
)

const strongSecret = "0123456789abcdef0123456789abcdef" // ровно 32 байта

func env(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

func TestLoad_DevBypassForbiddenInProduction(t *testing.T) {
	_, err := load(env(map[string]string{
		"APP_ENV": "production", "DEV_UNSIGNED_INITDATA": "true", "JWT_SECRET": strongSecret,
	}))
	if err == nil || !strings.Contains(err.Error(), "DEV_UNSIGNED_INITDATA") {
		t.Fatalf("обход подписи в production должен останавливать старт, получили %v", err)
	}
}

// Забытый APP_ENV — это production: иначе функция без переменной окружения
// молча включила бы обход.
func TestLoad_EmptyAppEnvMeansProduction(t *testing.T) {
	cfg, err := load(env(map[string]string{"JWT_SECRET": strongSecret}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppEnv != "production" || !cfg.IsProduction() {
		t.Fatalf("AppEnv = %q, ожидали production", cfg.AppEnv)
	}
	if _, err := load(env(map[string]string{"JWT_SECRET": strongSecret, "DEV_UNSIGNED_INITDATA": "true"})); err == nil {
		t.Fatal("обход без APP_ENV должен останавливать старт")
	}
}

func TestLoad_UnknownAppEnvRejected(t *testing.T) {
	if _, err := load(env(map[string]string{"APP_ENV": "prod", "JWT_SECRET": strongSecret})); err == nil {
		t.Fatal("опечатка в APP_ENV не должна проходить молча")
	}
}

func TestLoad_ProductionRequiresStrongJWTSecret(t *testing.T) {
	for name, secret := range map[string]string{"пустой": "", "31 байт": strongSecret[:31]} {
		t.Run(name, func(t *testing.T) {
			_, err := load(env(map[string]string{"APP_ENV": "production", "JWT_SECRET": secret}))
			if err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
				t.Fatalf("ожидали ошибку про JWT_SECRET, получили %v", err)
			}
		})
	}
	if _, err := load(env(map[string]string{"APP_ENV": "production", "JWT_SECRET": strongSecret})); err != nil {
		t.Fatalf("32 байта достаточно: %v", err)
	}
}

func TestLoad_DevelopmentAllowsBypassAndShortSecret(t *testing.T) {
	cfg, err := load(env(map[string]string{
		"APP_ENV": "development", "DEV_UNSIGNED_INITDATA": "true", "JWT_SECRET": "local-dev-not-a-secret",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.DevUnsignedInitData || cfg.IsProduction() {
		t.Fatalf("cfg = %+v", cfg)
	}
}

// Даже локально пустым ключом подписывать нельзя: такой JWT подделывается.
func TestLoad_EmptyJWTSecretRejectedEverywhere(t *testing.T) {
	if _, err := load(env(map[string]string{"APP_ENV": "development"})); err == nil {
		t.Fatal("пустой JWT_SECRET должен останавливать старт и в development")
	}
}

func TestLoad_JWTTTL(t *testing.T) {
	cfg, err := load(env(map[string]string{"APP_ENV": "development", "JWT_SECRET": "x"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JWTTTL != time.Hour {
		t.Errorf("по умолчанию JWT живёт час, получили %v", cfg.JWTTTL)
	}
	cfg, err = load(env(map[string]string{"APP_ENV": "development", "JWT_SECRET": "x", "JWT_TTL": "15m"}))
	if err != nil || cfg.JWTTTL != 15*time.Minute {
		t.Errorf("JWT_TTL=15m: %v, %v", cfg.JWTTTL, err)
	}
	for _, bad := range []string{"час", "0s", "-5m"} {
		if _, err := load(env(map[string]string{"APP_ENV": "development", "JWT_SECRET": "x", "JWT_TTL": bad})); err == nil {
			t.Errorf("JWT_TTL=%q должен отклоняться", bad)
		}
	}
}

func TestLoad_StrictBool(t *testing.T) {
	if _, err := load(env(map[string]string{"APP_ENV": "development", "JWT_SECRET": "x", "DEV_UNSIGNED_INITDATA": "да"})); err == nil {
		t.Fatal("непонятное значение флага не должно трактоваться молча")
	}
}

func TestLoad_ReadsBotToken(t *testing.T) {
	cfg, err := load(env(map[string]string{"APP_ENV": "development", "JWT_SECRET": "x", "MAX_BOT_TOKEN": "test-bot-token"}))
	if err != nil || cfg.MaxBotToken != "test-bot-token" {
		t.Fatalf("MaxBotToken = %q, err = %v", cfg.MaxBotToken, err)
	}
}

// Секреты не должны утекать в лог через %v конфига.
func TestConfig_StringHidesSecrets(t *testing.T) {
	cfg := Config{AppEnv: "development", JWTSecret: strongSecret, MaxBotToken: "test-bot-token", JWTTTL: time.Hour}
	s := cfg.String()
	if strings.Contains(s, strongSecret) || strings.Contains(s, "test-bot-token") {
		t.Fatalf("секрет попал в строку конфига: %s", s)
	}
}

func TestLoad_BotNameDefaultsToTeamBot(t *testing.T) {
	cfg, _ := load(env(map[string]string{"APP_ENV": "development", "JWT_SECRET": "x"}))
	if cfg.MaxBotName != DefaultBotName {
		t.Fatalf("MaxBotName по умолчанию = %q", cfg.MaxBotName)
	}
	cfg, _ = load(env(map[string]string{"APP_ENV": "development", "JWT_SECRET": "x", "MAX_BOT_NAME": "other_bot"}))
	if cfg.MaxBotName != "other_bot" {
		t.Fatalf("MaxBotName из окружения = %q", cfg.MaxBotName)
	}
}

func TestReminderHour(t *testing.T) {
	cfg, _ := load(env(map[string]string{"APP_ENV": "development", "JWT_SECRET": "x"}))
	if cfg.ReminderHour != 10 {
		t.Fatalf("по умолчанию 10:00, получили %d", cfg.ReminderHour)
	}
	if h, err := reminderHour(env(map[string]string{"REMINDER_HOUR": "9"})); err != nil || h != 9 {
		t.Fatalf("из окружения: %d %v", h, err)
	}
	for _, bad := range []string{"24", "-1", "десять"} {
		if _, err := load(env(map[string]string{"APP_ENV": "development", "JWT_SECRET": "x", "REMINDER_HOUR": bad})); err == nil {
			t.Fatalf("REMINDER_HOUR=%s должен отклоняться", bad)
		}
	}
}

func TestBotIdentity(t *testing.T) {
	b, err := botIdentity(env(nil))
	if err != nil || b.Name != DefaultBotName || b.ID != DefaultBotID {
		t.Fatalf("по умолчанию — бот команды: %+v %v", b, err)
	}
	b, err = botIdentity(env(map[string]string{"MAX_BOT_NAME": "other_bot", "MAX_BOT_ID": "7"}))
	if err != nil || b.Name != "other_bot" || b.ID != 7 {
		t.Fatalf("из окружения: %+v %v", b, err)
	}
	for _, bad := range []string{"0", "-3", "бот"} {
		if _, err := load(env(map[string]string{"APP_ENV": "development", "JWT_SECRET": "x", "MAX_BOT_ID": bad})); err == nil {
			t.Fatalf("MAX_BOT_ID=%s должен отклоняться", bad)
		}
	}
}
