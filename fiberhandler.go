package fiberhandler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/prongbang/fibererror"
	"github.com/prongbang/goerror"
	"github.com/prongbang/gopkg/core"
	"github.com/prongbang/gopkg/multipartx"
	"github.com/prongbang/gopkg/streamx"
	"github.com/prongbang/gopkg/typex"
)

type DoFunc func(ctx context.Context) (any, error)

type ApiHandler interface {
	Do(c *fiber.Ctx, requestPtr any, validateRequest bool, doFunc DoFunc) error
	DoMultipart(c *fiber.Ctx, requestPtr any, validateRequest bool, allowedTypes []string, doFunc DoFunc) error
}

type apiHandler[T any] struct {
	Response    fibererror.Response
	Validate    *validator.Validate
	TokenParser *TokenParser[T]
}

func (h *apiHandler[T]) getUserRequestInfo(c *fiber.Ctx) *T {
	return h.getRequestInfo(c, func(c *fiber.Ctx) string {
		if multipartx.IsMultipartForm(c) {
			return c.FormValue("token")
		}
		return h.getRequestToken(c)
	})
}

func (h *apiHandler[T]) getRequestInfo(c *fiber.Ctx, onRequestToken func(c *fiber.Ctx) string) *T {
	tequestToken := onRequestToken(c)
	if core.IsEmpty(tequestToken) {
		return nil
	}

	tokenData, err := (*h.TokenParser).ParseToken(tequestToken)
	if err != nil {
		slog.Error("Failed to parse token", slog.String("error", err.Error()))
		return nil
	}
	return tokenData
}

func (h *apiHandler[T]) getRequestToken(c *fiber.Ctx) string {
	requestToken := core.ExtractToken(core.Authorization(c))
	if core.IsEmpty(requestToken) {
		accessToken := core.AccessToken{}
		_ = c.BodyParser(&accessToken)
		return accessToken.Token
	}
	return requestToken
}

func (h *apiHandler[T]) DoMultipart(c *fiber.Ctx, requestPtr any, validateRequest bool, allowedTypes []string, doFunc DoFunc) error {
	if c.Method() == http.MethodGet || c.Method() == http.MethodDelete {
		return nil
	}

	if requestPtr == nil {
		return nil
	}

	// Ensure multipart form is parsed
	if _, err := c.MultipartForm(); err != nil {
		slog.Error("Invalid request", slog.String("error", err.Error()))
		return h.Response.With(c).Response(goerror.NewBadRequest())
	}

	// Validate type assertion for Multipart Request
	multipartReq, ok := requestPtr.(multipartx.Request)
	if !ok {
		slog.Error("Invalid request", slog.String("error", "the task requires implementing the multipartx.Request"))
		return h.Response.With(c).Response(goerror.NewBadRequest("Invalid request type"))
	}

	// Process form fields
	for fieldName, fieldPtr := range multipartReq.FormFields() {
		if err := typex.SetField(c.FormValue(fieldName), fieldPtr); err != nil {
			slog.Error("Invalid request", slog.String("error", err.Error()))
			return h.Response.With(c).Response(goerror.NewBadRequest(fmt.Sprintf("Invalid value for field '%s': %v", fieldName, err)))
		}
	}

	// Process file fields with optimized allocation
	var allowedMimeTypes map[string]bool
	if validateRequest && len(allowedTypes) > 0 {
		allowedMimeTypes = make(map[string]bool, len(allowedTypes))
		for _, v := range allowedTypes {
			allowedMimeTypes[v] = true
		}
	}

	for fieldName, filePtr := range multipartReq.FileFields() {
		if fileHeader, err := c.FormFile(fieldName); err == nil {
			if validateRequest && allowedMimeTypes != nil {
				if multipartx.ValidateMimeType(fileHeader, allowedMimeTypes) == nil {
					*filePtr = fileHeader
				}
			} else {
				*filePtr = fileHeader
			}
		}
	}

	// Validate request if needed
	if validateRequest {
		if err := h.Validate.Struct(requestPtr); err != nil {
			slog.Error("Invalid request", slog.String("error", err.Error()))
			return h.Response.With(c).Response(NewDataInvalidError())
		}
	}

	requestInfo := &core.RequestInfo[T]{
		Claims: h.getUserRequestInfo(c),
	}

	reqModel, ok := requestPtr.(core.Request[T])
	if ok {
		reqModel.SetRequestInfo(requestInfo)
	}

	data, err := doFunc(c.UserContext())
	if err != nil {
		slog.Error("Invalid request", slog.String("error", err.Error()))
		return h.Response.With(c).Response(err)
	}

	return h.Response.With(c).Response(goerror.NewOK(data))
}

func (h *apiHandler[T]) Do(c *fiber.Ctx, requestPtr any, validateRequest bool, doFunc DoFunc) error {
	err := h.requestParserIfNeeded(c, requestPtr)
	if err != nil {
		return err
	}

	if validateRequest {
		err := h.Validate.Struct(requestPtr)
		if err != nil {
			slog.Error("Invalid request", slog.String("error", err.Error()))
			return h.Response.With(c).Response(NewDataInvalidError())
		}
	}

	requestInfo := &core.RequestInfo[T]{
		Claims: h.getUserRequestInfo(c),
	}

	reqModel, ok := requestPtr.(core.Request[T])
	if ok {
		reqModel.SetRequestInfo(requestInfo)
	}

	data, err := doFunc(c.UserContext())
	if err != nil {
		slog.Error("Invalid request", slog.String("error", err.Error()))
		return h.Response.With(c).Response(err)
	}

	streamData, ok := data.(*streamx.Stream)
	if ok {
		return h.sendStream(c, streamData)
	}

	return h.Response.With(c).Response(goerror.NewOK(data))
}

func (h *apiHandler[T]) sendStream(c *fiber.Ctx, streamData *streamx.Stream) error {
	streamx.AttachmentHeader(c, streamData.ContentType, streamData.Filename)
	if streamData.Size != nil {
		return c.SendStream(streamData.Data, *streamData.Size)
	}
	return c.SendStream(streamData.Data)
}

func (h *apiHandler[T]) requestParserIfNeeded(c *fiber.Ctx, requestPtr interface{}) error {
	if requestPtr == nil {
		slog.Error("Invalid request", slog.String("error", "the request is null"))
		return nil
	}

	switch c.Method() {
	case http.MethodGet, http.MethodDelete:
		err := c.QueryParser(requestPtr)
		if err != nil {
			slog.Error("Invalid request", slog.String("error", err.Error()))
			return h.Response.With(c).Response(goerror.NewBadRequest())
		}
	default:
		err := c.BodyParser(requestPtr)
		if err != nil {
			slog.Error("Invalid request", slog.String("error", err.Error()))
			return h.Response.With(c).Response(goerror.NewBadRequest())
		}
	}

	return nil
}

func New[T any](response fibererror.Response, validate *validator.Validate, tokenParser ...TokenParser[T]) ApiHandler {
	var newTokenParser TokenParser[T]
	if len(tokenParser) == 0 {
		newTokenParser = NewJWTParser[T]()
	} else {
		newTokenParser = tokenParser[0]
	}

	return &apiHandler[T]{
		Response:    response,
		Validate:    validate,
		TokenParser: &newTokenParser,
	}
}
