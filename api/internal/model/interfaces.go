package model

import (
	"context"
	k8srepo "serverless-api/api/internal/repository/k8s"
)

type BotRepository interface {
	Create(ctx context.Context, bot *Bot) (string, error)
	GetByID(ctx context.Context, id string) (*Bot, error)
	UpdateConfig(ctx context.Context, id string, config BotConfig) error
	UpdateStatus(ctx context.Context, id string, status BotStatus) error
	Delete(ctx context.Context, id string) error
}

type ShardingRepository interface {
	SetCount(ctx context.Context, botID string, count uint32) error
}

type K8SOperator interface {
	DeployBot(ctx context.Context, bot k8srepo.Bot) error
}
