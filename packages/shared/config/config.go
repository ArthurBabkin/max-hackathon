// Package config читает настройки из окружения. В Cloud Functions переменные
// приходят из версии функции и из Lockbox, локально — из .env через compose.
package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

// Get возвращает значение переменной или запасной вариант.
func Get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// MustGet завершает процесс, если переменная не задана.
// Для функций это лучше, чем работать с пустым токеном и молча отдавать ошибки.
func MustGet(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("переменная окружения %s не задана", key)
	}
	return v
}

const (
	EnvProduction  = "production"
	EnvDevelopment = "development"
	EnvTest        = "test"

	// minProdJWTSecret — HS256 с ключом короче 256 бит подбирается перебором.
	minProdJWTSecret = 32
)

// Config — настройки входа в мини-приложение, которые опасно перепутать:
// их проверяет Load, и сервис с неверной комбинацией не стартует.
type Config struct {
	AppEnv              string        // APP_ENV: production | development | test
	DevUnsignedInitData bool          // DEV_UNSIGNED_INITDATA
	JWTSecret           string        // JWT_SECRET
	JWTTTL              time.Duration // JWT_TTL, по умолчанию 1h
	MaxBotToken         string        // MAX_BOT_TOKEN — им же проверяется подпись initData
	MaxBotName          string        // MAX_BOT_NAME — для ссылок https://max.ru/<bot>?start=...
	ReminderHour        int           // REMINDER_HOUR — час напоминаний по зоне ученика, по умолчанию 10
}

// DefaultBotName — бот команды; имя публичное, как адрес сайта.
const DefaultBotName = "t356_hakaton_max_bot"

// DefaultReminderHour — ТЗ §6.3: напоминания в 10:00 по времени ученика.
const DefaultReminderHour = 10

// ReminderHour читает REMINDER_HOUR (0–23). Нужен и API (план напоминаний
// пересчитывается после каждого изменения трекера), и воркеру, которому
// остальной конфиг входа ни к чему.
func ReminderHour() (int, error) { return reminderHour(os.Getenv) }

func reminderHour(getenv func(string) string) (int, error) {
	v := getenv("REMINDER_HOUR")
	if v == "" {
		return DefaultReminderHour, nil
	}
	h, err := strconv.Atoi(v)
	if err != nil || h < 0 || h > 23 {
		return 0, fmt.Errorf("REMINDER_HOUR=%q: ожидали час от 0 до 23", v)
	}
	return h, nil
}

// IsProduction — true и для пустого APP_ENV: Load подставляет production,
// если переменная забыта.
func (c Config) IsProduction() bool { return c.AppEnv == EnvProduction }

// String не печатает секреты: конфиг можно логировать целиком.
func (c Config) String() string {
	return fmt.Sprintf("Config{AppEnv:%s DevUnsignedInitData:%t JWTSecret:%s JWTTTL:%s MaxBotToken:%s MaxBotName:%s ReminderHour:%d}",
		c.AppEnv, c.DevUnsignedInitData, mask(c.JWTSecret), c.JWTTTL, mask(c.MaxBotToken), c.MaxBotName, c.ReminderHour)
}

func mask(s string) string {
	if s == "" {
		return "<пусто>"
	}
	return "<задан>"
}

// Load читает конфиг из окружения и отказывает в опасных комбинациях.
// Вызывающий обязан остановить процесс при ошибке — на старте, а не на
// первом запросе.
func Load() (Config, error) { return load(os.Getenv) }

func load(getenv func(string) string) (Config, error) {
	cfg := Config{
		AppEnv:      getenv("APP_ENV"),
		JWTSecret:   getenv("JWT_SECRET"),
		MaxBotToken: getenv("MAX_BOT_TOKEN"),
		MaxBotName:  getenv("MAX_BOT_NAME"),
		JWTTTL:      time.Hour,
	}
	if cfg.MaxBotName == "" {
		cfg.MaxBotName = DefaultBotName
	}
	hour, err := reminderHour(getenv)
	if err != nil {
		return Config{}, err
	}
	cfg.ReminderHour = hour
	switch cfg.AppEnv {
	case "":
		// Забытая переменная — это прод: безопасные значения по умолчанию.
		cfg.AppEnv = EnvProduction
	case EnvProduction, EnvDevelopment, EnvTest:
	default:
		return Config{}, fmt.Errorf("APP_ENV=%q: ожидали production, development или test", cfg.AppEnv)
	}

	if v := getenv("DEV_UNSIGNED_INITDATA"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("DEV_UNSIGNED_INITDATA=%q: ожидали true или false", v)
		}
		cfg.DevUnsignedInitData = b
	}
	if v := getenv("JWT_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("JWT_TTL=%q: ожидали положительную длительность, например 1h", v)
		}
		cfg.JWTTTL = d
	}

	if cfg.IsProduction() && cfg.DevUnsignedInitData {
		return Config{}, errors.New("DEV_UNSIGNED_INITDATA=true запрещён при APP_ENV=production: " +
			"обход подписи initData допустим только локально")
	}
	if cfg.JWTSecret == "" {
		return Config{}, errors.New("JWT_SECRET не задан: сессии мини-приложения нечем подписывать")
	}
	if cfg.IsProduction() && len(cfg.JWTSecret) < minProdJWTSecret {
		return Config{}, fmt.Errorf("JWT_SECRET короче %d байт: в production нужен случайный ключ "+
			"(например, openssl rand -hex 32)", minProdJWTSecret)
	}
	return cfg, nil
}
