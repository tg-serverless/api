package handler

import "github.com/gofiber/fiber/v2"

func (h *Handlers) RegisterRoutes(app *fiber.App) {
	app.Post("/deploy", h.DeployBot)
	app.Get("/bots/:id", h.GetBot)
	app.Put("/bots/:id", h.UpdateBot)
	app.Delete("/bots/:id", h.DeleteBot)
}
