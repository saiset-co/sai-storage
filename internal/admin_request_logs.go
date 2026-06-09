package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/saiset-co/sai-service/admin"
	saiTypes "github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-storage/types"
)

func (p *AdminPanel) pageRequestLogs(_ *saiTypes.RequestCtx) (*admin.PageData, error) {
	collections, err := p.service.GetRepo().ListCollectionNames(context.Background())
	if err != nil {
		return nil, err
	}

	items := make([]twoColItem, 0)
	for _, c := range collections {
		if strings.HasSuffix(c, "_request_logs") {
			items = append(items, twoColItem{
				Label: strings.TrimSuffix(c, "_request_logs"),
				URL:   "/admin/ajax/request-logs?collection=" + c,
			})
		}
	}

	scripts := twoColScript() + requestLogFilterScript() + archiveDocsScript()

	return &admin.PageData{
		Sections: []admin.Section{
			{Title: "Логи запросов", ContentHTML: template.HTML(twoColPage(items, "rlPanel", scripts))},
		},
	}, nil
}

func (p *AdminPanel) handleAjaxRequestLogs(ctx *saiTypes.RequestCtx) {
	collection := string(ctx.QueryArgs().Peek("collection"))
	selectedMethod := string(ctx.QueryArgs().Peek("method"))
	searchPath := string(ctx.QueryArgs().Peek("search"))

	ctx.SetContentType("text/html; charset=utf-8")

	if collection == "" {
		ctx.Response.SetBodyString(`<p style="font-size:13px;color:#94a3b8">Выберите коллекцию.</p>`)
		return
	}

	var sb strings.Builder

	sb.WriteString(`<form onsubmit="_rlFilter(event,'rlPanel');return false;" class="flex flex-wrap items-end gap-3 mb-6">`)
	sb.WriteString(`<input type="hidden" name="collection" value="` + template.HTMLEscapeString(collection) + `">`)

	sb.WriteString(`<div><label class="block text-sm font-medium text-slate-700 mb-1">Метод</label>`)
	sb.WriteString(`<select name="method" class="h-10 rounded-xl border border-slate-300 px-3 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500">`)
	for _, m := range []string{"", "GET", "POST", "PUT", "DELETE", "PATCH"} {
		selected := ""
		if m == selectedMethod {
			selected = " selected"
		}
		label := m
		if m == "" {
			label = "Все методы"
		}
		sb.WriteString(fmt.Sprintf(`<option value="%s"%s>%s</option>`, m, selected, label))
	}
	sb.WriteString(`</select></div>`)

	sb.WriteString(fmt.Sprintf(
		`<div><label class="block text-sm font-medium text-slate-700 mb-1">Поиск (путь, IP, пользователь...)</label>`+
			`<input type="text" name="search" value="%s" placeholder="regex по тексту..." `+
			`class="h-10 rounded-xl border border-slate-300 px-3 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500" style="min-width:220px"></div>`,
		template.HTMLEscapeString(searchPath),
	))

	sb.WriteString(`<button type="submit" class="inline-flex h-10 items-center rounded-xl bg-indigo-600 px-4 text-sm font-semibold text-white hover:bg-indigo-500">Показать</button>`)
	sb.WriteString(`</form>`)

	filter := map[string]interface{}{}
	if selectedMethod != "" {
		filter["method"] = selectedMethod
	}
	if searchPath != "" {
		rx := map[string]interface{}{"$regex": searchPath, "$options": "i"}
		filter["$or"] = []interface{}{
			map[string]interface{}{"path": rx},
			map[string]interface{}{"ip": rx},
			map[string]interface{}{"user_id": rx},
			map[string]interface{}{"collection": rx},
			map[string]interface{}{"body": rx},
		}
	}

	page := pageNum(ctx)
	skip := (page - 1) * adminPerPage

	docs, total, err := p.service.GetRepo().ReadDocuments(context.Background(), types.ReadDocumentsRequest{
		Collection: collection,
		Filter:     filter,
		Sort:       map[string]int{"request_unix": -1},
		Limit:      adminPerPage,
		Skip:       skip,
		Count:      1,
	})
	if err != nil {
		sb.WriteString(`<p class="text-rose-500 text-sm">` + template.HTMLEscapeString(err.Error()) + `</p>`)
		ctx.Response.SetBodyString(sb.String())
		return
	}

	sb.WriteString(archiveQueryModal())

	sb.WriteString(`<div class="overflow-x-auto"><table class="min-w-full divide-y divide-slate-200 text-sm">`)
	sb.WriteString(`<thead class="bg-slate-50"><tr>`)
	for _, h := range []string{"Время", "Метод", "Путь", "IP", "User ID", "Действия"} {
		sb.WriteString(fmt.Sprintf(`<th class="px-4 py-3 text-left font-medium text-slate-600">%s</th>`, h))
	}
	sb.WriteString(`</tr></thead><tbody class="divide-y divide-slate-100">`)

	for _, doc := range docs {
		ts, _ := doc["request_unix"].(float64)
		timeStr := "—"
		if ts > 0 {
			t := time.Unix(int64(ts), 0)
			timeStr = `<div>` + t.Format("02.01.06") + `</div><div>` + t.Format("15:04:05") + `</div>`
		}
		method, _ := doc["method"].(string)
		path, _ := doc["path"].(string)
		ip, _ := doc["ip"].(string)
		userID, _ := doc["user_id"].(string)
		opID, _ := doc["operation_id"].(string)
		srcCollection, _ := doc["collection"].(string)
		if srcCollection == "" {
			if bodyStr, ok := doc["body"].(string); ok && bodyStr != "" {
				var m map[string]interface{}
				if json.Unmarshal([]byte(bodyStr), &m) == nil {
					srcCollection, _ = m["collection"].(string)
				}
			}
		}

		docJSON, _ := ctx.Marshal(doc)

		arcCol := ""
		restoreAction := ""
		if opID != "" && srcCollection != "" {
			switch method {
			case "POST":
				arcCol = srcCollection + "_create_archive"
				restoreAction = "/admin/restore/create"
			case "PUT":
				arcCol = srcCollection + "_update_archive"
				restoreAction = "/admin/restore/update"
			case "DELETE":
				arcCol = srcCollection + "_delete_archive"
				restoreAction = "/admin/restore/delete"
			}
		}

		primaryBtn := fmt.Sprintf(
			`<button data-q="%s" data-op-id="%s" data-log-col="%s" data-arc-col="%s" data-src-col="%s" data-action="%s" onclick="_openArchiveDetails(this)" `+
				`style="display:inline-flex;align-items:center;padding:5px 12px;background:#6366f1;border:none;cursor:pointer;font-size:12px;font-weight:600;color:white;border-radius:8px;white-space:nowrap">Детали</button>`,
			template.HTMLEscapeString(string(docJSON)),
			template.HTMLEscapeString(opID),
			template.HTMLEscapeString(collection),
			template.HTMLEscapeString(arcCol),
			template.HTMLEscapeString(srcCollection),
			template.HTMLEscapeString(restoreAction),
		)

		sb.WriteString(`<tr class="hover:bg-slate-50">`)
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 text-xs text-slate-500 whitespace-nowrap">%s</td>`, timeStr))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3">%s</td>`, methodBadge(method)))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 font-mono text-xs">%s</td>`, template.HTMLEscapeString(path)))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 text-xs font-mono text-slate-400">%s</td>`, template.HTMLEscapeString(ip)))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 text-xs font-mono text-slate-400">%s</td>`, template.HTMLEscapeString(userID)))
		sb.WriteString(`<td class="px-4 py-3">` + primaryBtn + `</td>`)
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody></table></div>`)

	ajaxURL := fmt.Sprintf("/admin/ajax/request-logs?collection=%s&method=%s&search=%s",
		collection, selectedMethod, searchPath)
	sb.WriteString(ajaxPaginationBar(page, total, adminPerPage, ajaxURL, "rlPanel"))

	if len(docs) == 0 {
		sb.WriteString(`<p class="text-slate-500 text-sm mt-4">Логов не найдено.</p>`)
	}

	ctx.Response.SetBodyString(sb.String())
}

func methodBadge(method string) string {
	colors := map[string]string{
		"GET":    "bg-emerald-100 text-emerald-700",
		"POST":   "bg-blue-100 text-blue-700",
		"PUT":    "bg-amber-100 text-amber-700",
		"DELETE": "bg-rose-100 text-rose-700",
		"PATCH":  "bg-purple-100 text-purple-700",
	}
	cls := colors[method]
	if cls == "" {
		cls = "bg-slate-100 text-slate-600"
	}
	return fmt.Sprintf(`<span class="inline-flex items-center rounded px-2 py-0.5 text-xs font-semibold %s">%s</span>`,
		cls, template.HTMLEscapeString(method))
}


func requestLogFilterScript() string {
	return `<script>if(!window._rlFilterInit){window._rlFilterInit=true;` +
		`window._rlFilter=function(e,panelID){` +
		`e.preventDefault();` +
		`var form=e.target;` +
		`var collection=form.querySelector('[name=collection]').value;` +
		`var method=form.querySelector('[name=method]').value;` +
		`var search=form.querySelector('[name=search]').value;` +
		`var url='/admin/ajax/request-logs?collection='+encodeURIComponent(collection)` +
		`+'&method='+encodeURIComponent(method)` +
		`+'&search='+encodeURIComponent(search);` +
		`_loadPanel(url,panelID,null);};` +
		`}</script>`
}
