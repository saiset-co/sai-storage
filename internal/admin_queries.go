package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/saiset-co/sai-service/admin"
	saiTypes "github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-storage/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const adminPerPage = 50

func pageNum(ctx *saiTypes.RequestCtx) int {
	p, _ := strconv.Atoi(string(ctx.QueryArgs().Peek("page")))
	if p < 1 {
		return 1
	}
	return p
}

func (p *AdminPanel) pageQueryStats(ctx *saiTypes.RequestCtx) (*admin.PageData, error) {
	page := pageNum(ctx)
	skip := (page - 1) * adminPerPage

	docs, total, err := p.service.GetRepo().ReadDocuments(context.Background(), types.ReadDocumentsRequest{
		Collection: "_admin_query_stats",
		Sort:       map[string]int{"count": -1},
		Limit:      adminPerPage,
		Skip:       skip,
		Count:      1,
	})
	if err != nil {
		return nil, err
	}

	var sb strings.Builder
	sb.WriteString(`<div class="overflow-x-auto"><table class="min-w-full divide-y divide-slate-200 text-sm">`)
	sb.WriteString(`<thead class="bg-slate-50"><tr>`)
	for _, h := range []string{"Коллекция", "Операция", "Вызовов", "Последний", "Поля фильтра", "Действия"} {
		sb.WriteString(fmt.Sprintf(`<th class="px-4 py-3 text-left font-medium text-slate-600">%s</th>`, h))
	}
	sb.WriteString(`</tr></thead><tbody class="divide-y divide-slate-100">`)

	for _, doc := range docs {
		collection, _ := doc["collection"].(string)
		operation, _ := doc["operation"].(string)
		count := toAnyInt64(doc["count"])
		lastSeen := formatNanoTwoLine(toAnyInt64(doc["last_seen"]))

		filterKeys := toStringSlice(doc["filter_keys"])
		sortKeys := toIntMap(doc["sort_keys"])
		keysJSON, _ := ctx.Marshal(buildIndexSpec(filterKeys, sortKeys))
		queryStr := formatQueryPreview(collection, operation, filterKeys)
		opID, _ := doc["last_operation_id"].(string)

		primaryBtn := fmt.Sprintf(
			`<button data-collection="%s" data-keys="%s" onclick="_openIdxCreate(this)" `+
				`style="display:inline-flex;align-items:center;padding:5px 12px;background:#d97706;border:none;cursor:pointer;font-size:12px;font-weight:600;color:white;border-radius:8px 0 0 8px;white-space:nowrap">Создать индексы</button>`,
			template.HTMLEscapeString(collection), template.HTMLEscapeString(string(keysJSON)),
		)
		dropdownItems := []string{
			queryViewBtn(queryStr, opID, collection+"_request_logs"),
		}

		sb.WriteString(`<tr class="hover:bg-slate-50">`)
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 font-mono">%s</td>`, template.HTMLEscapeString(collection)))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3">%s</td>`, template.HTMLEscapeString(operation)))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 font-semibold">%d</td>`, count))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 text-xs text-slate-500">%s</td>`, lastSeen))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 text-center">%d</td>`, len(filterKeys)))
		sb.WriteString(`<td class="px-4 py-3">` + sdWrap(primaryBtn, "#d97706", dropdownItems) + `</td>`)
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody></table></div>`)
	sb.WriteString(paginationBar(page, total, adminPerPage, "/admin/pages/query-stats"))

	if len(docs) == 0 {
		sb.WriteString(`<p class="text-slate-500 text-sm mt-4">Нет данных. Включите <code>track_query_stats: true</code> в конфиге.</p>`)
	}

	sb.WriteString(queryPreviewModal())
	sb.WriteString(queryPreviewScript())
	sb.WriteString(indexCreatorModal())
	sb.WriteString(indexCreatorScript())
	sb.WriteString(sdScript())

	return &admin.PageData{
		Notices: admin.ReadFlash(ctx, "/admin/pages/query-stats"),
		Sections: []admin.Section{
			{
				Title:       "Частые запросы",
				Actions:     template.HTML(clearBtn("/admin/query-stats/clear", "Очистить всё")),
				ContentHTML: template.HTML(sb.String()),
			},
		},
	}, nil
}

func (p *AdminPanel) pageSlowQueries(ctx *saiTypes.RequestCtx) (*admin.PageData, error) {
	page := pageNum(ctx)
	skip := (page - 1) * adminPerPage

	sortByTs := bson.D{{Key: "$sort", Value: bson.D{{Key: "ts", Value: 1}}}}
	groupStage := bson.D{{Key: "$group", Value: bson.D{
		{Key: "_id", Value: bson.D{
			{Key: "collection", Value: "$collection"},
			{Key: "operation", Value: "$operation"},
			{Key: "filter_fingerprint", Value: "$filter_fingerprint"},
		}},
		{Key: "max_duration_ms", Value: bson.D{{Key: "$max", Value: "$duration_ms"}}},
		{Key: "max_docs_count", Value: bson.D{{Key: "$max", Value: "$docs_count"}}},
		{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		{Key: "last_seen", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		{Key: "filter_keys", Value: bson.D{{Key: "$first", Value: "$filter_keys"}}},
		{Key: "operation_id", Value: bson.D{{Key: "$last", Value: "$operation_id"}}},
	}}}

	countDocs, _, _ := p.service.GetRepo().AggregateDocuments(context.Background(), types.AggregateDocumentsRequest{
		Collection: "_admin_slow_queries",
		Pipeline:   types.OrderedPipeline{sortByTs, groupStage, bson.D{{Key: "$count", Value: "n"}}},
	})
	var total int64
	if len(countDocs) > 0 {
		total = toAnyInt64(countDocs[0]["n"])
	}

	docs, _, err := p.service.GetRepo().AggregateDocuments(context.Background(), types.AggregateDocumentsRequest{
		Collection: "_admin_slow_queries",
		Pipeline: types.OrderedPipeline{
			sortByTs,
			groupStage,
			bson.D{{Key: "$sort", Value: bson.D{{Key: "max_duration_ms", Value: -1}}}},
			bson.D{{Key: "$skip", Value: int64(skip)}},
			bson.D{{Key: "$limit", Value: int64(adminPerPage)}},
		},
	})

	var sb strings.Builder

	if err != nil {
		sb.WriteString(`<div class="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">` + template.HTMLEscapeString(err.Error()) + `</div>`)
	}

	currentThreshold := fmt.Sprintf("%d", p.service.GetSlowQueryThreshold())
	sb.WriteString(modal("thresholdModal", "Порог медленных запросов", "thresholdForm", "thresholdErr", "thresholdBtn", "Применить", "/admin/slow-queries/threshold",
		mField("slow_ms", "Порог (мс), 0 = отключить", currentThreshold, "number"),
	))
	sb.WriteString(modalScript())

	sb.WriteString(`<div class="overflow-x-auto"><table class="min-w-full divide-y divide-slate-200 text-sm">`)
	sb.WriteString(`<thead class="bg-slate-50"><tr>`)
	for _, h := range []string{"Коллекция", "Операция", "Вызовов", "Последний", "Поля фильтра", "Макс. мс", "Макс. docs", "Действия"} {
		sb.WriteString(fmt.Sprintf(`<th class="px-4 py-3 text-left font-medium text-slate-600">%s</th>`, h))
	}
	sb.WriteString(`</tr></thead><tbody class="divide-y divide-slate-100">`)

	for _, doc := range docs {
		idMap, _ := doc["_id"].(map[string]interface{})
		collection, _ := idMap["collection"].(string)
		operation, _ := idMap["operation"].(string)
		durationMs := toAnyInt64(doc["max_duration_ms"])
		maxDocs := toAnyInt64(doc["max_docs_count"])
		count := toAnyInt64(doc["count"])
		lastSeen := formatNanoTwoLine(toAnyInt64(doc["last_seen"]))

		fKeys := toStringSlice(doc["filter_keys"])
		sKeys := toIntMap(doc["sort_keys"])
		keysJSON, _ := ctx.Marshal(buildIndexSpec(fKeys, sKeys))
		queryStr := formatQueryPreview(collection, operation, fKeys)
		opID, _ := doc["operation_id"].(string)

		primaryBtn := fmt.Sprintf(
			`<button data-collection="%s" data-keys="%s" onclick="_openIdxCreate(this)" `+
				`style="display:inline-flex;align-items:center;padding:5px 12px;background:#d97706;border:none;cursor:pointer;font-size:12px;font-weight:600;color:white;border-radius:8px 0 0 8px;white-space:nowrap">Создать индексы</button>`,
			template.HTMLEscapeString(collection), template.HTMLEscapeString(string(keysJSON)),
		)
		dropdownItems := []string{
			queryViewBtn(queryStr, opID, collection+"_request_logs"),
		}

		docsCell := slowDocsCell(maxDocs)

		sb.WriteString(`<tr class="hover:bg-slate-50">`)
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 font-mono">%s</td>`, template.HTMLEscapeString(collection)))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3">%s</td>`, template.HTMLEscapeString(operation)))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3">%d</td>`, count))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 text-xs text-slate-500">%s</td>`, lastSeen))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 text-center">%d</td>`, len(fKeys)))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 font-semibold text-rose-600">%d мс</td>`, durationMs))
		sb.WriteString(`<td class="px-4 py-3">` + docsCell + `</td>`)
		sb.WriteString(`<td class="px-4 py-3">` + sdWrap(primaryBtn, "#d97706", dropdownItems) + `</td>`)
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody></table></div>`)
	sb.WriteString(paginationBar(page, total, adminPerPage, "/admin/pages/slow-queries"))

	if len(docs) == 0 && err == nil {
		sb.WriteString(`<p class="text-slate-500 text-sm mt-4">Долгих запросов не найдено.</p>`)
	}

	sb.WriteString(queryPreviewModal())
	sb.WriteString(queryPreviewScript())
	sb.WriteString(indexCreatorModal())
	sb.WriteString(indexCreatorScript())
	sb.WriteString(sdScript())

	actions := template.HTML(
		openModalBtn("Настроить порог", "thresholdModal", "inline-flex h-9 items-center rounded-xl bg-slate-600 px-4 text-sm font-semibold text-white hover:bg-slate-500") +
			clearBtn("/admin/slow-queries/clear", "Очистить всё"),
	)

	return &admin.PageData{
		Notices: admin.ReadFlash(ctx, "/admin/pages/slow-queries"),
		Sections: []admin.Section{
			{Title: "Медленные запросы", Actions: actions, ContentHTML: template.HTML(sb.String())},
		},
	}, nil
}

func (p *AdminPanel) pageCustomQueries(ctx *saiTypes.RequestCtx) (*admin.PageData, error) {
	docs, _, err := p.service.GetRepo().ReadDocuments(context.Background(), readRequest("_admin_custom_queries", nil, map[string]int{"cr_time": -1}, 200))
	if err != nil {
		return nil, err
	}

	var sb strings.Builder
	sb.WriteString(jsSearch("Поиск по имени или запросу...", "tbl-custom-queries"))
	sb.WriteString(`<div class="overflow-x-auto"><table id="tbl-custom-queries" class="min-w-full divide-y divide-slate-200 text-sm">`)
	sb.WriteString(`<thead class="bg-slate-50"><tr>`)
	for _, h := range []string{"Имя", "Описание", "Действия"} {
		sb.WriteString(fmt.Sprintf(`<th class="px-4 py-3 text-left font-medium text-slate-600">%s</th>`, h))
	}
	sb.WriteString(`</tr></thead><tbody class="divide-y divide-slate-100">`)

	for _, doc := range docs {
		id, _ := doc["internal_id"].(string)
		name, _ := doc["name"].(string)
		collection, _ := doc["collection"].(string)
		operation, _ := doc["operation"].(string)
		description, _ := doc["description"].(string)
		queryRaw, _ := doc["query_raw"].(string)

		queryFull := formatCustomQuery(collection, operation, doc["body"], queryRaw)
		queryDisplay := queryFull
		if len([]rune(queryDisplay)) > 80 {
			queryDisplay = string([]rune(queryDisplay)[:77]) + "..."
		}
		searchVal := template.HTMLEscapeString(name + " " + collection + " " + queryFull + " " + description)

		primaryBtn := fmt.Sprintf(
			`<button data-query="%s" onclick="_runCQ(this)" `+
				`style="display:inline-flex;align-items:center;padding:5px 12px;background:#6366f1;border:none;cursor:pointer;font-size:12px;font-weight:600;color:white;border-radius:8px 0 0 8px;white-space:nowrap">Запустить</button>`,
			template.HTMLEscapeString(queryFull),
		)
		editBtn := fmt.Sprintf(
			`<button type="button" data-id="%s" data-query="%s" data-name="%s" data-desc="%s" onclick="_openCQEdit(this)" `+
				`style="display:block;width:100%%;text-align:left;padding:6px 10px;border-radius:6px;font-size:12px;font-weight:500;color:#334155;background:none;border:none;cursor:pointer;white-space:nowrap" `+
				`onmouseover="this.style.background='#f1f5f9'" onmouseout="this.style.background=''">Изменить</button>`,
			template.HTMLEscapeString(id), template.HTMLEscapeString(queryFull),
			template.HTMLEscapeString(name), template.HTMLEscapeString(description),
		)
		deleteBtn := fmt.Sprintf(
			`<button type="button" data-id="%s" onclick="_cqDelete(this)" `+
				`style="display:block;width:100%%;text-align:left;padding:6px 10px;border-radius:6px;font-size:12px;font-weight:500;color:#ef4444;background:none;border:none;cursor:pointer;white-space:nowrap" `+
				`onmouseover="this.style.background='#fef2f2'" onmouseout="this.style.background=''">Удалить</button>`,
			template.HTMLEscapeString(id),
		)
		dropdownItems := []string{
			sdBtnData("Детали", "data-q", queryFull, "_openQP(this)"),
			editBtn,
			deleteBtn,
		}

		sb.WriteString(fmt.Sprintf(`<tr class="hover:bg-slate-50" data-search="%s">`, searchVal))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 font-medium">%s</td>`, template.HTMLEscapeString(name)))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 text-sm text-slate-500">%s</td>`, template.HTMLEscapeString(description)))
		sb.WriteString(`<td class="px-4 py-3">` + sdWrap(primaryBtn, "#6366f1", dropdownItems) + `</td>`)
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody></table></div>`)

	if len(docs) == 0 {
		sb.WriteString(`<p class="text-slate-500 text-sm mt-4">Кастомных запросов нет.</p>`)
	}

	cqContent := mTextarea("query", "Запрос", "db.collection.find({})") +
		mField("name", "Имя (опционально)", "", "text") +
		mField("description", "Описание (опционально)", "", "text")

	sb.WriteString(modal("customQueryModal", "Добавить кастомный запрос", "customQueryForm", "customQueryErr", "customQueryBtn", "Сохранить", "/admin/custom-queries", cqContent))
	sb.WriteString(modalScript())
	sb.WriteString(queryPreviewModal())
	sb.WriteString(queryPreviewScript())
	sb.WriteString(customQueryEditModal())
	sb.WriteString(customQueryEditScript())
	sb.WriteString(customQueryRunModal())
	sb.WriteString(customQueryRunScript())
	sb.WriteString(cqDeleteScript())
	sb.WriteString(sdScript())

	return &admin.PageData{
		Notices: admin.ReadFlash(ctx, "/admin/pages/custom-queries"),
		Sections: []admin.Section{
			{
				Title:       "Кастомные запросы",
				Actions:     template.HTML(openModalBtn("+ Добавить запрос", "customQueryModal", "inline-flex h-9 items-center rounded-xl bg-indigo-600 px-4 text-sm font-semibold text-white hover:bg-indigo-500")),
				ContentHTML: template.HTML(sb.String()),
			},
		},
	}, nil
}

func queryViewBtn(queryStr, opID, logCollection string) string {
	return fmt.Sprintf(
		`<button type="button" data-op-id="%s" data-log-col="%s" data-q="%s" onclick="_openQPByOpID(this)" `+
			`style="display:block;width:100%%;text-align:left;padding:6px 10px;border-radius:6px;font-size:12px;font-weight:500;color:#334155;background:none;border:none;cursor:pointer;white-space:nowrap" `+
			`onmouseover="this.style.background='#f1f5f9'" onmouseout="this.style.background=''">Просмотр</button>`,
		template.HTMLEscapeString(opID),
		template.HTMLEscapeString(logCollection),
		template.HTMLEscapeString(queryStr),
	)
}

func (p *AdminPanel) handleAjaxRequestLogBody(ctx *saiTypes.RequestCtx) {
	opID := string(ctx.QueryArgs().Peek("op_id"))
	logCollection := string(ctx.QueryArgs().Peek("log_collection"))
	ctx.SetContentType("text/plain; charset=utf-8")
	if opID == "" || logCollection == "" {
		ctx.Response.SetBodyString("")
		return
	}
	docs, _, err := p.service.GetRepo().ReadDocuments(context.Background(), types.ReadDocumentsRequest{
		Collection: logCollection,
		Filter:     map[string]interface{}{"operation_id": opID},
		Limit:      1,
	})
	if err != nil || len(docs) == 0 {
		ctx.Response.SetBodyString("")
		return
	}
	doc := docs[0]
	if body, ok := doc["body"].(string); ok && body != "" {
		ctx.Response.SetBodyString(body)
		return
	}
	if b, err := ctx.Marshal(doc); err == nil {
		ctx.Response.SetBodyString(string(b))
	}
}

func formatCustomQuery(collection, operation string, body interface{}, queryRaw string) string {
	if queryRaw != "" {
		return queryRaw
	}
	bodyJSON := "{}"
	if body != nil {
		if b, err := json.Marshal(body); err == nil {
			bodyJSON = string(b)
		}
	}
	return fmt.Sprintf("db.%s.%s(%s)", collection, operation, bodyJSON)
}

type indexKeySpec struct {
	K string `json:"k"`
	D int    `json:"d"`
}

func buildIndexSpec(filterKeys []string, sortKeys map[string]int) []indexKeySpec {
	sortSet := make(map[string]bool, len(sortKeys))
	for k := range sortKeys {
		sortSet[k] = true
	}
	spec := make([]indexKeySpec, 0, len(filterKeys)+len(sortKeys))
	for _, k := range filterKeys {
		if !sortSet[k] {
			spec = append(spec, indexKeySpec{K: k, D: 1})
		}
	}
	sorted := make([]string, 0, len(sortKeys))
	for k := range sortKeys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		spec = append(spec, indexKeySpec{K: k, D: sortKeys[k]})
	}
	return spec
}

func toIntMap(v interface{}) map[string]int {
	if v == nil {
		return nil
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	result := make(map[string]int, len(m))
	for k, val := range m {
		result[k] = int(toAnyInt64(val))
	}
	return result
}

func toStringSlice(v interface{}) []string {
	var items []interface{}
	switch a := v.(type) {
	case []interface{}:
		items = a
	case primitive.A:
		items = []interface{}(a)
	default:
		return nil
	}
	out := make([]string, 0, len(items))
	for _, k := range items {
		if s, ok := k.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func formatFilterKeys(v interface{}) string {
	return strings.Join(toStringSlice(v), ", ")
}

func formatNano(ns int64) string {
	if ns == 0 {
		return "—"
	}
	return time.Unix(0, ns).Format("02.01.2006 15:04:05")
}

func formatNanoTwoLine(ns int64) string {
	if ns == 0 {
		return "—"
	}
	t := time.Unix(0, ns)
	return `<div>` + t.Format("02.01.06") + `</div><div>` + t.Format("15:04:05") + `</div>`
}

func toAnyInt64(v interface{}) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int32:
		return int64(t)
	case float64:
		return int64(t)
	case int:
		return int64(t)
	}
	return 0
}

func readRequest(collection string, filter map[string]interface{}, sortBy map[string]int, limit int) types.ReadDocumentsRequest {
	return types.ReadDocumentsRequest{
		Collection: collection,
		Filter:     filter,
		Sort:       sortBy,
		Limit:      limit,
	}
}

func (p *AdminPanel) handleRunCustomQuery(ctx *saiTypes.RequestCtx) {
	queryRaw := strings.TrimSpace(string(ctx.QueryArgs().Peek("query_raw")))
	ctx.SetContentType("text/html; charset=utf-8")

	if queryRaw == "" {
		ctx.Response.SetBodyString(`<p class="text-rose-500 text-sm">Запрос не указан</p>`)
		return
	}

	collection, operation, rawBody, err := extractQueryParts(queryRaw)
	if err != nil {
		ctx.Response.SetBodyString(`<p class="text-rose-500 text-sm">` + template.HTMLEscapeString(err.Error()) + `</p>`)
		return
	}

	opLower := strings.ToLower(operation)
	var docs []map[string]interface{}
	var total int64

	switch opLower {
	case "find", "findone":
		var filter map[string]interface{}
		if rawBody != "" {
			ctx.Unmarshal([]byte(rawBody), &filter)
		}
		limit := 100
		if opLower == "findone" {
			limit = 1
		}
		resp, e := p.service.ReadDocuments(context.Background(), types.ReadDocumentsRequest{
			Collection: collection,
			Filter:     filter,
			Limit:      limit,
			Count:      1,
		})
		docs, total, err = resp.Data, resp.Total, e
	case "aggregate":
		req := types.AggregateDocumentsRequest{Collection: collection, Count: 1}
		ctx.Unmarshal([]byte(rawBody), &req.Pipeline)
		resp, e := p.service.AggregateDocuments(context.Background(), req)
		docs, total, err = resp.Data, resp.Total, e
	case "updateone":
		arg1, arg2, parseErr := splitTwoArgs(rawBody)
		if parseErr != nil {
			ctx.Response.SetBodyString(`<p class="text-rose-500 text-sm">` + template.HTMLEscapeString(parseErr.Error()) + `</p>`)
			return
		}
		var filterMap, updateMap map[string]interface{}
		ctx.Unmarshal([]byte(arg1), &filterMap)
		ctx.Unmarshal([]byte(arg2), &updateMap)
		resp, e := p.service.UpdateDocuments(context.Background(), types.UpdateDocumentsRequest{
			Collection: collection,
			Filter:     filterMap,
			Data:       updateMap,
		})
		p.service.LogRequest(context.Background(), collection, map[string]interface{}{
			"method": "CUSTOM_QUERY", "path": "/admin/custom-queries/run",
			"query_raw": queryRaw, "request_time": time.Now().Format(time.RFC3339),
			"request_unix": time.Now().Unix(), "results": resp.Updated,
		})
		if e != nil {
			ctx.Response.SetBodyString(`<p class="text-rose-500 text-sm">` + template.HTMLEscapeString(e.Error()) + `</p>`)
			return
		}
		ctx.Response.SetBodyString(fmt.Sprintf(`<p class="text-emerald-600 text-sm font-medium">✓ Обновлено документов: %d</p>`, resp.Updated))
		return
	case "updatemany", "update":
		arg1, arg2, parseErr := splitTwoArgs(rawBody)
		if parseErr != nil {
			ctx.Response.SetBodyString(`<p class="text-rose-500 text-sm">` + template.HTMLEscapeString(parseErr.Error()) + `</p>`)
			return
		}
		var filterMap, updateMap map[string]interface{}
		ctx.Unmarshal([]byte(arg1), &filterMap)
		ctx.Unmarshal([]byte(arg2), &updateMap)
		resp, e := p.service.UpdateDocuments(context.Background(), types.UpdateDocumentsRequest{
			Collection: collection,
			Filter:     filterMap,
			Data:       updateMap,
		})
		p.service.LogRequest(context.Background(), collection, map[string]interface{}{
			"method": "CUSTOM_QUERY", "path": "/admin/custom-queries/run",
			"query_raw": queryRaw, "request_time": time.Now().Format(time.RFC3339),
			"request_unix": time.Now().Unix(), "results": resp.Updated,
		})
		if e != nil {
			ctx.Response.SetBodyString(`<p class="text-rose-500 text-sm">` + template.HTMLEscapeString(e.Error()) + `</p>`)
			return
		}
		ctx.Response.SetBodyString(fmt.Sprintf(`<p class="text-emerald-600 text-sm font-medium">✓ Обновлено документов: %d</p>`, resp.Updated))
		return
	case "deleteone":
		var filterMap map[string]interface{}
		ctx.Unmarshal([]byte(rawBody), &filterMap)
		resp, e := p.service.DeleteDocuments(context.Background(), types.DeleteDocumentsRequest{
			Collection: collection,
			Filter:     filterMap,
		})
		if e != nil {
			ctx.Response.SetBodyString(`<p class="text-rose-500 text-sm">` + template.HTMLEscapeString(e.Error()) + `</p>`)
			return
		}
		deleted := resp.Deleted
		p.service.LogRequest(context.Background(), collection, map[string]interface{}{
			"method": "CUSTOM_QUERY", "path": "/admin/custom-queries/run",
			"query_raw": queryRaw, "request_time": time.Now().Format(time.RFC3339),
			"request_unix": time.Now().Unix(), "results": deleted,
		})
		ctx.Response.SetBodyString(fmt.Sprintf(`<p class="text-rose-600 text-sm font-medium">✓ Удалено документов: %d</p>`, deleted))
		return
	case "deletemany", "delete":
		var filterMap map[string]interface{}
		ctx.Unmarshal([]byte(rawBody), &filterMap)
		resp, e := p.service.DeleteDocuments(context.Background(), types.DeleteDocumentsRequest{
			Collection: collection,
			Filter:     filterMap,
		})
		p.service.LogRequest(context.Background(), collection, map[string]interface{}{
			"method": "CUSTOM_QUERY", "path": "/admin/custom-queries/run",
			"query_raw": queryRaw, "request_time": time.Now().Format(time.RFC3339),
			"request_unix": time.Now().Unix(), "results": resp.Deleted,
		})
		if e != nil {
			ctx.Response.SetBodyString(`<p class="text-rose-500 text-sm">` + template.HTMLEscapeString(e.Error()) + `</p>`)
			return
		}
		ctx.Response.SetBodyString(fmt.Sprintf(`<p class="text-rose-600 text-sm font-medium">✓ Удалено документов: %d</p>`, resp.Deleted))
		return
	default:
		ctx.Response.SetBodyString(`<p class="text-amber-600 text-sm">Операция "<b>` + template.HTMLEscapeString(operation) + `</b>" не поддерживается. Поддерживаются: find, findOne, aggregate, updateOne, updateMany, deleteOne, deleteMany.</p>`)
		return
	}

	if err != nil {
		ctx.Response.SetBodyString(`<p class="text-rose-500 text-sm">` + template.HTMLEscapeString(err.Error()) + `</p>`)
		return
	}

	p.service.LogRequest(context.Background(), collection, map[string]interface{}{
		"method":       "CUSTOM_QUERY",
		"path":         "/admin/custom-queries/run",
		"query_raw":    queryRaw,
		"request_time": time.Now().Format(time.RFC3339),
		"request_unix": time.Now().Unix(),
		"results":      total,
	})

	if len(docs) == 0 {
		ctx.Response.SetBodyString(`<p class="text-slate-500 text-sm">Документов не найдено.</p>`)
		return
	}

	headerSet := make(map[string]struct{})
	for _, doc := range docs {
		for k := range doc {
			if k != "_id" {
				headerSet[k] = struct{}{}
			}
		}
	}
	headers := make([]string, 0, len(headerSet))
	for h := range headerSet {
		headers = append(headers, h)
	}
	sort.Strings(headers)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<p class="text-xs text-slate-500 mb-3">Найдено: %d, показано: %d</p>`, total, len(docs)))
	sb.WriteString(`<div class="overflow-x-auto"><table class="min-w-full divide-y divide-slate-200 text-xs">`)
	sb.WriteString(`<thead class="bg-slate-50"><tr>`)
	for _, h := range headers {
		sb.WriteString(`<th class="px-3 py-2 text-left font-medium text-slate-600">` + template.HTMLEscapeString(h) + `</th>`)
	}
	sb.WriteString(`</tr></thead><tbody class="divide-y divide-slate-100">`)
	for _, doc := range docs {
		sb.WriteString(`<tr class="hover:bg-slate-50">`)
		for _, h := range headers {
			v := doc[h]
			var valStr string
			if v == nil {
				valStr = ""
			} else if s, ok := v.(string); ok {
				valStr = s
			} else {
				b, _ := ctx.Marshal(v)
				valStr = string(b)
			}
			valStr = truncate(valStr, 80)
			sb.WriteString(`<td class="px-3 py-2 font-mono">` + template.HTMLEscapeString(valStr) + `</td>`)
		}
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody></table></div>`)
	ctx.Response.SetBodyString(sb.String())
}

func clearBtn(action, label string) string {
	return `<button onclick="_clearAll('` + action + `',this)" class="inline-flex h-9 items-center rounded-xl bg-rose-600 px-4 text-sm font-semibold text-white hover:bg-rose-500" style="margin-left:8px">` + label + `</button>` +
		`<script>if(!window._clearAll){window._clearAll=function(url,btn){` +
		`if(!confirm('Удалить все записи?'))return;` +
		`btn.disabled=true;` +
		`fetch(window.location.origin+url,{method:'POST',headers:{'X-Requested-With':'fetch'}})` +
		`.then(function(r){return r.json();})` +
		`.then(function(d){if(d.ok){location.reload();}else{btn.disabled=false;alert(d.error||'Ошибка');}})` +
		`.catch(function(){btn.disabled=false;alert('Ошибка сети');});}}</script>`
}

func indexCreatorModal() string {
	return `<div id="idxCreateModal" style="display:none;position:fixed;inset:0;background:rgba(15,23,42,0.5);z-index:50;align-items:center;justify-content:center;padding:16px">` +
		`<div style="background:white;border-radius:16px;width:100%;max-width:640px;max-height:90vh;overflow:hidden;display:flex;flex-direction:column;box-shadow:0 25px 50px rgba(0,0,0,0.3)">` +
		`<div style="display:flex;align-items:center;justify-content:space-between;padding:20px 24px;border-bottom:1px solid #e2e8f0;flex:0 0 auto">` +
		`<div><h2 style="font-size:18px;font-weight:700;color:#0f172a">Создать индексы</h2>` +
		`<div id="idxCreateColDisplay" style="font-size:13px;color:#64748b;font-family:monospace;margin-top:2px"></div></div>` +
		`<button onclick="document.getElementById('idxCreateModal').style.display='none'" style="width:32px;height:32px;border-radius:8px;background:#f1f5f9;border:none;cursor:pointer;font-size:18px;color:#64748b">×</button>` +
		`</div>` +
		`<div style="flex:1 1 auto;overflow-y:auto;padding:24px">` +
		`<div id="idxCardsBody"></div>` +
		`<button type="button" onclick="_idxAddCard()" style="font-size:13px;color:#6366f1;background:none;border:none;cursor:pointer;padding:0;margin-bottom:8px">+ добавить индекс</button>` +
		`<div id="idxCreateErr" style="color:#ef4444;font-size:13px;margin-top:4px"></div>` +
		`<div id="idxCreateResult" style="margin-top:8px"></div>` +
		`</div>` +
		`<div style="padding:16px 24px;border-top:1px solid #e2e8f0;display:flex;justify-content:flex-end;flex:0 0 auto">` +
		`<button type="button" id="idxCreateBtn" onclick="_idxSubmitAll()" class="inline-flex h-11 items-center rounded-xl bg-indigo-600 px-5 text-sm font-semibold text-white hover:bg-indigo-500">Создать все</button>` +
		`</div></div></div>`
}

func indexCreatorScript() string {
	return `<script>if(!window._idxCreateInit){window._idxCreateInit=true;` +
		`window._idxCollection='';` +
		`window._idxNewRow=function(fn,dir){` +
		`var d=document.createElement('div');d.className='idx-field-row flex items-center gap-2 mb-2';` +
		`d.innerHTML='<input type="text" value="'+(fn||'')+'" placeholder="поле" class="idx-field-name flex-1 h-9 rounded-lg border border-slate-300 px-3 text-sm font-mono">'+` +
		`'<select class="idx-field-dir h-9 rounded-lg border border-slate-300 px-2 text-sm">'+` +
		`'<option value="1"'+(dir===1?' selected':'')+'>1 (↑ asc)</option>'+` +
		`'<option value="-1"'+(dir===-1?' selected':'')+'>-1 (↓ desc)</option></select>'+` +
		`'<button type="button" onclick="this.closest(\'.idx-field-row\').remove()" style="width:28px;height:36px;background:none;border:none;cursor:pointer;color:#94a3b8;font-size:18px">×</button>';` +
		`return d;};` +
		`window._idxNewCard=function(){` +
		`var c=document.createElement('div');c.className='idx-card';` +
		`c.style='border:1px solid #e2e8f0;border-radius:12px;padding:16px;margin-bottom:12px';` +
		`c.innerHTML='<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px">'+` +
		`'<span style="font-size:14px;font-weight:600;color:#334155">Индекс</span>'+` +
		`'<button type="button" onclick="this.closest(\'.idx-card\').remove()" style="font-size:13px;color:#94a3b8;background:none;border:none;cursor:pointer">× удалить</button>'+` +
		`'</div><div class="idx-rows"></div>'+` +
		`'<button type="button" onclick="this.closest(\'.idx-card\').querySelector(\'.idx-rows\').appendChild(_idxNewRow(\'\',1))" '+` +
		`'style="font-size:13px;color:#6366f1;background:none;border:none;cursor:pointer;padding:0;margin-bottom:12px">+ поле</button>'+` +
		`'<div style="display:flex;align-items:center;gap:12px;padding-top:12px;border-top:1px solid #f1f5f9">'+` +
		`'<label style="display:flex;align-items:center;gap:4px;font-size:13px;color:#64748b;white-space:nowrap">'+` +
		`'<input type="checkbox" class="idx-unique"> Unique</label>'+` +
		`'<input type="text" placeholder="Название (необязательно)" class="idx-name" '+` +
		`'style="flex:1;height:32px;border-radius:8px;border:1px solid #e2e8f0;padding:0 12px;font-size:13px"></div>';` +
		`c.querySelector('.idx-rows').appendChild(_idxNewRow('',1));return c;};` +
		`window._idxAddCard=function(){document.getElementById('idxCardsBody').appendChild(_idxNewCard());};` +
		`window._openIdxCreate=function(btn){` +
		`_idxCollection=btn.getAttribute('data-collection');` +
		`var spec=[];try{spec=JSON.parse(btn.getAttribute('data-keys'));}catch(e){}` +
		`document.getElementById('idxCreateColDisplay').textContent=_idxCollection;` +
		`var body=document.getElementById('idxCardsBody');body.innerHTML='';` +
		`var card=_idxNewCard();body.appendChild(card);` +
		`if(spec.length>0){var rows=card.querySelector('.idx-rows');rows.innerHTML='';` +
		`spec.forEach(function(f){rows.appendChild(_idxNewRow(f.k,f.d));});}` +
		`document.getElementById('idxCreateErr').textContent='';` +
		`document.getElementById('idxCreateResult').innerHTML='';` +
		`document.getElementById('idxCreateModal').style.display='flex';};` +
		`window._idxSubmitAll=async function(){` +
		`var btn=document.getElementById('idxCreateBtn');` +
		`var errEl=document.getElementById('idxCreateErr');` +
		`var resultEl=document.getElementById('idxCreateResult');` +
		`var cards=document.querySelectorAll('#idxCardsBody .idx-card');` +
		`if(!cards.length){errEl.textContent='Добавьте хотя бы один индекс';return;}` +
		`btn.disabled=true;btn.textContent='Создаём...';errEl.textContent='';resultEl.innerHTML='';` +
		`var results=[];var idx=0;` +
		`for(var card of cards){idx++;` +
		`var fields={};` +
		`card.querySelectorAll('.idx-field-row').forEach(function(row){` +
		`var n=row.querySelector('.idx-field-name').value.trim();` +
		`var d=row.querySelector('.idx-field-dir').value;` +
		`if(n)fields[n]=d;});` +
		`if(!Object.keys(fields).length){results.push({i:idx,ok:false,msg:'Нет полей'});continue;}` +
		`var fd=new FormData();` +
		`fd.append('collection',_idxCollection);` +
		`fd.append('keys_raw',Object.entries(fields).map(function(e){return e[0]+':'+e[1];}).join(','));` +
		`if(card.querySelector('.idx-unique').checked)fd.append('unique','true');` +
		`var nm=card.querySelector('.idx-name').value.trim();if(nm)fd.append('name',nm);` +
		`try{var r=await fetch(window.location.origin+'/admin/indexes',{method:'POST',body:fd,headers:{'X-Requested-With':'fetch'}});` +
		`var data=await r.json();results.push({i:idx,ok:!data.error,msg:data.error||data.message||'OK'});}` +
		`catch(e){results.push({i:idx,ok:false,msg:String(e)});}}` +
		`btn.disabled=false;btn.textContent='Создать все';` +
		`resultEl.innerHTML=results.map(function(r){` +
		`return '<div style="color:'+(r.ok?'#16a34a':'#ef4444')+';font-size:13px;margin-bottom:2px">'+(r.ok?'✓':'✗')+' Индекс '+r.i+': '+r.msg+'</div>';` +
		`}).join('');};}</script>`
}

func slowDocsCell(maxDocs int64) string {
	switch {
	case maxDocs == 0:
		return `<span class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium bg-amber-50 text-amber-700" title="Мало документов — вероятно нет индекса">0 ⚠ индекс</span>`
	case maxDocs < 100:
		return fmt.Sprintf(`<span class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium bg-amber-50 text-amber-700" title="Мало документов — вероятно нет индекса">%d ⚠ индекс</span>`, maxDocs)
	case maxDocs < 1000:
		return fmt.Sprintf(`<span class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium bg-indigo-50 text-indigo-700" title="Много документов — проблема объёма данных">%d ℹ объём</span>`, maxDocs)
	default:
		return fmt.Sprintf(`<span class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium bg-rose-50 text-rose-700" title="Очень много документов — добавить пагинацию или проекцию">%d ✕ объём</span>`, maxDocs)
	}
}

func formatQueryPreview(collection, operation string, keys []string) string {
	if operation == "aggregate" {
		if len(keys) == 0 {
			return fmt.Sprintf("db.%s.aggregate([...])", collection)
		}
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = `"` + k + `": ...`
		}
		return fmt.Sprintf("db.%s.aggregate([{\"$match\": {%s}}, ...])", collection, strings.Join(parts, ", "))
	}
	if len(keys) == 0 {
		return fmt.Sprintf("db.%s.%s({})", collection, operation)
	}
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = `"` + k + `": ...`
	}
	return fmt.Sprintf("db.%s.%s({%s})", collection, operation, strings.Join(parts, ", "))
}

func queryPreviewModal() string {
	return `<div id="qpModal" style="display:none;position:fixed;inset:0;background:rgba(15,23,42,0.5);z-index:50;align-items:center;justify-content:center;padding:16px">` +
		`<div style="background:white;border-radius:16px;width:100%;max-width:700px;max-height:90vh;overflow:hidden;display:flex;flex-direction:column;box-shadow:0 25px 50px rgba(0,0,0,0.3)">` +
		`<div style="display:flex;align-items:center;justify-content:space-between;padding:20px 24px;border-bottom:1px solid #e2e8f0;flex:0 0 auto">` +
		`<h2 style="font-size:18px;font-weight:700;color:#0f172a">Запрос</h2>` +
		`<button onclick="document.getElementById('qpModal').style.display='none'" ` +
		`style="width:32px;height:32px;border-radius:8px;background:#f1f5f9;border:none;cursor:pointer;font-size:18px;color:#64748b">×</button>` +
		`</div>` +
		`<div style="flex:1 1 auto;overflow-y:auto;padding:24px">` +
		`<pre id="qpContent" style="font-size:13px;color:#1e293b;background:#f8fafc;border-radius:8px;padding:16px;overflow-x:auto;white-space:pre-wrap;word-break:break-all;margin:0"></pre>` +
		`</div></div></div>`
}

func queryPreviewScript() string {
	return `<script>if(!window._qpInit){window._qpInit=true;` +
		`window._openQP=function(btn){` +
		`document.getElementById('qpContent').textContent=btn.getAttribute('data-q');` +
		`document.getElementById('qpModal').style.display='flex';};` +
		`window._openQPByOpID=function(btn){` +
		`var opID=btn.getAttribute('data-op-id');` +
		`var logCol=btn.getAttribute('data-log-col');` +
		`var fallback=btn.getAttribute('data-q');` +
		`document.getElementById('qpContent').textContent=fallback;` +
		`document.getElementById('qpModal').style.display='flex';` +
		`if(opID&&logCol){` +
		`fetch(window.location.origin+'/admin/ajax/request-log-body?op_id='+encodeURIComponent(opID)+'&log_collection='+encodeURIComponent(logCol),` +
		`{headers:{'X-Requested-With':'fetch'}})` +
		`.then(function(r){return r.text();})` +
		`.then(function(t){if(t)document.getElementById('qpContent').textContent=t;})` +
		`.catch(function(){});}};` +
		`}</script>`
}

func splitTwoArgs(body string) (arg1, arg2 string, err error) {
	body = strings.TrimSpace(body)
	if body == "" || body[0] != '{' {
		return body, "", nil
	}
	depth := 0
	for i, ch := range body {
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				arg1 = strings.TrimSpace(body[:i+1])
				rest := strings.TrimSpace(body[i+1:])
				if len(rest) > 0 && rest[0] == ',' {
					arg2 = strings.TrimSpace(rest[1:])
				}
				return arg1, arg2, nil
			}
		}
	}
	return body, "", fmt.Errorf("несбалансированные скобки")
}

func extractQueryParts(queryRaw string) (collection, operation, rawBody string, err error) {
	if !strings.HasPrefix(queryRaw, "db.") {
		err = fmt.Errorf("запрос должен начинаться с db.")
		return
	}
	rest := queryRaw[3:]
	dotIdx := strings.Index(rest, ".")
	if dotIdx < 0 {
		err = fmt.Errorf("не найдено имя коллекции")
		return
	}
	collection = rest[:dotIdx]
	rest = rest[dotIdx+1:]
	parenIdx := strings.Index(rest, "(")
	if parenIdx < 0 {
		err = fmt.Errorf("не найдена открывающая скобка")
		return
	}
	operation = rest[:parenIdx]
	start := strings.Index(queryRaw, "(")
	end := strings.LastIndex(queryRaw, ")")
	if start >= 0 && end > start {
		rawBody = strings.TrimSpace(queryRaw[start+1 : end])
	}
	return
}

func customQueryEditModal() string {
	return `<div id="cqEditModal" style="display:none;position:fixed;inset:0;background:rgba(15,23,42,0.5);z-index:50;align-items:center;justify-content:center;padding:16px">` +
		`<div style="background:white;border-radius:16px;width:100%;max-width:560px;max-height:90vh;overflow:hidden;display:flex;flex-direction:column;box-shadow:0 25px 50px rgba(0,0,0,0.3)">` +
		`<div style="display:flex;align-items:center;justify-content:space-between;padding:24px;border-bottom:1px solid #e2e8f0;flex:0 0 auto">` +
		`<h2 style="font-size:18px;font-weight:700;color:#0f172a">Изменить запрос</h2>` +
		`<button onclick="document.getElementById('cqEditModal').style.display='none'" style="width:32px;height:32px;border-radius:8px;background:#f1f5f9;border:none;cursor:pointer;font-size:18px;color:#64748b">×</button>` +
		`</div>` +
		`<form id="cqEditForm" onsubmit="_doModal(event,'/admin/custom-queries/update','cqEditBtn','cqEditErr','cqEditModal','cqEditForm')" style="display:flex;flex:1 1 auto;min-height:0;flex-direction:column">` +
		`<div style="flex:1 1 auto;overflow-y:auto;padding:24px">` +
		`<div id="cqEditErr" style="display:none;padding:12px;border-radius:8px;background:#fef2f2;color:#ef4444;font-size:13px;margin-bottom:16px"></div>` +
		`<input type="hidden" id="cqEditId" name="id">` +
		`<div class="grid gap-4">` +
		`<div><label class="mb-2 block text-sm font-medium text-slate-700">Запрос</label>` +
		`<textarea id="cqEditQuery" name="query" rows="5" class="w-full rounded-xl border border-slate-300 bg-white px-4 py-3 text-sm text-slate-900 outline-none transition focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 font-mono resize-y"></textarea></div>` +
		`<div><label class="mb-2 block text-sm font-medium text-slate-700">Имя (опционально)</label>` +
		`<input id="cqEditName" type="text" name="name" class="h-11 w-full rounded-xl border border-slate-300 bg-white px-4 text-sm text-slate-900 outline-none transition focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100"></div>` +
		`<div><label class="mb-2 block text-sm font-medium text-slate-700">Описание (опционально)</label>` +
		`<input id="cqEditDesc" type="text" name="description" class="h-11 w-full rounded-xl border border-slate-300 bg-white px-4 text-sm text-slate-900 outline-none transition focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100"></div>` +
		`</div></div>` +
		`<div style="padding:16px 24px;border-top:1px solid #e2e8f0;display:flex;justify-content:flex-end;flex:0 0 auto">` +
		`<button type="submit" id="cqEditBtn" class="inline-flex h-11 items-center rounded-xl bg-indigo-600 px-5 text-sm font-semibold text-white hover:bg-indigo-500">Сохранить</button>` +
		`</div></form></div></div>`
}

func customQueryEditScript() string {
	return `<script>if(!window._cqEditInit){window._cqEditInit=true;` +
		`window._openCQEdit=function(btn){` +
		`document.getElementById('cqEditId').value=btn.getAttribute('data-id');` +
		`document.getElementById('cqEditQuery').value=btn.getAttribute('data-query');` +
		`document.getElementById('cqEditName').value=btn.getAttribute('data-name');` +
		`document.getElementById('cqEditDesc').value=btn.getAttribute('data-desc');` +
		`document.getElementById('cqEditErr').style.display='none';` +
		`document.getElementById('cqEditModal').style.display='flex';` +
		`};}</script>`
}

func customQueryRunModal() string {
	return `<div id="cqRunModal" style="display:none;position:fixed;inset:0;background:rgba(15,23,42,0.5);z-index:50;align-items:center;justify-content:center;padding:16px">` +
		`<div style="background:white;border-radius:16px;width:100%;max-width:1100px;max-height:90vh;overflow:hidden;display:flex;flex-direction:column;box-shadow:0 25px 50px rgba(0,0,0,0.3)">` +
		`<div style="display:flex;align-items:flex-start;justify-content:space-between;padding:20px 24px;border-bottom:1px solid #e2e8f0;flex:0 0 auto">` +
		`<div style="flex:1;min-width:0;padding-right:16px">` +
		`<h2 style="font-size:18px;font-weight:700;color:#0f172a;margin-bottom:4px">Результаты запроса</h2>` +
		`<code id="cqRunQueryText" style="font-size:11px;color:#64748b;word-break:break-all;display:block"></code>` +
		`</div>` +
		`<button onclick="document.getElementById('cqRunModal').style.display='none'" ` +
		`style="width:32px;height:32px;flex-shrink:0;border-radius:8px;background:#f1f5f9;border:none;cursor:pointer;font-size:18px;color:#64748b">×</button>` +
		`</div>` +
		`<div style="flex:1 1 auto;overflow-y:auto;padding:24px"><div id="cqRunContent" class="text-sm text-slate-500">—</div></div>` +
		`</div></div>`
}

func cqDeleteScript() string {
	return `<script>if(!window._cqDeleteInit){window._cqDeleteInit=true;` +
		`window._cqDelete=function(btn){` +
		`var id=btn.getAttribute('data-id');` +
		`if(!confirm('Удалить запрос?'))return;` +
		`var fd=new FormData();fd.append('id',id);` +
		`fetch(window.location.origin+'/admin/custom-queries/delete',{method:'POST',headers:{'X-Requested-With':'fetch'},body:fd})` +
		`.then(function(r){return r.json();})` +
		`.then(function(d){if(d.ok){location.reload();}else{alert(d.error||'Ошибка');}})` +
		`.catch(function(){alert('Ошибка сети');});};}</script>`
}

func customQueryRunScript() string {
	return `<script>if(!window._cqRunInit){window._cqRunInit=true;` +
		`window._runCQ=function(btn){` +
		`var q=btn.getAttribute('data-query');` +
		`var opM=q.match(/\.([a-zA-Z]+)\s*\(/);` +
		`var op=(opM?opM[1]:'').toLowerCase();` +
		`var destructive=['update','updateone','updatemany','delete','deleteone','deletemany'].indexOf(op)>=0;` +
		`if(destructive&&!confirm('Операция "'+( opM?opM[1]:op)+'" изменит данные. Выполнить?')){return;}` +
		`document.getElementById('cqRunModal').style.display='flex';` +
		`document.getElementById('cqRunQueryText').textContent=q;` +
		`document.getElementById('cqRunContent').innerHTML='<p class="text-slate-500 text-sm">Загрузка...</p>';` +
		`fetch(window.location.origin+'/admin/custom-queries/run?query_raw='+encodeURIComponent(q),{headers:{'X-Requested-With':'fetch'}})` +
		`.then(function(r){return r.text();})` +
		`.then(function(h){document.getElementById('cqRunContent').innerHTML=h;})` +
		`.catch(function(){document.getElementById('cqRunContent').innerHTML='<p class="text-rose-500 text-sm">Ошибка запроса</p>';});};` +
		`}</script>`
}
