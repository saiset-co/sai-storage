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
				Label: c,
				URL:   ajaxPath + "?collection=" + c,
			})
		}
	}

	scripts := twoColScript() + archiveDocsScript() + sdScript()

	return &admin.PageData{
		Sections: []admin.Section{
			{Title: title, ContentHTML: template.HTML(twoColPage(items, panelID, scripts))},
		},
	}, nil
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
	page := pageNum(ctx)
	skip := (page - 1) * adminPerPage

	groups, total, err := p.service.GetRepo().GetArchiveGroups(context.Background(), collection, skip, adminPerPage)
	if err != nil {
		ctx.Response.SetBodyString(`<p class="text-rose-500 text-sm">` + template.HTMLEscapeString(err.Error()) + `</p>`)
		return
	}

	var sb strings.Builder

	sb.WriteString(archiveDocsModal(restoreAction))
	sb.WriteString(archiveQueryModal())

	sb.WriteString(`<div class="overflow-x-auto"><table class="min-w-full divide-y divide-slate-200 text-sm">`)
	sb.WriteString(`<thead class="bg-slate-50"><tr>`)
	for _, h := range []string{"Дата", "Operation ID", "Документов", "Действия"} {
		sb.WriteString(fmt.Sprintf(`<th class="px-4 py-3 text-left font-medium text-slate-600">%s</th>`, h))
	}
	sb.WriteString(`</tr></thead><tbody class="divide-y divide-slate-100">`)

	for _, g := range groups {
		queryFull := formatArchiveShellQuery(sourceCollection, suffix, g.Filter, g.Update)

		rowStyle := ""
		if g.RestoredAt > 0 {
			rowStyle = ` style="background:#dcfce7"`
		}

		primaryBtn := fmt.Sprintf(
			`<button data-col="%s" data-opid="%s" data-srccol="%s" data-action="%s" data-count="%d" data-restored="%d" onclick="_openArchiveDocsBtn(this)" `+
				`style="display:inline-flex;align-items:center;padding:5px 12px;background:#6366f1;border:none;cursor:pointer;font-size:12px;font-weight:600;color:white;border-radius:8px 0 0 8px;white-space:nowrap">Документы</button>`,
			template.HTMLEscapeString(collection),
			template.HTMLEscapeString(g.OperationID),
			template.HTMLEscapeString(sourceCollection),
			template.HTMLEscapeString(restoreAction),
			g.Count, g.RestoredAt,
		)
		dropdownItems := []string{
			sdBtnData("Детали", "data-q", queryFull, "_openArchiveQuery(this)"),
		}

		sb.WriteString(fmt.Sprintf(`<tr class="hover:bg-slate-50"%s>`, rowStyle))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 text-xs text-slate-500 whitespace-nowrap">%s</td>`, formatNanoTwoLine(g.ArchiveTime)))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 font-mono text-xs text-slate-400">%s</td>`, g.OperationID))
		sb.WriteString(fmt.Sprintf(`<td class="px-4 py-3 font-semibold">%d</td>`, g.Count))
		sb.WriteString(`<td class="px-4 py-3">` + sdWrap(primaryBtn, "#6366f1", dropdownItems) + `</td>`)
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody></table></div>`)

	ajaxURL := baseAjaxURL + "?collection=" + collection
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
				b, _ := json.Marshal(v)
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
		`<div style="background:white;border-radius:16px;width:100%;max-width:800px;max-height:90vh;overflow:hidden;display:flex;flex-direction:column;box-shadow:0 25px 50px rgba(0,0,0,0.3)">` +
		`<div style="display:flex;align-items:center;justify-content:space-between;padding:20px 24px;border-bottom:1px solid #e2e8f0;flex:0 0 auto">` +
		`<h2 style="font-size:18px;font-weight:700;color:#0f172a">Запрос</h2>` +
		`<button onclick="document.getElementById('archiveQueryModal').style.display='none'" ` +
		`style="width:32px;height:32px;border-radius:8px;background:#f1f5f9;border:none;cursor:pointer;font-size:18px;color:#64748b">×</button>` +
		`</div>` +
		`<div style="flex:1 1 auto;overflow-y:auto;padding:24px">` +
		`<pre id="archiveQueryContent" style="font-size:13px;color:#1e293b;background:#f8fafc;border-radius:8px;padding:16px;overflow-x:auto;white-space:pre-wrap;word-break:break-all"></pre>` +
		`</div></div></div>`
}

func archiveDocsModal(restoreAction string) string {
	return `<div id="archiveDocsModal" style="display:none;position:fixed;inset:0;background:rgba(15,23,42,0.5);z-index:50;align-items:center;justify-content:center;padding:16px">` +
		`<div style="background:white;border-radius:16px;width:100%;max-width:1000px;max-height:90vh;overflow:hidden;display:flex;flex-direction:column;box-shadow:0 25px 50px rgba(0,0,0,0.3)">` +
		`<div style="display:flex;align-items:center;justify-content:space-between;padding:20px 24px;border-bottom:1px solid #e2e8f0;flex:0 0 auto">` +
		`<h2 style="font-size:18px;font-weight:700;color:#0f172a">Документы операции</h2>` +
		`<div style="display:flex;align-items:center;gap:12px">` +
		`<form id="archiveRestoreForm" method="POST" action="` + restoreAction + `" onsubmit="return _archiveRestore(event)">` +
		`<input type="hidden" id="archiveRestoreCol" name="collection">` +
		`<input type="hidden" id="archiveRestoreOpID" name="archive_operation_id">` +
		`<button id="archiveRestoreBtn" type="submit" class="inline-flex h-9 items-center rounded-xl bg-amber-500 px-4 text-sm font-semibold text-white hover:bg-amber-400">Восстановить</button>` +
		`</form>` +
		`<button onclick="document.getElementById('archiveDocsModal').style.display='none'" ` +
		`style="width:32px;height:32px;border-radius:8px;background:#f1f5f9;border:none;cursor:pointer;font-size:18px;color:#64748b">×</button>` +
		`</div></div>` +
		`<div style="flex:1 1 auto;overflow-y:auto;padding:24px"><div id="archiveDocsContent" class="text-sm text-slate-500">—</div></div>` +
		`</div></div>`
}

func archiveDocsScript() string {
	return `<script>if(!window._archiveInit){window._archiveInit=true;` +
		`window._openArchiveQuery=function(btn){` +
		`document.getElementById('archiveQueryContent').textContent=btn.getAttribute('data-q');` +
		`document.getElementById('archiveQueryModal').style.display='flex';};` +
		`window._openArchiveDocsBtn=function(btn){` +
		`_openArchiveDocs(` +
		`btn.getAttribute('data-col'),` +
		`btn.getAttribute('data-opid'),` +
		`btn.getAttribute('data-srccol'),` +
		`btn.getAttribute('data-action'),` +
		`parseInt(btn.getAttribute('data-count'),10),` +
		`parseInt(btn.getAttribute('data-restored'),10));};` +
		`window._openArchiveDocs=function(archiveCol,opID,sourceCol,action,count,restoredAt){` +
		`document.getElementById('archiveDocsModal').style.display='flex';` +
		`document.getElementById('archiveDocsContent').innerHTML='<p class="text-slate-500 text-sm">Загрузка...</p>';` +
		`document.getElementById('archiveRestoreForm').action=action;` +
		`document.getElementById('archiveRestoreCol').value=sourceCol;` +
		`document.getElementById('archiveRestoreOpID').value=opID;` +
		`var btn=document.getElementById('archiveRestoreBtn');` +
		`if(restoredAt>0){` +
		`btn.disabled=true;btn.textContent='Уже восстановлено';btn.className=btn.className.replace('bg-amber-500','bg-slate-300').replace('hover:bg-amber-400','');` +
		`}else{` +
		`btn.disabled=false;btn.textContent='Восстановить '+count+' документов';btn.className='inline-flex h-9 items-center rounded-xl bg-amber-500 px-4 text-sm font-semibold text-white hover:bg-amber-400';}` +
		`fetch('/admin/archive/docs?collection='+encodeURIComponent(archiveCol)+'&operation_id='+encodeURIComponent(opID),` +
		`{headers:{'X-Requested-With':'fetch'}})` +
		`.then(function(r){return r.text();})` +
		`.then(function(h){document.getElementById('archiveDocsContent').innerHTML=h;})` +
		`.catch(function(){document.getElementById('archiveDocsContent').innerHTML='<p class="text-rose-500 text-sm">Ошибка загрузки</p>';});};` +
		`window._archiveRestore=function(e){e.preventDefault();` +
		`if(!confirm('Восстановить все документы операции?'))return false;` +
		`var form=document.getElementById('archiveRestoreForm');` +
		`var btn=document.getElementById('archiveRestoreBtn');` +
		`btn.disabled=true;btn.textContent='...';` +
		`fetch(form.action,{method:'POST',body:new FormData(form),headers:{'X-Requested-With':'fetch'}})` +
		`.then(function(r){return r.json();})` +
		`.then(function(d){btn.disabled=false;` +
		`if(d.ok){document.getElementById('archiveDocsModal').style.display='none';location.reload();}` +
		`else{btn.disabled=false;btn.textContent='Восстановить';alert(d.error||'Ошибка');}})` +
		`.catch(function(){btn.disabled=false;btn.textContent='Восстановить';alert('Ошибка сети');});` +
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

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-3]) + "..."
}
