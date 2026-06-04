package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/saiset-co/sai-service/admin"
	saiTypes "github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-storage/types"
)

func (h *Handler) CreateIndex(ctx *saiTypes.RequestCtx) {
	var req types.CreateIndexRequest
	if err := ctx.ReadJSON(&req); err != nil {
		admin.WriteActionJSON(ctx, "", err)
		return
	}
	if req.Collection == "" || len(req.Keys) == 0 {
		admin.WriteActionJSON(ctx, "", fmt.Errorf("collection and keys are required"))
		return
	}
	if err := h.service.GetRepo().CreateIndex(context.Background(), req); err != nil {
		admin.WriteActionJSON(ctx, "", err)
		return
	}
	admin.WriteActionJSON(ctx, "Индекс успешно создан", nil)
}

func (h *Handler) RestoreUpdate(ctx *saiTypes.RequestCtx) {
	collection := strings.TrimSpace(string(ctx.FormValue("collection")))
	opID := strings.TrimSpace(string(ctx.FormValue("archive_operation_id")))
	if collection == "" || opID == "" {
		admin.WriteActionJSON(ctx, "", fmt.Errorf("collection и archive_operation_id обязательны"))
		return
	}

	archiveCollection := fmt.Sprintf("%s_update_archive", collection)

	check, _, _ := h.service.GetRepo().ReadDocuments(context.Background(), types.ReadDocumentsRequest{
		Collection: archiveCollection,
		Filter:     map[string]interface{}{"archive_operation_id": opID, "restored_at": map[string]interface{}{"$gt": 0}},
		Limit:      1,
	})
	if len(check) > 0 {
		admin.WriteActionJSON(ctx, "", fmt.Errorf("операция уже была восстановлена"))
		return
	}

	docs, _, err := h.service.GetRepo().ReadDocuments(context.Background(), types.ReadDocumentsRequest{
		Collection: archiveCollection,
		Filter:     map[string]interface{}{"archive_operation_id": opID},
	})
	if err != nil {
		admin.WriteActionJSON(ctx, "", err)
		return
	}

	restored := 0
	for _, doc := range docs {
		internalID, ok := doc["internal_id"]
		if !ok {
			continue
		}
		data := cleanArchiveFields(doc)
		h.service.GetRepo().DeleteDocuments(context.Background(), types.DeleteDocumentsRequest{
			Collection: collection,
			Filter:     map[string]interface{}{"internal_id": internalID},
		})
		if _, err := h.service.GetRepo().CreateDocuments(context.Background(), types.CreateDocumentsRequest{
			Collection: collection,
			Data:       []interface{}{data},
		}); err == nil {
			restored++
		}
	}

	h.service.GetRepo().UpdateDocuments(context.Background(), types.UpdateDocumentsRequest{
		Collection: archiveCollection,
		Filter:     map[string]interface{}{"archive_operation_id": opID},
		Data:       map[string]interface{}{"$set": map[string]interface{}{"restored_at": time.Now().UnixNano()}},
	})

	admin.WriteActionJSON(ctx, fmt.Sprintf("Восстановлено документов: %d", restored), nil)
}

func (h *Handler) RestoreDelete(ctx *saiTypes.RequestCtx) {
	collection := strings.TrimSpace(string(ctx.FormValue("collection")))
	opID := strings.TrimSpace(string(ctx.FormValue("archive_operation_id")))
	if collection == "" || opID == "" {
		admin.WriteActionJSON(ctx, "", fmt.Errorf("collection и archive_operation_id обязательны"))
		return
	}

	archiveCollection := fmt.Sprintf("%s_delete_archive", collection)

	check, _, _ := h.service.GetRepo().ReadDocuments(context.Background(), types.ReadDocumentsRequest{
		Collection: archiveCollection,
		Filter:     map[string]interface{}{"archive_operation_id": opID, "restored_at": map[string]interface{}{"$gt": 0}},
		Limit:      1,
	})
	if len(check) > 0 {
		admin.WriteActionJSON(ctx, "", fmt.Errorf("операция уже была восстановлена"))
		return
	}

	docs, _, err := h.service.GetRepo().ReadDocuments(context.Background(), types.ReadDocumentsRequest{
		Collection: archiveCollection,
		Filter:     map[string]interface{}{"archive_operation_id": opID},
	})
	if err != nil {
		admin.WriteActionJSON(ctx, "", err)
		return
	}

	restored := 0
	for _, doc := range docs {
		internalID, ok := doc["internal_id"]
		if !ok {
			continue
		}
		data := cleanArchiveFields(doc)
		h.service.GetRepo().DeleteDocuments(context.Background(), types.DeleteDocumentsRequest{
			Collection: collection,
			Filter:     map[string]interface{}{"internal_id": internalID},
		})
		if _, err := h.service.GetRepo().CreateDocuments(context.Background(), types.CreateDocumentsRequest{
			Collection: collection,
			Data:       []interface{}{data},
		}); err == nil {
			restored++
		}
	}

	h.service.GetRepo().UpdateDocuments(context.Background(), types.UpdateDocumentsRequest{
		Collection: archiveCollection,
		Filter:     map[string]interface{}{"archive_operation_id": opID},
		Data:       map[string]interface{}{"$set": map[string]interface{}{"restored_at": time.Now().UnixNano()}},
	})

	admin.WriteActionJSON(ctx, fmt.Sprintf("Восстановлено документов: %d", restored), nil)
}

func (h *Handler) SetSlowQueryThreshold(ctx *saiTypes.RequestCtx) {
	rawMs := strings.TrimSpace(string(ctx.FormValue("slow_ms")))
	ms := 0
	if rawMs != "" {
		parsed, err := strconv.Atoi(rawMs)
		if err != nil {
			admin.WriteActionJSON(ctx, "", fmt.Errorf("неверное значение: %s", rawMs))
			return
		}
		ms = parsed
	}
	h.service.SetSlowQueryThreshold(ms)
	admin.WriteActionJSON(ctx, fmt.Sprintf("Порог установлен: %d мс", ms), nil)
}

func (h *Handler) SaveCustomQuery(ctx *saiTypes.RequestCtx) {
	query := strings.TrimSpace(string(ctx.FormValue("query")))
	name := strings.TrimSpace(string(ctx.FormValue("name")))
	description := strings.TrimSpace(string(ctx.FormValue("description")))

	if query == "" {
		admin.WriteActionJSON(ctx, "", fmt.Errorf("запрос обязателен"))
		return
	}

	collection, operation, err := parseMongoShellQuery(query)
	if err != nil {
		admin.WriteActionJSON(ctx, "", err)
		return
	}

	if name == "" {
		name = query
		if len([]rune(name)) > 60 {
			name = string([]rune(name)[:57]) + "..."
		}
	}

	_, err = h.service.GetRepo().CreateDocuments(context.Background(), types.CreateDocumentsRequest{
		Collection: "_admin_custom_queries",
		Data: []interface{}{map[string]interface{}{
			"name":        name,
			"collection":  collection,
			"operation":   operation,
			"query_raw":   query,
			"description": description,
		}},
	})
	if err != nil {
		admin.WriteActionJSON(ctx, "", err)
		return
	}
	admin.WriteActionJSON(ctx, "Запрос сохранён", nil)
}

func parseMongoShellQuery(q string) (collection, operation string, err error) {
	if !strings.HasPrefix(q, "db.") {
		return "", "", fmt.Errorf("запрос должен начинаться с db.")
	}
	rest := q[3:]
	dotIdx := strings.Index(rest, ".")
	if dotIdx < 0 {
		return "", "", fmt.Errorf("не найдено имя коллекции")
	}
	collection = rest[:dotIdx]
	rest = rest[dotIdx+1:]
	parenIdx := strings.Index(rest, "(")
	if parenIdx < 0 {
		return "", "", fmt.Errorf("не найдена открывающая скобка")
	}
	operation = rest[:parenIdx]
	if collection == "" || operation == "" {
		return "", "", fmt.Errorf("неверный формат: db.collection.operation(...)")
	}
	return collection, operation, nil
}

func (h *Handler) UpdateCustomQuery(ctx *saiTypes.RequestCtx) {
	id := strings.TrimSpace(string(ctx.FormValue("id")))
	query := strings.TrimSpace(string(ctx.FormValue("query")))
	name := strings.TrimSpace(string(ctx.FormValue("name")))
	description := strings.TrimSpace(string(ctx.FormValue("description")))

	if id == "" || query == "" {
		admin.WriteActionJSON(ctx, "", fmt.Errorf("id и запрос обязательны"))
		return
	}

	collection, operation, err := parseMongoShellQuery(query)
	if err != nil {
		admin.WriteActionJSON(ctx, "", err)
		return
	}

	if name == "" {
		name = query
		if len([]rune(name)) > 60 {
			name = string([]rune(name)[:57]) + "..."
		}
	}

	_, err = h.service.GetRepo().UpdateDocuments(context.Background(), types.UpdateDocumentsRequest{
		Collection: "_admin_custom_queries",
		Filter:     map[string]interface{}{"internal_id": id},
		Data: map[string]interface{}{
			"name":        name,
			"collection":  collection,
			"operation":   operation,
			"query_raw":   query,
			"description": description,
		},
	})
	if err != nil {
		admin.WriteActionJSON(ctx, "", err)
		return
	}
	admin.WriteActionJSON(ctx, "Запрос обновлён", nil)
}

func (h *Handler) DeleteCustomQuery(ctx *saiTypes.RequestCtx) {
	id := string(ctx.FormValue("id"))
	if id == "" {
		admin.WriteActionJSON(ctx, "", fmt.Errorf("id обязателен"))
		return
	}
	_, err := h.service.GetRepo().DeleteDocuments(context.Background(), types.DeleteDocumentsRequest{
		Collection: "_admin_custom_queries",
		Filter:     map[string]interface{}{"internal_id": id},
	})
	if err != nil {
		admin.WriteActionJSON(ctx, "", err)
		return
	}
	admin.WriteActionJSON(ctx, "Запрос удалён", nil)
}

func (h *Handler) ClearQueryStats(ctx *saiTypes.RequestCtx) {
	if _, err := h.service.GetRepo().DeleteDocuments(context.Background(), types.DeleteDocumentsRequest{
		Collection: "_admin_query_stats",
		Filter:     map[string]interface{}{},
	}); err != nil {
		admin.WriteActionJSON(ctx, "", err)
		return
	}
	admin.WriteActionJSON(ctx, "Статистика запросов очищена", nil)
}

func (h *Handler) ClearSlowQueries(ctx *saiTypes.RequestCtx) {
	if _, err := h.service.GetRepo().DeleteDocuments(context.Background(), types.DeleteDocumentsRequest{
		Collection: "_admin_slow_queries",
		Filter:     map[string]interface{}{},
	}); err != nil {
		admin.WriteActionJSON(ctx, "", err)
		return
	}
	admin.WriteActionJSON(ctx, "Долгие запросы очищены", nil)
}

func (h *Handler) CreateIndexFromForm(ctx *saiTypes.RequestCtx) {
	collection := string(ctx.FormValue("collection"))
	keysRaw := string(ctx.FormValue("keys_raw"))
	unique := string(ctx.FormValue("unique")) == "true"
	sparse := string(ctx.FormValue("sparse")) == "true"
	name := string(ctx.FormValue("name"))

	keys := make(map[string]int)
	for _, part := range splitComma(keysRaw) {
		kv := splitColon(part)
		if len(kv) == 2 {
			dir := 1
			if kv[1] == "-1" {
				dir = -1
			}
			keys[kv[0]] = dir
		}
	}

	if collection == "" || len(keys) == 0 {
		admin.WriteActionJSON(ctx, "", fmt.Errorf("collection и keys обязательны"))
		return
	}

	req := types.CreateIndexRequest{
		Collection: collection,
		Keys:       keys,
		Unique:     unique,
		Sparse:     sparse,
		Name:       name,
	}
	if err := h.service.GetRepo().CreateIndex(context.Background(), req); err != nil {
		admin.WriteActionJSON(ctx, "", err)
		return
	}
	admin.WriteActionJSON(ctx, "Индекс успешно создан", nil)
}

func splitComma(s string) []string {
	var parts []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			parts = append(parts, t)
		}
	}
	return parts
}

func splitColon(s string) []string {
	return strings.SplitN(strings.TrimSpace(s), ":", 2)
}

func cleanArchiveFields(doc map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(doc))
	skip := map[string]bool{
		"archive_operation_id": true,
		"archive_time":         true,
		"source_collection":    true,
		"archive_filter":       true,
		"archive_update":       true,
		"restored_at":          true,
		"_id":                  true,
	}
	for k, v := range doc {
		if !skip[k] {
			result[k] = v
		}
	}
	return result
}
