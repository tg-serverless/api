package bots

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"serverless-api/api/internal/model"
)

type postgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) model.BotRepository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, bot *model.Bot) (string, error) {
	configBytes, err := json.Marshal(bot.Config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal bot config: %w", err)
	}

	now := time.Now()
	if bot.CreatedAt.IsZero() {
		bot.CreatedAt = now
	}
	bot.UpdatedAt = now

	if bot.Status == "" {
		bot.Status = model.BotStatusCreating
	}

	bot.IsActive = true

	query := `
		INSERT INTO bots (
			name, git_url, git_entrypoint, config, is_active, status, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		) RETURNING id
	`

	var id string
	row := r.db.QueryRowContext(ctx, query,
		bot.Name,
		bot.GitURL,
		bot.GitEntrypoint,
		configBytes,
		bot.IsActive,
		string(bot.Status),
		bot.CreatedAt,
		bot.UpdatedAt,
	)
	if err := row.Scan(&id); err != nil {
		return "", fmt.Errorf("failed to insert bot: %w", err)
	}

	bot.ID = id
	return id, nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id string) (*model.Bot, error) {
	query := `
		SELECT 
			id, name, git_url, git_entrypoint, config, is_active, status, created_at, updated_at
		FROM bots 
		WHERE id = $1
	`

	row := r.db.QueryRowContext(ctx, query, id)

	var bot model.Bot
	var configBytes []byte
	var statusStr string

	err := row.Scan(
		&bot.ID,
		&bot.Name,
		&bot.GitURL,
		&bot.GitEntrypoint,
		&configBytes,
		&bot.IsActive,
		&statusStr,
		&bot.CreatedAt,
		&bot.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrBotNotFound
		}
		return nil, fmt.Errorf("failed to scan bot: %w", err)
	}

	bot.Status = model.BotStatus(statusStr)

	if err := json.Unmarshal(configBytes, &bot.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bot config: %w", err)
	}

	return &bot, nil
}

func (r *postgresRepository) UpdateConfig(ctx context.Context, id string, config model.BotConfig) error {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `
		UPDATE bots 
		SET config = $1, updated_at = $2
		WHERE id = $3
	`

	res, err := r.db.ExecContext(ctx, query, configBytes, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update bot config: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return model.ErrBotNotFound
	}

	return nil
}

func (r *postgresRepository) UpdateStatus(ctx context.Context, id string, status model.BotStatus) error {
	query := `
		UPDATE bots
		SET status = $1, updated_at = $2
		WHERE id = $3
	`

	res, err := r.db.ExecContext(ctx, query, string(status), time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update bot status: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return model.ErrBotNotFound
	}

	return nil
}

func (r *postgresRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM bots WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete bot: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return model.ErrBotNotFound
	}

	return nil
}
