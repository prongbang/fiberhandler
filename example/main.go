package main

import (
	"context"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/prongbang/fibererror"
	"github.com/prongbang/fiberhandler"
	"github.com/prongbang/gopkg/core"
)

type Model struct {
	Message     string            `json:"message"`
	RequestInfo *core.RequestInfo `json:"-"`
}

func (r *Model) SetRequestInfo(info *core.RequestInfo) {
	r.RequestInfo = info
}

func main() {
	app := fiber.New()

	response := fibererror.New()
	validate := validator.New()
	handle := fiberhandler.New(response, validate)

	app.Post("/post", func(c *fiber.Ctx) error {
		req := Model{}
		return handle.Do(c, &req, true, func(ctx context.Context) (any, error) {
			return req, nil
		})
	})

	app.Listen(":8080")
}
