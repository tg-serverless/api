package model

import (
	"errors"
	"time"
)

var ErrBotNotFound = errors.New("bot not found")

// BotStatus — перечисление возможных статусов бота
type BotStatus string

const (
	BotStatusCreating BotStatus = "creating"
	BotStatusActive   BotStatus = "active"
	BotStatusFailed   BotStatus = "failed"
)

type Bot struct {
	ID            string    `json:"id" db:"id"`
	Name          string    `json:"name" db:"name"`
	GitURL        string    `json:"git_url" db:"git_url"`
	GitEntrypoint string    `json:"git_entrypoint" db:"git_url"`
	Config        BotConfig `json:"config" db:"config"` // JSONB
	IsActive      bool      `json:"is_active" db:"is_active"`
	Status        BotStatus `json:"status" db:"status"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// BotConfig мапится в JSONB колонка
type BotConfig struct {
	MinReplicas int               `json:"min_replicas"`
	MaxReplicas int               `json:"max_replicas"`
	NumTopics   int               `json:"num_topics"`
	Env         map[string]string `json:"env"`
	Resources   ResourceConfig    `json:"resources"`
}

type ResourceConfig struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}
