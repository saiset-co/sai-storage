package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strings"

	"github.com/saiset-co/sai-service/admin"
	saiTypes "github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-storage/types"
)

func (p *AdminPanel) pageCreateArchive(_ *saiTypes.RequestCtx) (*admin.PageData, error) {
	return p.buildArchivePage("create_archive", "Логи созданий", "crtArcPanel")
}

func (p *AdminPanel) pageUpdateArchive(_ *saiTypes.RequestCtx) (*admin.PageData, error) {
	return p.buildArchivePage("update_archive", "Логи обновлений", "updArcPanel")
}

func (p *AdminPanel) pageDeleteArchive(_ *saiTypes.RequestCtx) (*admin.PageData, error) {
	return p.buildArchivePage("delete_archive", "Логи удалений", "delArcPanel")
}

func (p *AdminPanel) buildArchivePage(suffix, title, panelID string) (*admin.PageData, error) {
	collections, err := p.service.GetRepo().ListCollectionNames(context.Background())
	if err != nil {
		return nil, err
	}

	items := make([]twoColItem, 0)
	for _, c := range collections {
		if strings.HasSuffix(c, "_"+suffix) {
			ajaxPath := "/admin/ajax/" + strings.ReplaceAll(suffix, "_", "-")
			items = append(items, twoColItem{
				Label: strings.TrimSuffix(c, "_"+suffix),
				URL:   ajaxPath + "?collection=" + c,
			})
		}
	}

	scripts := twoColScript() + archiveDocsScript()

	return &admin.PageData{
		Sections: []admin.Section{
			{Title: title, ContentHTML: template.HTML(twoColPage(items, panelID, scripts))},
		},
	}, nil
}

func (p *AdminPanel) handleAjaxCreateArchive(ctx *saiTypes.RequestCtx) {
	p.buildAjaxArchiveContent(ctx, "create_archive", "/admin/restore/create", "/admin/ajax/create-archive", "crtArcPanel")
}

func (p *AdminPanel) handleAjaxUpdateArchive(ctx *saiTypes.RequestCtx) {
	p.buildAjaxArchiveContent(ctx, "update_archive", "/admin/restore/update", "/admin/ajax/update-archive", "updArcPanel")
}

func (p *AdminPanel) handleAjaxDeleteArchive(ctx *saiTypes.RequestCtx) {
	p.buildAjaxArchiveContent(ctx, "delete_archive", "/admin/restore/delete", "/admin/ajax/delete-archive", "delArcPanel")
}

func (p *AdminPanel) buildAjaxArchiveContent(ctx *saiTypes.RequestCtx, suffix, restoreAction, baseAjaxURL, panelID string) {
	collection := string(ctx.QueryArgs().Peek("collection"))
	ctx.SetContentType("text/html; charset=utf-8")

	if collection == "" {
		ctx.Response.SetBodyString(`<p style="font-size:13px;color:#94a3b8">Выберите коллекцию.</p>`)
		return
	}

	sourceCollection := strings.TrimSuffix(collection, "_"+suffix)
	search := string(ctx.QueryArgs().Peek("search"))
	page := pageNum(ctx)
	skip := (page - 1) * adminPerPage

	groups, total, err := p.service.GetRepo().GetArchiveGroups(context.Background(), collection, search, skip, adminPerPage)
	if err != nil {
		ctx.Response.SetBodyString(`<p class="text-rose-500 text-sm">` + template.HTMLEscapeString(err.Error()) + `</p>`)
		return
	}

	var sb strings.Builder

	sb.WriteString(archiveSearchForm(collection, search, baseAjaxURL, panelID))
	sb.WriteString(archiveQueryModal())

	sb.WriteString(`<div class="overflow-x-auto"><table class="min-w-full divide-y divide-slate-200 text-sm">`)
	sb.WriteString(`<thead class="bg-slate-50"><tr>`)
	for _, h := range []string{"Дата", "Operation ID", "Документов", "Действия"} {
		sb.WriteString(fmt.Sprintf(`<th class="px-4 py-3 text-left font-medium text-slate-600">%s</th>`, h))
	}
	sb.WriteString(`</tr></thead><tbody class="divide-y divide-slate-100">`)

	for _, g := range groups {
		queryFull := formatArchiveShellQuery(sourceCollection, "", g.Filter, g.Update)
		logCollection := sourceCollection + "_request_logs"

		rowStyle := ""
		if g.RestoredAt > 0 {
			rowStyle = ` style="background:#dcfce7"`
		}

		primaryBtn := fmt.Sprintf(
			`<button data-q="%s" data-op-id="%s" data-log-col="%s" data-arc-col="%s" data-src-col="%s" data-action="%s" data-count="%d" data-restored="%d" onclick="_openArchiveDetails(this)" `+
				`style="display:inline-flex;align-items:center;padding:5px 12px;background:#6366f1;border:none;cursor:pointer;font-size:12px;font-weight:600;color:white;border-radius:8px;white-space:nowrap">Детали</button>`,
			template.HTMLEscapeString(queryFull),
			template.HTMLEscapeString(g.OperationID),
			template.HTMLEscapeString(logCollection),
			template.HTMLEscapeString(collection),
			template.HTMLEscapeString(sourceCollection),
			template.HTMLEscapeString(restoreAction),
			g.Count, g.RestoredAt,
		)

		sb.WriteString(fmt.Sprintf(`<tr class="hover:bg-slate-50"%s>`, rowStyle))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 text-xs text-slate-500 whitespace-nowrap">%s</td>`, formatNanoTwoLine(g.ArchiveTime)))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 font-mono text-xs text-slate-400">%s</td>`, g.OperationID))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 font-semibold">%d</td>`, g.Count))
		sb.WriteString(`<td class="px-4 py-3">` + primaryBtn + `</td>`)
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody></table></div>`)

	ajaxURL := baseAjaxURL + "?collection=" + collection
	if search != "" {
		ajaxURL += "&search=" + search
	}
	sb.WriteString(ajaxPaginationBar(page, total, adminPerPage, ajaxURL, panelID))

	if len(groups) == 0 {
		sb.WriteString(`<p class="text-slate-500 text-sm mt-4">Записей в архиве нет.</p>`)
	}

	ctx.Response.SetBodyString(sb.String())
}

func (p *AdminPanel) handleArchiveDocs(ctx *saiTypes.RequestCtx) {
	archiveCollection := string(ctx.QueryArgs().Peek("collection"))
	opID := string(ctx.QueryArgs().Peek("operation_id"))

	if archiveCollection == "" || opID == "" {
		ctx.SetStatusCode(400)
		return
	}

	docs, _, err := p.service.GetRepo().ReadDocuments(context.Background(), types.ReadDocumentsRequest{
		Collection: archiveCollection,
		Filter:     map[string]interface{}{"archive_operation_id": opID},
		Sort:       map[string]int{"cr_time": 1},
		Limit:      500,
	})

	ctx.SetContentType("text/html; charset=utf-8")

	var sb strings.Builder
	if err != nil {
		sb.WriteString(`<p class="text-rose-500 text-sm">` + template.HTMLEscapeString(err.Error()) + `</p>`)
		ctx.Response.SetBodyString(sb.String())
		return
	}
	if len(docs) == 0 {
		sb.WriteString(`<p class="text-slate-500 text-sm">Документов не найдено.</p>`)
		ctx.Response.SetBodyString(sb.String())
		return
	}

	metaSkip := map[string]bool{
		"_id": true, "archive_operation_id": true, "archive_time": true,
		"source_collection": true, "archive_filter": true, "archive_update": true,
	}

	headerSet := make(map[string]struct{})
	for _, doc := range docs {
		for k := range doc {
			if !metaSkip[k] {
				headerSet[k] = struct{}{}
			}
		}
	}
	headers := make([]string, 0, len(headerSet))
	for h := range headerSet {
		headers = append(headers, h)
	}
	sort.Strings(headers)

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
			valStr = truncate(valStr, 60)
			sb.WriteString(`<td class="px-3 py-2 font-mono">` + template.HTMLEscapeString(valStr) + `</td>`)
		}
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody></table></div>`)

	ctx.Response.SetBodyString(sb.String())
}

func archiveQueryModal() string {
	return `<div id="archiveQueryModal" style="display:none;position:fixed;inset:0;background:rgba(15,23,42,0.5);z-index:50;align-items:center;justify-content:center;padding:16px">` +
		`<div style="background:white;border-radius:16px;width:100%;max-width:900px;max-height:90vh;overflow:hidden;display:flex;flex-direction:column;box-shadow:0 25px 50px rgba(0,0,0,0.3)">` +
		`<div style="display:flex;align-items:center;justify-content:space-between;padding:20px 24px;border-bottom:1px solid #e2e8f0;flex:0 0 auto">` +
		`<h2 style="font-size:18px;font-weight:700;color:#0f172a">Детали операции</h2>` +
		`<div style="display:flex;align-items:center;gap:12px">` +
		`<form id="archiveRestoreForm" style="display:none" method="POST" onsubmit="return _archiveRestore(event)">` +
		`<input type="hidden" id="archiveRestoreCol" name="collection">` +
		`<input type="hidden" id="archiveRestoreOpID" name="archive_operation_id">` +
		`<button id="archiveRestoreBtn" type="submit" class="inline-flex h-9 items-center rounded-xl bg-amber-500 px-4 text-sm font-semibold text-white hover:bg-amber-400">Восстановить</button>` +
		`</form>` +
		`<button onclick="document.getElementById('archiveQueryModal').style.display='none'" ` +
		`style="width:32px;height:32px;border-radius:8px;background:#f1f5f9;border:none;cursor:pointer;font-size:18px;color:#64748b">×</button>` +
		`</div></div>` +
		`<div style="flex:1 1 auto;overflow-y:auto;padding:24px">` +
		`<div id="archiveQueryLogInfo" style="display:none"></div>` +
		`<div id="archiveQueryDocsSection" style="display:none;margin-top:16px">` +
		`<div style="font-size:14px;font-weight:600;color:#334155;margin-bottom:8px;padding-bottom:8px;border-bottom:1px solid #e2e8f0">Документы операции</div>` +
		`<div id="archiveDocsContent" class="text-sm text-slate-500">—</div>` +
		`</div>` +
		`<pre id="archiveQueryContent" style="font-size:13px;color:#1e293b;background:#f8fafc;border-radius:8px;padding:16px;overflow-x:auto;white-space:pre-wrap;word-break:break-all;display:none"></pre>` +
		`</div></div></div>`
}


func archiveDocsScript() string {
	return `<script>if(!window._archiveInit){window._archiveInit=true;` +
		`window._openArchiveDetails=function(btn){` +
		`var q=btn.getAttribute('data-q')||'';` +
		`var opID=btn.getAttribute('data-op-id')||'';` +
		`var logCol=btn.getAttribute('data-log-col')||'';` +
		`var arcCol=btn.getAttribute('data-arc-col')||'';` +
		`var srcCol=btn.getAttribute('data-src-col')||'';` +
		`var action=btn.getAttribute('data-action')||'';` +
		`var count=parseInt(btn.getAttribute('data-count')||'0',10);` +
		`var restored=parseInt(btn.getAttribute('data-restored')||'0',10);` +
		`var c=document.getElementById('archiveQueryContent');` +
		`c.style.display='none';c.textContent=q;` +
		`var infoDiv=document.getElementById('archiveQueryLogInfo');` +
		`infoDiv.innerHTML='';infoDiv.style.display='none';` +
		`var docsSection=document.getElementById('archiveQueryDocsSection');` +
		`docsSection.style.display='none';` +
		`document.getElementById('archiveDocsContent').innerHTML='—';` +
		`var restoreForm=document.getElementById('archiveRestoreForm');` +
		`if(arcCol&&srcCol&&action&&opID){` +
		`restoreForm.style.display='block';` +
		`restoreForm.setAttribute('action',action);` +
		`document.getElementById('archiveRestoreCol').value=srcCol;` +
		`document.getElementById('archiveRestoreOpID').value=opID;` +
		`var rb=document.getElementById('archiveRestoreBtn');` +
		`if(restored>0){rb.disabled=true;rb.textContent='Уже восстановлено';rb.className='inline-flex h-9 items-center rounded-xl bg-slate-300 px-4 text-sm font-semibold text-white';}` +
		`else{rb.disabled=false;rb.textContent=count>0?'Восстановить '+count+' документов':'Восстановить';rb.className='inline-flex h-9 items-center rounded-xl bg-amber-500 px-4 text-sm font-semibold text-white hover:bg-amber-400';}` +
		`}else{restoreForm.style.display='none';}` +
		`document.getElementById('archiveQueryModal').style.display='flex';` +
		`if(opID&&logCol){` +
		`fetch(window.location.origin+'/admin/ajax/request-log-info?op_id='+encodeURIComponent(opID)+'&log_collection='+encodeURIComponent(logCol),{headers:{'X-Requested-With':'fetch'}})` +
		`.then(function(r){return r.text();})` +
		`.then(function(h){if(h){infoDiv.innerHTML=h;infoDiv.style.display='block';}else{c.style.display='block';}})` +
		`.catch(function(){c.style.display='block';});}` +
		`if(arcCol&&opID){` +
		`docsSection.style.display='block';` +
		`var dc=document.getElementById('archiveDocsContent');` +
		`dc.innerHTML='<p class="text-slate-500 text-sm">Загрузка документов...</p>';` +
		`fetch(window.location.origin+'/admin/archive/docs?collection='+encodeURIComponent(arcCol)+'&operation_id='+encodeURIComponent(opID),{headers:{'X-Requested-With':'fetch'}})` +
		`.then(function(r){return r.text();})` +
		`.then(function(h){dc.innerHTML=h;})` +
		`.catch(function(){dc.innerHTML='<p class="text-rose-500 text-sm">Ошибка загрузки</p>';});}};` +
		`window._archiveRestore=function(e){e.preventDefault();` +
		`if(!confirm('Восстановить все документы операции?'))return false;` +
		`var form=document.getElementById('archiveRestoreForm');` +
		`var btn=document.getElementById('archiveRestoreBtn');` +
		`btn.disabled=true;btn.textContent='...';` +
		`fetch(window.location.origin+form.getAttribute('action'),{method:'POST',body:new FormData(form),headers:{'X-Requested-With':'fetch'}})` +
		`.then(function(r){return r.json();})` +
		`.then(function(d){btn.disabled=false;` +
		`if(d.ok){document.getElementById('archiveQueryModal').style.display='none';location.reload();}` +
		`else{btn.textContent='Восстановить';alert(d.error||'Ошибка');}})` +
		`.catch(function(){btn.textContent='Восстановить';alert('Ошибка сети');});` +
		`return false;};}</script>`
}

func formatArchiveShellQuery(sourceCollection, op string, filter, update interface{}) string {
	filterStr := "{}"
	if filter != nil {
		if b, err := json.Marshal(filter); err == nil {
			filterStr = string(b)
		}
	}
	if update != nil {
		updateStr := "{}"
		if b, err := json.Marshal(update); err == nil {
			updateStr = string(b)
		}
		if op == "" {
			op = "updateMany"
		}
		return fmt.Sprintf("db.%s.%s(%s, %s)", sourceCollection, op, filterStr, updateStr)
	}
	if op == "" {
		op = "deleteMany"
	}
	return fmt.Sprintf("db.%s.%s(%s)", sourceCollection, op, filterStr)
}

func archiveSearchForm(collection, search, baseAjaxURL, panelID string) string {
	return fmt.Sprintf(
		`<form onsubmit="_loadPanel('%s?collection=%s&search='+encodeURIComponent(this.search.value),'%s',null);return false" `+
			`class="flex items-center gap-2 mb-4">`+
			`<input name="search" value="%s" placeholder="Поиск по Operation ID, internal_id..." `+
			`class="h-9 flex-1 rounded-xl border border-slate-300 px-3 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500">`+
			`<button type="submit" class="inline-flex h-9 items-center rounded-xl bg-indigo-600 px-4 text-sm font-semibold text-white hover:bg-indigo-500">Найти</button>`+
			`</form>`,
		baseAjaxURL, template.HTMLEscapeString(collection),
		panelID,
		template.HTMLEscapeString(search),
	)
}


func archiveRLCell(label, value string) string {
	return `<div><span style="color:#64748b">` + label + `:</span> <span style="color:#1e293b;font-weight:500">` + template.HTMLEscapeString(value) + `</span></div>`
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-3]) + "..."
}
