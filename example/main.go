package main

import (
	"context"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/golang-jwt/jwt/v5"
	"github.com/prongbang/fibererror"
	"github.com/prongbang/fiberhandler"
	"github.com/prongbang/gopkg/core"
)

type Claims struct {
	Name  string `json:"name"`
	Admin bool   `json:"admin"`
	jwt.RegisteredClaims
}

type GetRequest struct {
	Message                  string `json:"message"`
	core.RequestInfo[Claims] `json:"requestInfo"`
}

type DeleteRequest struct {
	Message                  string `json:"message"`
	core.RequestInfo[Claims] `json:"requestInfo"`
}

type PutRequest struct {
	Message                  string `json:"message"`
	core.RequestInfo[Claims] `json:"requestInfo"`
}

type PostRequest struct {
	Message                  string `json:"message"`
	Token                    string `json:"token"`
	core.RequestInfo[Claims] `json:"requestInfo"`
}

func main() {
	app := fiber.New()
	app.Use(logger.New())

	response := fibererror.New()
	validate := validator.New()
	handle := fiberhandler.New[Claims](response, validate)

	app.Get("/get", func(c *fiber.Ctx) error {
		req := GetRequest{}
		return handle.Do(c, &req, true, func(ctx context.Context) (any, error) {
			return req, nil
		})
	})
	app.Delete("/delete", func(c *fiber.Ctx) error {
		req := DeleteRequest{}
		return handle.Do(c, &req, true, func(ctx context.Context) (any, error) {
			return req, nil
		})
	})
	app.Post("/post", func(c *fiber.Ctx) error {
		req := PostRequest{}
		return handle.Do(c, &req, true, func(ctx context.Context) (any, error) {
			return req, nil
		})
	})
	app.Put("/put", func(c *fiber.Ctx) error {
		req := PutRequest{}
		return handle.Do(c, &req, true, func(ctx context.Context) (any, error) {
			return req, nil
		})
	})

	app.Listen(":8080")
}
