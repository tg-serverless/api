package handler

import (
	"errors"
	"serverless-api/api/internal/dto"
	"serverless-api/api/internal/model"
	k8srepo "serverless-api/api/internal/repository/k8s"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type Handlers struct {
	BotsRepo     model.BotRepository
	ShardingRepo model.ShardingRepository
	K8S          model.K8SOperator
	Logger       *zap.Logger
}

func (h *Handlers) sugar() *zap.SugaredLogger {
	if h != nil && h.Logger != nil {
		return h.Logger.Sugar()
	}
	return zap.NewNop().Sugar()
}

func (h *Handlers) DeployBot(c *fiber.Ctx) error {
	var req dto.DeployBotRequest
	if err := c.BodyParser(&req); err != nil {
		h.sugar().Warnf("bad deploy request: %v", err)
		return c.Status(fiber.StatusBadRequest).SendString("invalid request")
	}

	if req.Bot.GitRepoURL == "" {
		return c.Status(fiber.StatusBadRequest).SendString("git repo url is required")
	}
	if req.NumTopics <= 0 {
		return c.Status(fiber.StatusBadRequest).SendString("topics count must be > 0")
	}

	botModel := &model.Bot{
		Name:          extractNameFromGitURL(req.Bot.GitRepoURL),
		GitURL:        req.Bot.GitRepoURL,
		GitEntrypoint: req.Bot.GitEntrypoint,
		Config: model.BotConfig{
			MinReplicas: int(req.Bot.MinReplicas),
			MaxReplicas: int(req.Bot.MaxReplicas),
			NumTopics:   req.NumTopics,
			Env:         map[string]string{},
			Resources: model.ResourceConfig{
				CPU:    req.Bot.Resources.CPU,
				Memory: req.Bot.Resources.Memory,
			},
		},
	}

	id, err := h.BotsRepo.Create(c.Context(), botModel)
	if err != nil {
		h.sugar().Errorf("failed to create bot in repository: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("failed to create bot")
	}
	botModel.ID = id

	if err := h.ShardingRepo.SetCount(c.Context(), id, uint32(req.NumTopics)); err != nil {
		h.sugar().Errorf("sharding set failed for bot %s: %v", id, err)
		if derr := h.BotsRepo.Delete(c.Context(), id); derr != nil {
			h.sugar().Errorf("rollback delete failed for bot %s: %v", id, derr)
			return c.Status(fiber.StatusInternalServerError).SendString("set sharding failed and rollback failed")
		}
		return c.Status(fiber.StatusInternalServerError).SendString("set sharding failed")
	}

	kbot := k8srepo.Bot{
		BotID:         botModel.ID,
		GitRepoURL:    botModel.GitURL,
		GitEntrypoint: botModel.GitEntrypoint,
		MinReplicas:   float64(botModel.Config.MinReplicas),
		MaxReplicas:   float64(botModel.Config.MaxReplicas),
	}

	if h.K8S == nil {
		h.sugar().Infof("k8s operator is nil, skipping deploy for bot %s", id)
	} else {
		if err := h.K8S.DeployBot(c.Context(), kbot); err != nil {
			h.sugar().Errorf("deploy k8s failed for bot %s: %v", id, err)
			if derr := h.BotsRepo.Delete(c.Context(), id); derr != nil {
				h.sugar().Errorf("rollback delete failed for bot %s: %v", id, derr)
				return c.Status(fiber.StatusInternalServerError).SendString("deploy k8s failed and rollback failed")
			}
			return c.Status(fiber.StatusInternalServerError).SendString("deploy k8s failed")
		}
	}

	if err := h.BotsRepo.UpdateStatus(c.Context(), id, model.BotStatusActive); err != nil {
		h.sugar().Errorf("update status failed for bot %s: %v", id, err)
		if derr := h.BotsRepo.Delete(c.Context(), id); derr != nil {
			h.sugar().Errorf("rollback delete failed for bot %s: %v", id, derr)
			return c.Status(fiber.StatusInternalServerError).SendString("update status failed and rollback failed")
		}
		return c.Status(fiber.StatusInternalServerError).SendString("update status failed")
	}

	h.sugar().Infof("bot %s deployed successfully", id)
	return c.Status(fiber.StatusAccepted).SendString("deploy started")
}

func (h *Handlers) GetBot(c *fiber.Ctx) error {
	id := c.Params("id")
	bot, err := h.BotsRepo.GetByID(c.Context(), id)
	if err != nil {
		h.sugar().Errorf("get bot failed: %v", err)
		if errors.Is(err, model.ErrBotNotFound) {
			return c.SendStatus(fiber.StatusNotFound)
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.JSON(bot)
}

func (h *Handlers) UpdateBot(c *fiber.Ctx) error {
	id := c.Params("id")
	var cfg model.BotConfig
	if err := c.BodyParser(&cfg); err != nil {
		h.sugar().Warnf("bad update config request: %v", err)
		return c.Status(fiber.StatusBadRequest).SendString("invalid request")
	}
	if _, err := h.BotsRepo.GetByID(c.Context(), id); err != nil {
		if errors.Is(err, model.ErrBotNotFound) {
			h.sugar().Warnf("bot %s not found", id)
			return c.SendStatus(fiber.StatusNotFound)
		}
		h.sugar().Errorf("failed to get bot %s: %v", id, err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	if err := h.BotsRepo.UpdateConfig(c.Context(), id, cfg); err != nil {
		h.sugar().Errorf("failed to update config for bot %s: %v", id, err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	h.sugar().Infof("bot %s config updated", id)
	return c.SendString("updated")
}

func (h *Handlers) DeleteBot(c *fiber.Ctx) error {
	id := c.Params("id")
	if _, err := h.BotsRepo.GetByID(c.Context(), id); err != nil {
		if errors.Is(err, model.ErrBotNotFound) {
			h.sugar().Warnf("bot %s not found", id)
			return c.SendStatus(fiber.StatusNotFound)
		}
		h.sugar().Errorf("failed to get bot %s: %v", id, err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	if err := h.ShardingRepo.SetCount(c.Context(), id, 0); err != nil {
		h.sugar().Warnf("failed to reset sharding for bot %s: %v", id, err)
	}

	if err := h.BotsRepo.Delete(c.Context(), id); err != nil {
		h.sugar().Errorf("failed to delete bot %s from db: %v", id, err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	h.sugar().Infof("bot %s deleted", id)
	return c.SendString("deleted")
}

func extractNameFromGitURL(gitURL string) string {
	if gitURL == "" {
		return ""
	}
	name := gitURL
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '/' {
			name = name[i+1:]
			break
		}
	}
	if len(name) >= 4 && name[len(name)-4:] == ".git" {
		name = name[:len(name)-4]
	}
	return name
}
