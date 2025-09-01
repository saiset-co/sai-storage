package handlers

import (
	"net/http"

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
		ctx.Error(err, http.StatusInternalServerError)
		return
	}

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
	}

	// Try to read JSON body if present (for complex filters)
	if len(ctx.PostBody()) > 0 {
		if err := ctx.ReadJSON(&req); err != nil {
			ctx.Error(saiTypes.WrapError(err, "Invalid JSON in request body"), fasthttp.StatusBadRequest)
			return
		}
	}

	// Collection is required
	if req.Collection == "" {
		ctx.Error(saiTypes.NewError("collection parameter is required"), fasthttp.StatusBadRequest)
		return
	}

	response, err := h.service.ReadDocuments(ctx, req)
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	ctx.SuccessJSON(response)
}

func (h *Handler) UpdateDocuments(ctx *saiTypes.RequestCtx) {
	var req types.UpdateDocumentsRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.Error(saiTypes.WrapError(err, "Invalid JSON in request body"), fasthttp.StatusInternalServerError)
		return
	}

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
		ctx.Error(saiTypes.WrapError(err, "Invalid JSON in request body"), fasthttp.StatusBadRequest)
		return
	}

	response, err := h.service.DeleteDocuments(ctx, req)
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	ctx.SuccessJSON(response)
}
