package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"sort"
	"strings"

	"github.com/saiset-co/sai-service/admin"
	saiTypes "github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-storage/types"
)

func (p *AdminPanel) pageCollections(_ *saiTypes.RequestCtx) (*admin.PageData, error) {
	stats, err := p.service.GetRepo().GetAdminCollectionStats(context.Background())
	if err != nil {
		return nil, err
	}

	var sb strings.Builder

	sb.WriteString(`<div id="colBackBtn" style="display:none;margin-bottom:16px">`)
	sb.WriteString(`<button onclick="_colBack()" style="display:inline-flex;align-items:center;gap:6px;height:36px;border:1px solid #cbd5e1;border-radius:8px;background:white;color:#475569;font-size:14px;font-weight:500;padding:0 16px;cursor:pointer">← Коллекции</button>`)
	sb.WriteString(`</div>`)

	sb.WriteString(`<div id="colStatsTable"><div class="overflow-x-auto"><table class="min-w-full divide-y divide-slate-200 text-sm">`)
	sb.WriteString(`<thead class="bg-slate-50"><tr>`)
	for _, h := range []string{"Коллекция", "Документов", "Размер", "Индексов"} {
		sb.WriteString(fmt.Sprintf(`<th class="px-4 py-3 text-left font-medium text-slate-600">%s</th>`, h))
	}
	sb.WriteString(`</tr></thead><tbody class="divide-y divide-slate-100">`)
	for _, s := range stats {
		sb.WriteString(`<tr class="hover:bg-slate-50">`)
		sb.WriteString(fmt.Sprintf(
			`<td class="px-4 py-3"><button class="text-indigo-600 hover:text-indigo-800 font-semibold text-left" onclick="_colOpen('%s')">%s</button></td>`,
			template.JSEscapeString(s.Name), template.HTMLEscapeString(s.Name),
		))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3">%d</td>`, s.Count))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3">%s</td>`, formatBytes(s.StorageSize)))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3">%d</td>`, s.NumIndexes))
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody></table></div></div>`)

	sb.WriteString(`<div id="colBrowsePanel"></div>`)

	sb.WriteString(docViewModal())
	sb.WriteString(twoColScript())
	sb.WriteString(collectionBrowseScript())
	sb.WriteString(modalScript())
	sb.WriteString(docViewScript())
	sb.WriteString(`<script>if(!window._colOpenInit){window._colOpenInit=true;` +
		`window._colOpen=function(name){` +
		`document.getElementById('colStatsTable').style.display='none';` +
		`document.getElementById('colBackBtn').style.display='block';` +
		`_loadPanel('/admin/ajax/collection-browse?collection='+encodeURIComponent(name),'colBrowsePanel',null);` +
		`};` +
		`window._colBack=function(){` +
		`document.getElementById('colStatsTable').style.display='';` +
		`document.getElementById('colBackBtn').style.display='none';` +
		`document.getElementById('colBrowsePanel').innerHTML='';` +
		`};}</script>`)

	return &admin.PageData{
		Sections: []admin.Section{
			{Title: "Коллекции", ContentHTML: template.HTML(sb.String())},
		},
	}, nil
}

func (p *AdminPanel) handleAjaxCollectionBrowse(ctx *saiTypes.RequestCtx) {
	collection := string(ctx.QueryArgs().Peek("collection"))
	filterRaw := strings.TrimSpace(string(ctx.QueryArgs().Peek("query")))
	page := pageNum(ctx)

	ctx.SetContentType("text/html; charset=utf-8")

	if collection == "" {
		ctx.Response.SetBodyString(`<p style="font-size:13px;color:#94a3b8">Выберите коллекцию.</p>`)
		return
	}

	if filterRaw == "" {
		filterRaw = "{}"
	}

	var sb strings.Builder

	sb.WriteString(`<div style="display:flex;align-items:center;gap:8px;margin-bottom:12px">`)
	sb.WriteString(`<input type="hidden" id="cbCollection" value="` + template.HTMLEscapeString(collection) + `">`)
	sb.WriteString(`<input type="text" id="cbQueryInput" onkeydown="_cbKeyExec(event,'colBrowsePanel')" placeholder="{}" ` +
		`style="flex:1;font-family:monospace;font-size:13px;border:1px solid #cbd5e1;border-radius:8px;padding:0 12px;height:36px;outline:none;min-width:0" ` +
		`value="` + template.HTMLEscapeString(filterRaw) + `">`)
	sb.WriteString(`<button onclick="_cbExec('colBrowsePanel')" style="flex-shrink:0;height:36px;border:none;border-radius:8px;background:#0f172a;color:white;font-size:13px;font-weight:600;padding:0 16px;cursor:pointer">▶ Выполнить</button>`)
	sb.WriteString(`</div>`)

	var filter map[string]interface{}
	if err := json.Unmarshal([]byte(filterRaw), &filter); err != nil {
		sb.WriteString(`<p class="text-rose-500 text-sm">Неверный JSON: ` + template.HTMLEscapeString(err.Error()) + `</p>`)
		ctx.Response.SetBodyString(sb.String())
		return
	}

	skip := (page - 1) * adminPerPage
	resp, execErr := p.service.ReadDocuments(context.Background(), types.ReadDocumentsRequest{
		Collection: collection,
		Filter:     filter,
		Limit:      adminPerPage,
		Skip:       skip,
		Count:      1,
	})

	if execErr != nil {
		sb.WriteString(`<p class="text-rose-500 text-sm mt-2">` + template.HTMLEscapeString(execErr.Error()) + `</p>`)
		ctx.Response.SetBodyString(sb.String())
		return
	}

	docs, total := resp.Data, resp.Total

	sb.WriteString(fmt.Sprintf(`<div class="text-xs text-slate-400 mb-2">Всего: %d</div>`, total))

	if len(docs) == 0 {
		sb.WriteString(`<p class="text-slate-500 text-sm">Документов не найдено.</p>`)
		ctx.Response.SetBodyString(sb.String())
		return
	}

	sb.WriteString(`<div class="overflow-x-auto"><table class="min-w-full divide-y divide-slate-200 text-xs">`)
	sb.WriteString(`<thead class="bg-slate-50"><tr>`)
	for _, h := range []string{"internal_id", "cr_time", "ch_time", ""} {
		sb.WriteString(`<th class="px-3 py-2 text-left font-medium text-slate-600">` + h + `</th>`)
	}
	sb.WriteString(`</tr></thead><tbody class="divide-y divide-slate-100">`)
	for _, doc := range docs {
		docJSON, _ := json.MarshalIndent(doc, "", "  ")
		internalID, _ := doc["internal_id"].(string)
		crTime := docNano(doc, "cr_time")
		chTime := docNano(doc, "ch_time")
		sb.WriteString(`<tr class="hover:bg-slate-50">`)
		sb.WriteString(`<td class="px-3 py-2 font-mono">` + template.HTMLEscapeString(internalID) + `</td>`)
		sb.WriteString(`<td class="px-3 py-2">` + crTime + `</td>`)
		sb.WriteString(`<td class="px-3 py-2">` + chTime + `</td>`)
		sb.WriteString(fmt.Sprintf(
			`<td class="px-3 py-2"><button data-doc='%s' onclick="_docView(this.dataset.doc)" style="font-size:12px;color:#6366f1;font-weight:500;border:1px solid #e0e7ff;background:none;cursor:pointer;padding:2px 8px;border-radius:4px">Просмотр</button></td>`,
			template.HTMLEscapeString(string(docJSON)),
		))
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody></table></div>`)

	baseURL := fmt.Sprintf("/admin/ajax/collection-browse?collection=%s&query=%s",
		url.QueryEscape(collection), url.QueryEscape(filterRaw))
	sb.WriteString(ajaxPaginationBar(page, total, adminPerPage, baseURL, "colBrowsePanel"))

	ctx.Response.SetBodyString(sb.String())
}

func docNano(doc map[string]interface{}, key string) string {
	var ns int64
	switch v := doc[key].(type) {
	case int64:
		ns = v
	case float64:
		ns = int64(v)
	case int32:
		ns = int64(v)
	}
	if ns == 0 {
		return ""
	}
	return formatNano(ns)
}

func browseDocHeaders(docs []map[string]interface{}) []string {
	seen := make(map[string]struct{})
	for _, doc := range docs {
		for k := range doc {
			if k != "_id" {
				seen[k] = struct{}{}
			}
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if k == "internal_id" && i != 0 {
			keys = append([]string{"internal_id"}, append(keys[:i], keys[i+1:]...)...)
			break
		}
	}
	return keys
}

func docViewModal() string {
	return `<div id="docViewModal" style="display:none;position:fixed;inset:0;background:rgba(15,23,42,0.5);z-index:50;align-items:center;justify-content:center;padding:16px">` +
		`<div style="background:white;border-radius:16px;width:100%;max-width:800px;max-height:90vh;overflow:hidden;display:flex;flex-direction:column;box-shadow:0 25px 50px rgba(0,0,0,0.3)">` +
		`<div style="display:flex;align-items:center;justify-content:space-between;padding:20px 24px;border-bottom:1px solid #e2e8f0;flex:0 0 auto">` +
		`<h2 style="font-size:18px;font-weight:700;color:#0f172a">Документ</h2>` +
		`<button onclick="document.getElementById('docViewModal').style.display='none'" style="width:32px;height:32px;border-radius:8px;background:#f1f5f9;border:none;cursor:pointer;font-size:18px;color:#64748b">×</button>` +
		`</div>` +
		`<div style="flex:1 1 auto;overflow-y:auto;padding:24px">` +
		`<pre id="docViewContent" style="font-size:12px;font-family:monospace;white-space:pre-wrap;word-break:break-all;margin:0;color:#0f172a"></pre>` +
		`</div></div></div>`
}

func docViewScript() string {
	return `<script>if(!window._docViewInit){window._docViewInit=true;` +
		`window._docView=function(jsonStr){` +
		`try{var o=JSON.parse(jsonStr);document.getElementById('docViewContent').textContent=JSON.stringify(o,null,2);}` +
		`catch(e){document.getElementById('docViewContent').textContent=jsonStr;}` +
		`document.getElementById('docViewModal').style.display='flex';` +
		`};}</script>`
}

func collectionBrowseScript() string {
	return `<script>if(!window._cbInit){window._cbInit=true;` +
		`window._cbExec=function(panelID){` +
		`var col=document.getElementById('cbCollection');` +
		`var q=document.getElementById('cbQueryInput');` +
		`if(!col||!q)return;` +
		`var u='/admin/ajax/collection-browse?collection='+encodeURIComponent(col.value)+'&query='+encodeURIComponent(q.value);` +
		`_loadPanel(u,panelID,null);};` +
		`window._cbKeyExec=function(e,panelID){if(e.key==='Enter'){e.preventDefault();_cbExec(panelID);}};` +
		`}</script>`
}

func (p *AdminPanel) pageIndexes(_ *saiTypes.RequestCtx) (*admin.PageData, error) {
	collections, err := p.service.GetRepo().ListCollectionNames(context.Background())
	if err != nil {
		return nil, err
	}

	items := make([]twoColItem, 0, len(collections))
	for _, c := range collections {
		if !isAdminCollection(c) {
			items = append(items, twoColItem{
				Label: c,
				URL:   "/admin/ajax/indexes?collection=" + c,
			})
		}
	}

	scripts := twoColScript() + modalScript()

	return &admin.PageData{
		Sections: []admin.Section{
			{Title: "Индексы", ContentHTML: template.HTML(twoColPage(items, "idxPanel", scripts))},
		},
	}, nil
}

func (p *AdminPanel) handleAjaxIndexes(ctx *saiTypes.RequestCtx) {
	collection := string(ctx.QueryArgs().Peek("collection"))
	ctx.SetContentType("text/html; charset=utf-8")

	if collection == "" {
		ctx.Response.SetBodyString(`<p style="font-size:13px;color:#94a3b8">Выберите коллекцию.</p>`)
		return
	}

	indexes, err := p.service.GetRepo().ListIndexes(context.Background(), collection)
	if err != nil {
		ctx.Response.SetBodyString(`<p class="text-rose-500 text-sm">` + template.HTMLEscapeString(err.Error()) + `</p>`)
		return
	}

	var sb strings.Builder

	indexModalContent := `<input type="hidden" name="collection" value="` + template.HTMLEscapeString(collection) + `">` +
		`<p class="text-sm text-slate-500">Коллекция: <strong>` + template.HTMLEscapeString(collection) + `</strong></p>` +
		mField("keys_raw", "Поля (field:1,field2:-1)", "", "text") +
		mField("name", "Имя индекса (опционально)", "", "text") +
		`<div class="flex gap-6">` +
		`<label class="inline-flex items-center gap-2 text-sm text-slate-700"><input type="checkbox" name="unique" value="true" class="rounded"> Unique</label>` +
		`<label class="inline-flex items-center gap-2 text-sm text-slate-700"><input type="checkbox" name="sparse" value="true" class="rounded"> Sparse</label>` +
		`</div>`
	sb.WriteString(modal("addIndexModal", "Добавить индекс", "addIndexForm", "addIndexErr", "addIndexBtn", "Создать", "/admin/indexes", indexModalContent))

	sb.WriteString(`<div class="mb-4">`)
	sb.WriteString(openModalBtn("+ Добавить индекс", "addIndexModal", "inline-flex h-9 items-center rounded-xl bg-indigo-600 px-4 text-sm font-semibold text-white hover:bg-indigo-500"))
	sb.WriteString(`</div>`)

	sb.WriteString(`<div class="overflow-x-auto"><table class="min-w-full divide-y divide-slate-200 text-sm">`)
	sb.WriteString(`<thead class="bg-slate-50"><tr>`)
	for _, h := range []string{"Имя", "Поля", "Unique", "Sparse"} {
		sb.WriteString(fmt.Sprintf(`<th class="px-4 py-3 text-left font-medium text-slate-600">%s</th>`, h))
	}
	sb.WriteString(`</tr></thead><tbody class="divide-y divide-slate-100">`)
	for _, idx := range indexes {
		fields := formatIndexFields(idx.Fields)
		sb.WriteString(`<tr class="hover:bg-slate-50">`)
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 font-mono">%s</td>`, idx.Name))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 font-mono text-xs">%s</td>`, fields))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3">%s</td>`, boolBadge(idx.Unique)))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3">%s</td>`, boolBadge(idx.Sparse)))
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody></table></div>`)

	if len(indexes) == 0 {
		sb.WriteString(`<p class="text-slate-500 text-sm mt-4">Индексов нет.</p>`)
	}

	ctx.Response.SetBodyString(sb.String())
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func formatIndexFields(fields map[string]int) string {
	parts := make([]string, 0, len(fields))
	for k, v := range fields {
		dir := "asc"
		if v == -1 {
			dir = "desc"
		}
		parts = append(parts, fmt.Sprintf("%s:%s", k, dir))
	}
	return strings.Join(parts, ", ")
}

func boolBadge(v bool) string {
	if v {
		return `<span class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium bg-emerald-100 text-emerald-700">да</span>`
	}
	return `<span class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium bg-slate-100 text-slate-500">нет</span>`
}

func isAdminCollection(name string) bool {
	return strings.HasPrefix(name, "_admin_") ||
		strings.HasPrefix(name, "system.") ||
		strings.HasSuffix(name, "_update_archive") ||
		strings.HasSuffix(name, "_delete_archive") ||
		strings.HasSuffix(name, "_request_logs")
}
