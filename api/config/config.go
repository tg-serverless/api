package config

import (
	"os"
)

type Config struct {
	// --- Server ---
	HTTPPort string // Порт API (по дефолту 8000)
	LogLevel string // debug/info

	// --- Kubernetes (Для управления секретами с Git URL и токенами) ---
	// Если пусто - используется In-Cluster Config (когда API запущен в поде)
	KubeConfigPath string
	// Неймспейс, куда деплоим ботов и секреты (обычно "bots")
	BotsNamespace string

	// --- System Redis (Для Gateway Sharding) ---
	// Сюда пишем маппинг bot_id -> shard_count
	SystemRedisAddr string
	SystemRedisPass string

	// --- Provisioning Targets (Куда создаем базы для юзеров) ---
	// Эти учетки должны иметь права CREATE DATABASE / CREATE USER
	// Postgres Cluster
	UserPgHost  string
	UserPgPort  string
	UserPgAdmin string
	UserPgPass  string

	// DatabaseURL - полный DSN для подключения к Postgres
	DatabaseURL string
}

func Load() *Config {
	return &Config{
		// Server
		HTTPPort: getEnv("HTTP_PORT", "8000"),
		LogLevel: getEnv("LOG_LEVEL", "info"),

		// K8s
		KubeConfigPath: getEnv("KUBECONFIG", ""), // Оставь пустым в проде
		BotsNamespace:  getEnv("BOTS_NAMESPACE", "bots"),

		// System Redis (Gateway Config)
		SystemRedisAddr: getEnv("SYSTEM_REDIS_ADDR", "redis-system.infra:6379"),
		SystemRedisPass: getEnv("SYSTEM_REDIS_PASS", ""),

		// User Resources (Admin Creds)
		DatabaseURL: getEnv("DATABASE_URL", ""),
		UserPgHost:  getEnv("USER_PG_HOST", "postgres.infra"),
		UserPgPort:  getEnv("USER_PG_PORT", "5432"),
		UserPgAdmin: getEnv("USER_PG_ADMIN_USER", "postgres"),
		UserPgPass:  getEnv("USER_PG_ADMIN_PASS", ""),
	}
}

// Хелпер для чтения ENV
func getEnv(key, defaultVal string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultVal
}
