package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/valyala/fasthttp"

	saiTypes "github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-storage/internal/service"
	"github.com/saiset-co/sai-storage/types"
)

type Handler struct {
	service *service.StorageService
}

func NewHandler(service *service.StorageService) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) CreateDocuments(ctx *saiTypes.RequestCtx) {
	var req types.CreateDocumentsRequest
	if err := ctx.ReadJSON(&req); err != nil {
		h.logRequest(ctx, "", map[string]interface{}{
			"error":    err.Error(),
			"raw_body": string(ctx.PostBody()),
		})
		ctx.Error(err, http.StatusInternalServerError)
		return
	}

	h.logRequest(ctx, req.Collection, req)

	response, err := h.service.CreateDocuments(ctx, req)
	if err != nil {
		ctx.Error(err, http.StatusInternalServerError)
		return
	}

	ctx.SuccessJSON(response)
}

func (h *Handler) ReadDocuments(ctx *saiTypes.RequestCtx) {
	req := types.ReadDocumentsRequest{
		Collection: string(ctx.QueryArgs().Peek("collection")),
		Limit:      ctx.QueryArgs().GetUintOrZero("limit"),
		Skip:       ctx.QueryArgs().GetUintOrZero("skip"),
		Count:      int(ctx.QueryArgs().GetUintOrZero("count")),
	}

	// Try to read JSON body if present (for complex filters)
	if len(ctx.PostBody()) > 0 {
		if err := ctx.ReadJSON(&req); err != nil {
			h.logRequest(ctx, req.Collection, map[string]interface{}{
				"error":    err.Error(),
				"raw_body": string(ctx.PostBody()),
			})
			ctx.Error(saiTypes.WrapError(err, "Invalid JSON in request body"), fasthttp.StatusBadRequest)
			return
		}
	}

	// Collection is required
	if req.Collection == "" {
		h.logRequest(ctx, req.Collection, req)
		ctx.Error(saiTypes.NewError("collection parameter is required"), fasthttp.StatusBadRequest)
		return
	}

	h.logRequest(ctx, req.Collection, req)

	response, err := h.service.ReadDocuments(ctx, req)
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	ctx.SuccessJSON(response)
}

func (h *Handler) AggregateDocuments(ctx *saiTypes.RequestCtx) {
	req := types.AggregateDocumentsRequest{
		Collection: string(ctx.QueryArgs().Peek("collection")),
		Limit:      ctx.QueryArgs().GetUintOrZero("limit"),
		Skip:       ctx.QueryArgs().GetUintOrZero("skip"),
		Count:      int(ctx.QueryArgs().GetUintOrZero("count")),
	}

	// Try to read JSON body if present (for pipeline/aggregation params)
	if len(ctx.PostBody()) > 0 {
		if err := ctx.ReadJSON(&req); err != nil {
			h.logRequest(ctx, req.Collection, map[string]interface{}{
				"error":    err.Error(),
				"raw_body": string(ctx.PostBody()),
			})
			ctx.Error(saiTypes.WrapError(err, "Invalid JSON in request body"), fasthttp.StatusBadRequest)
			return
		}
	}

	// Collection is required
	if req.Collection == "" {
		h.logRequest(ctx, req.Collection, req)
		ctx.Error(saiTypes.NewError("collection parameter is required"), fasthttp.StatusBadRequest)
		return
	}

	h.logRequest(ctx, req.Collection, req)

	response, err := h.service.AggregateDocuments(ctx, req)
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	ctx.SuccessJSON(response)
}

func (h *Handler) UpdateDocuments(ctx *saiTypes.RequestCtx) {
	var req types.UpdateDocumentsRequest
	if err := ctx.ReadJSON(&req); err != nil {
		h.logRequest(ctx, "", map[string]interface{}{
			"error":    err.Error(),
			"raw_body": string(ctx.PostBody()),
		})
		ctx.Error(saiTypes.WrapError(err, "Invalid JSON in request body"), fasthttp.StatusInternalServerError)
		return
	}

	h.logRequest(ctx, req.Collection, req)

	response, err := h.service.UpdateDocuments(ctx, req)
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	ctx.SuccessJSON(response)
}

func (h *Handler) DeleteDocuments(ctx *saiTypes.RequestCtx) {
	var req types.DeleteDocumentsRequest
	if err := ctx.ReadJSON(&req); err != nil {
		h.logRequest(ctx, "", map[string]interface{}{
			"error":    err.Error(),
			"raw_body": string(ctx.PostBody()),
		})
		ctx.Error(saiTypes.WrapError(err, "Invalid JSON in request body"), fasthttp.StatusBadRequest)
		return
	}

	h.logRequest(ctx, req.Collection, req)

	response, err := h.service.DeleteDocuments(ctx, req)
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	ctx.SuccessJSON(response)
}

func (h *Handler) logRequest(ctx *saiTypes.RequestCtx, collection string, body interface{}) {
	now := time.Now()
	requestInfo := map[string]interface{}{
		"collection":   collection,
		"method":       string(ctx.Method()),
		"path":         string(ctx.Path()),
		"request_time": now.Format(time.RFC3339),
		"request_unix": now.Unix(),
	}

	if body != nil {
		if b, err := ctx.Marshal(body); err == nil {
			requestInfo["body"] = string(b)
		}
	}

	if requestID := ctx.Request.Header.Peek("X-Request-ID"); len(requestID) > 0 {
		requestInfo["request_id"] = string(requestID)
	}

	if username := h.getAuthenticatedUser(ctx); username != "" {
		requestInfo["user"] = username
	}
	if authType := h.getAuthType(ctx); authType != "" {
		requestInfo["auth_type"] = authType
	}

	if ip := h.getRemoteIP(ctx); ip != "" {
		requestInfo["ip"] = ip
	}

	if userID := string(ctx.Request.Header.Peek("X-Forwarded-User-ID")); userID != "" {
		requestInfo["user_id"] = userID
	}

	h.service.LogRequest(ctx, collection, requestInfo)
}

func (h *Handler) getAuthenticatedUser(ctx *saiTypes.RequestCtx) string {
	if v := ctx.UserValue("authenticated_user"); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if v := ctx.UserValue("user_id"); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if v := ctx.UserValue("id"); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func (h *Handler) getAuthType(ctx *saiTypes.RequestCtx) string {
	if v := ctx.UserValue("auth_type"); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func (h *Handler) getRemoteIP(ctx *saiTypes.RequestCtx) string {
	if forwarded := ctx.Request.Header.Peek("X-Forwarded-For"); len(forwarded) > 0 {
		forwardedStr := string(forwarded)
		if comma := strings.IndexByte(forwardedStr, ','); comma > 0 {
			return strings.TrimSpace(forwardedStr[:comma])
		}
		return forwardedStr
	}

	if realIP := ctx.Request.Header.Peek("X-Real-IP"); len(realIP) > 0 {
		return string(realIP)
	}

	return ctx.RemoteIP().String()
}
