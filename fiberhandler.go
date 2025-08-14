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

type model[T any] struct {
	Type T
}

type DataInvalidError struct {
	goerror.Body
}

// Error implements error.
func (c *DataInvalidError) Error() string {
	return c.Message
}

func NewDataInvalidError() error {
	return &DataInvalidError{
		Body: goerror.Body{
			Code:    "CLE029",
			Message: "Invalid data provided",
		},
	}
}

func GetRequestToken(c *fiber.Ctx) string {
	requestToken := core.ExtractToken(core.Authorization(c))
	if core.IsEmpty(requestToken) {
		accessToken := core.AccessToken{}
		_ = c.BodyParser(&accessToken)
		return accessToken.Token
	}
	return requestToken
}

func GetRequestInfo(c *fiber.Ctx, onRequestToken func(c *fiber.Ctx) string) *core.UserRequestInfo {
	userRequestInfo := &core.UserRequestInfo{
		HasUserRequest: false,
	}
	userRequestToken := onRequestToken(c)
	if core.IsEmpty(userRequestToken) {
		return userRequestInfo
	}

	tokenData, err := core.GetTokenData(userRequestToken)
	if err != nil {
		return userRequestInfo
	}
	if tokenData != nil {
		userRequestInfo.Id = tokenData.UserID
		userRequestInfo.Roles = tokenData.Roles
		userRequestInfo.HasUserRequest = true
	}

	permissionId := c.Locals("permissionId")
	if permissionId != nil {
		userRequestInfo.ApiInfo = &core.ApiInfo{
			PermissionId: permissionId.(string),
		}
	}

	return userRequestInfo
}

type DoFunc func(ctx context.Context) (any, error)

type ApiHandler interface {
	GetUserRequestInfo(c *fiber.Ctx) *core.UserRequestInfo
	Do(c *fiber.Ctx, requestPtr any, validateRequest bool, doFunc DoFunc) error
	DoMultipart(c *fiber.Ctx, requestPtr any, validateRequest bool, allowedTypes []string, doFunc DoFunc) error
}

type apiHandler struct {
	Response fibererror.Response
	Validate *validator.Validate
}

func (h *apiHandler) GetUserRequestInfo(c *fiber.Ctx) *core.UserRequestInfo {
	return GetRequestInfo(c, func(c *fiber.Ctx) string {
		if multipartx.IsMultipartForm(c) {
			return c.FormValue("token")
		}
		return GetRequestToken(c)
	})
}

func (h *apiHandler) DoMultipart(c *fiber.Ctx, requestPtr any, validateRequest bool, allowedTypes []string, doFunc DoFunc) error {
	if c.Method() == http.MethodGet {
		return nil
	}

	if requestPtr == nil {
		return nil
	}

	// Ensure multipart form is parsed
	if _, err := c.MultipartForm(); err != nil {
		return h.Response.With(c).Response(goerror.NewBadRequest())
	}

	// Validate type assertion for Multipart Request
	multipartReq, ok := requestPtr.(multipartx.Request)
	if !ok {
		return h.Response.With(c).Response(goerror.NewBadRequest("Invalid request type"))
	}

	// Process form fields
	for fieldName, fieldPtr := range multipartReq.FormFields() {
		if err := typex.SetField(c.FormValue(fieldName), fieldPtr); err != nil {
			return h.Response.With(c).Response(goerror.NewBadRequest(fmt.Sprintf("Invalid value for field '%s': %v", fieldName, err)))
		}
	}

	// Process file fields
	allowedMimeTypes := map[string]bool{}
	if validateRequest {
		for _, v := range allowedTypes {
			allowedMimeTypes[v] = true
		}
	}

	for fieldName, filePtr := range multipartReq.FileFields() {
		if fileHeader, err := c.FormFile(fieldName); err == nil {
			if validateRequest {
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
			return h.Response.With(c).Response(NewDataInvalidError())
		}
	}

	requestInfo := &core.RequestInfo{
		UserRequestInfo: h.GetUserRequestInfo(c),
	}

	reqModel, ok := requestPtr.(core.Request)
	if ok {
		reqModel.SetRequestInfo(requestInfo)
	}

	data, err := doFunc(c.UserContext())
	if err != nil {
		return h.Response.With(c).Response(err)
	}

	return h.Response.With(c).Response(goerror.NewOK(data))
}

func (h *apiHandler) Do(c *fiber.Ctx, requestPtr any, validateRequest bool, doFunc DoFunc) error {
	_, err := h.bodyParserIfRequired(c, requestPtr)
	if err != nil {
		return err
	}

	if validateRequest {
		err := h.Validate.Struct(requestPtr)
		if err != nil {
			slog.Error("Invalid request", slog.String("err", err.Error()))
			return h.Response.With(c).Response(NewDataInvalidError())
		}
	}

	requestInfo := &core.RequestInfo{
		UserRequestInfo: h.GetUserRequestInfo(c),
	}

	reqModel, ok := requestPtr.(core.Request)
	if ok {
		reqModel.SetRequestInfo(requestInfo)
	}

	data, err := doFunc(c.UserContext())
	if err != nil {
		return err
	}

	streamData, ok := data.(*streamx.Stream)
	if ok {
		return h.sendStream(c, streamData)
	}

	return h.Response.With(c).Response(goerror.NewOK(data))
}

func (h *apiHandler) sendStream(c *fiber.Ctx, streamData *streamx.Stream) error {
	streamx.AttachmentHeader(c, streamData.ContentType, streamData.Filename)
	if streamData.Size != nil {
		return c.SendStream(streamData.Data, *streamData.Size)
	}
	return c.SendStream(streamData.Data)
}

func (h *apiHandler) bodyParserIfRequired(c *fiber.Ctx, requestPtr interface{}) (bool, error) {
	if c.Method() == http.MethodGet {
		return false, nil
	}

	if requestPtr == nil {
		return false, nil
	}

	err := c.BodyParser(requestPtr)
	if err != nil {
		return false, h.Response.With(c).Response(goerror.NewBadRequest())
	}

	return true, nil
}

func New(response fibererror.Response, validate *validator.Validate) ApiHandler {
	return &apiHandler{
		Response: response,
		Validate: validate,
	}
}
