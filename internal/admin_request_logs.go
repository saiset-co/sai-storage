package internal

import (
	"context"
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
				Label: c,
				URL:   "/admin/ajax/request-logs?collection=" + c,
			})
		}
	}

	scripts := twoColScript() + requestLogDetailScript() + requestLogFilterScript() + archiveDocsScript() + archiveFromLogScript()

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

	sb.WriteString(requestLogDetailModal())
	sb.WriteString(archiveDocsModal(""))

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

		docJSON, _ := ctx.Marshal(doc)

		var archiveBtn string
		if opID != "" && srcCollection != "" && (method == "PUT" || method == "DELETE") {
			archiveSuffix := "_update_archive"
			restoreAction := "/admin/restore/update"
			if method == "DELETE" {
				archiveSuffix = "_delete_archive"
				restoreAction = "/admin/restore/delete"
			}
			archiveBtn = fmt.Sprintf(
				` <button data-arc-col="%s" data-op-id="%s" data-src-col="%s" data-action="%s" onclick="_openArchiveFromLog(this)" `+
					`class="inline-flex items-center rounded px-2 py-1 text-xs font-medium bg-indigo-100 text-indigo-700 hover:bg-indigo-200">Архив</button>`,
				template.HTMLEscapeString(srcCollection+archiveSuffix),
				template.HTMLEscapeString(opID),
				template.HTMLEscapeString(srcCollection),
				template.HTMLEscapeString(restoreAction),
			)
		}

		sb.WriteString(`<tr class="hover:bg-slate-50">`)
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 text-xs text-slate-500 whitespace-nowrap">%s</td>`, timeStr))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3">%s</td>`, methodBadge(method)))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 font-mono text-xs">%s</td>`, template.HTMLEscapeString(path)))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 text-xs font-mono text-slate-400">%s</td>`, template.HTMLEscapeString(ip)))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 text-xs font-mono text-slate-400">%s</td>`, template.HTMLEscapeString(userID)))
		sb.WriteString(fmt.Sprintf(
			`<td class="px-4 py-3"><button data-doc="%s" onclick="_showLogDetail(this)" `+
				`class="inline-flex items-center rounded px-2 py-1 text-xs font-medium bg-slate-100 text-slate-600 hover:bg-slate-200">Детали</button>%s</td>`,
			template.HTMLEscapeString(string(docJSON)), archiveBtn,
		))
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

func requestLogDetailModal() string {
	return `<div id="rlDetailModal" style="display:none;position:fixed;inset:0;background:rgba(15,23,42,0.5);z-index:50;align-items:center;justify-content:center;padding:16px">` +
		`<div style="background:white;border-radius:16px;width:100%;max-width:760px;max-height:90vh;overflow:hidden;display:flex;flex-direction:column;box-shadow:0 25px 50px rgba(0,0,0,0.3)">` +
		`<div style="display:flex;align-items:center;justify-content:space-between;padding:20px 24px;border-bottom:1px solid #e2e8f0;flex:0 0 auto">` +
		`<h2 style="font-size:18px;font-weight:700;color:#0f172a">Детали запроса</h2>` +
		`<button onclick="document.getElementById('rlDetailModal').style.display='none'" ` +
		`style="width:32px;height:32px;border-radius:8px;background:#f1f5f9;border:none;cursor:pointer;font-size:18px;color:#64748b">×</button>` +
		`</div>` +
		`<div style="flex:1 1 auto;overflow-y:auto;padding:24px">` +
		`<pre id="rlDetailContent" style="font-size:12px;color:#1e293b;background:#f8fafc;border-radius:8px;padding:16px;overflow-x:auto;white-space:pre-wrap;word-break:break-all"></pre>` +
		`</div></div></div>`
}

func requestLogDetailScript() string {
	return `<script>if(!window._rlInit){window._rlInit=true;` +
		`window._showLogDetail=function(btn){` +
		`var s=btn.getAttribute('data-doc');` +
		`try{s=JSON.stringify(JSON.parse(s),null,2);}catch(e){}` +
		`document.getElementById('rlDetailContent').textContent=s;` +
		`document.getElementById('rlDetailModal').style.display='flex';};}</script>`
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
