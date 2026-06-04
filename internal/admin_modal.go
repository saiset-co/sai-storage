package internal

import (
	"fmt"
	"html/template"
	"strings"
)

func modal(id, title, formID, errID, btnID, btnLabel, action, content string) string {
	var b strings.Builder
	b.WriteString(`<div id="` + id + `" style="display:none;position:fixed;inset:0;background:rgba(15,23,42,0.5);z-index:50;align-items:center;justify-content:center;padding:16px">`)
	b.WriteString(`<div style="background:white;border-radius:16px;width:100%;max-width:560px;max-height:90vh;overflow:hidden;display:flex;flex-direction:column;box-shadow:0 25px 50px rgba(0,0,0,0.3)">`)
	b.WriteString(`<div style="display:flex;align-items:center;justify-content:space-between;padding:24px;border-bottom:1px solid #e2e8f0;flex:0 0 auto">`)
	b.WriteString(`<h2 style="font-size:18px;font-weight:700;color:#0f172a">` + title + `</h2>`)
	b.WriteString(`<button onclick="_closeModal('` + id + `','` + formID + `','` + errID + `')" style="width:32px;height:32px;border-radius:8px;background:#f1f5f9;border:none;cursor:pointer;font-size:18px;color:#64748b">×</button>`)
	b.WriteString(`</div>`)
	b.WriteString(`<form id="` + formID + `" onsubmit="_doModal(event,'` + action + `','` + btnID + `','` + errID + `','` + id + `','` + formID + `')" style="display:flex;flex:1 1 auto;min-height:0;flex-direction:column">`)
	b.WriteString(`<div style="flex:1 1 auto;overflow-y:auto;padding:24px">`)
	b.WriteString(`<div id="` + errID + `" style="display:none;padding:12px;border-radius:8px;background:#fef2f2;color:#ef4444;font-size:13px;margin-bottom:16px"></div>`)
	b.WriteString(`<div class="grid gap-4">` + content + `</div>`)
	b.WriteString(`</div>`)
	b.WriteString(`<div style="padding:16px 24px;border-top:1px solid #e2e8f0;display:flex;justify-content:flex-end;flex:0 0 auto">`)
	b.WriteString(`<button type="submit" id="` + btnID + `" class="inline-flex h-11 items-center rounded-xl bg-indigo-600 px-5 text-sm font-semibold text-white hover:bg-indigo-500">` + btnLabel + `</button>`)
	b.WriteString(`</div></form></div></div>`)
	return b.String()
}

func modalScript() string {
	return `<script>` +
		`if(!window._modalInit){window._modalInit=true;` +
		`window._closeModal=function(id,formID,errID){` +
		`document.getElementById(id).style.display='none';` +
		`document.getElementById(formID).reset();` +
		`if(errID)document.getElementById(errID).style.display='none';};` +
		`window._doModal=function(e,action,btnID,errID,modalID,formID){` +
		`e.preventDefault();` +
		`var btn=document.getElementById(btnID),err=document.getElementById(errID);` +
		`var orig=btn.textContent;` +
		`err.style.display='none';btn.disabled=true;btn.textContent='...';` +
		`fetch(action,{method:'POST',headers:{'X-Requested-With':'fetch'},body:new FormData(e.target)})` +
		`.then(function(r){return r.json();})` +
		`.then(function(d){btn.disabled=false;btn.textContent=orig;` +
		`if(d.ok){if(modalID)_closeModal(modalID,formID,errID);location.reload();}` +
		`else{err.textContent=d.error||'Ошибка';err.style.display='block';}})` +
		`.catch(function(ex){btn.disabled=false;btn.textContent=orig;});` +
		`}}</script>`
}

func mField(name, label, value, typ string) string {
	if typ == "" {
		typ = "text"
	}
	return `<div><label class="mb-2 block text-sm font-medium text-slate-700">` + label + `</label>` +
		`<input type="` + typ + `" name="` + name + `" value="` + value + `" ` +
		`class="h-11 w-full rounded-xl border border-slate-300 bg-white px-4 text-sm text-slate-900 outline-none transition focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100"></div>`
}

func mTextarea(name, label, placeholder string) string {
	return `<div><label class="mb-2 block text-sm font-medium text-slate-700">` + label + `</label>` +
		`<textarea name="` + name + `" rows="5" placeholder="` + placeholder + `" ` +
		`class="w-full rounded-xl border border-slate-300 bg-white px-4 py-3 text-sm text-slate-900 outline-none transition focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 font-mono resize-y"></textarea></div>`
}

func mSelect(name, label string, options []string, values []string) string {
	var b strings.Builder
	b.WriteString(`<div><label class="mb-2 block text-sm font-medium text-slate-700">` + label + `</label>`)
	b.WriteString(`<select name="` + name + `" class="h-11 w-full rounded-xl border border-slate-300 bg-white px-4 text-sm text-slate-900 outline-none transition focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100">`)
	for i, label := range options {
		val := label
		if i < len(values) {
			val = values[i]
		}
		b.WriteString(`<option value="` + val + `">` + label + `</option>`)
	}
	b.WriteString(`</select></div>`)
	return b.String()
}

func openModalBtn(label, modalID, class string) string {
	return `<button onclick="document.getElementById('` + modalID + `').style.display='flex'" class="` + class + `">` + label + `</button>`
}

func autocompleteSelect(id, name, formID string, opts []string, current string) string {
	var b strings.Builder
	b.WriteString(`<div style="position:relative;display:inline-block">`)
	b.WriteString(`<input type="text" id="ac_` + id + `" value="` + current + `" placeholder="Поиск..." autocomplete="off" `)
	b.WriteString(`class="rounded-lg border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500" style="min-width:260px" `)
	b.WriteString(`oninput="_acFilter('` + id + `',this.value)" onfocus="_acShow('` + id + `')" onkeydown="if(event.key==='Escape')_acHide('` + id + `')">`)
	b.WriteString(`<input type="hidden" name="` + name + `" id="ac_val_` + id + `" value="` + current + `">`)
	b.WriteString(`<div id="ac_drop_` + id + `" style="display:none;position:absolute;top:calc(100% + 4px);left:0;min-width:260px;background:white;border:1px solid #cbd5e1;border-radius:8px;box-shadow:0 4px 16px rgba(0,0,0,0.12);max-height:260px;overflow-y:auto;z-index:100">`)
	for _, opt := range opts {
		bg := ""
		if opt == current {
			bg = "background:#eef2ff;"
		}
		b.WriteString(fmt.Sprintf(
			`<div data-val="%s" onclick="_acPick('%s','%s','%s')" `+
				`style="%spadding:8px 12px;cursor:pointer;font-size:13px;font-family:monospace" `+
				`onmouseover="this.style.background='#f8fafc'" onmouseout="this.style.background='%s'">%s</div>`,
			opt, id, opt, formID, bg, bg, opt,
		))
	}
	b.WriteString(`</div></div>`)
	return b.String()
}

func autocompleteScript() string {
	return `<script>if(!window._acInit){window._acInit=true;` +
		`window._acFilter=function(id,q){var d=document.getElementById('ac_drop_'+id);d.style.display='block';` +
		`q=q.toLowerCase();d.querySelectorAll('[data-val]').forEach(function(it){it.style.display=!q||it.dataset.val.toLowerCase().includes(q)?'':'none';});};` +
		`window._acShow=function(id){var d=document.getElementById('ac_drop_'+id);d.style.display='block';` +
		`d.querySelectorAll('[data-val]').forEach(function(it){it.style.display='';});};` +
		`window._acHide=function(id){document.getElementById('ac_drop_'+id).style.display='none';};` +
		`window._acPick=function(id,val,formID){` +
		`document.getElementById('ac_'+id).value=val;` +
		`document.getElementById('ac_val_'+id).value=val;` +
		`document.getElementById('ac_drop_'+id).style.display='none';` +
		`if(formID){var f=document.getElementById(formID);if(f)f.submit();}};` +
		`document.addEventListener('click',function(e){document.querySelectorAll('[id^="ac_drop_"]').forEach(function(d){` +
		`var inp=document.getElementById(d.id.replace('ac_drop_','ac_'));` +
		`if(inp&&!inp.contains(e.target)&&!d.contains(e.target))d.style.display='none';});});}</script>`
}

func sdScript() string {
	return `<script>if(!window._sdInit){window._sdInit=true;` +
		`window._sdToggle=function(btn){` +
		`var dd=btn.nextElementSibling;var open=dd.style.display!=='none';` +
		`document.querySelectorAll('[data-sdopen]').forEach(function(el){el.style.display='none';el.removeAttribute('data-sdopen');});` +
		`if(!open){var r=btn.getBoundingClientRect();dd.style.visibility='hidden';dd.style.display='block';` +
		`var ddH=dd.offsetHeight;dd.style.right=(window.innerWidth-r.right)+'px';` +
		`if(r.bottom+ddH+3>window.innerHeight){dd.style.bottom=(window.innerHeight-r.top+3)+'px';dd.style.top='';}` +
		`else{dd.style.top=(r.bottom+3)+'px';dd.style.bottom='';}` +
		`dd.style.visibility='';dd.setAttribute('data-sdopen','1');}` +
		`event.stopPropagation();};` +
		`document.addEventListener('click',function(){` +
		`document.querySelectorAll('[data-sdopen]').forEach(function(el){el.style.display='none';el.removeAttribute('data-sdopen');});` +
		`});}</script>`
}

func sdWrap(primaryHTML, bgHex string, items []string) string {
	if len(items) == 0 {
		return primaryHTML
	}
	var b strings.Builder
	b.WriteString(`<div style="display:inline-flex;position:relative;vertical-align:middle">`)
	b.WriteString(primaryHTML)
	b.WriteString(`<button onclick="_sdToggle(this)" style="display:inline-flex;align-items:center;padding:0 7px;background:` + bgHex + `;border:none;border-left:1px solid rgba(255,255,255,0.25);cursor:pointer;font-size:10px;color:white;border-radius:0 8px 8px 0;line-height:1">▾</button>`)
	b.WriteString(`<div onclick="this.style.display='none';this.removeAttribute('data-sdopen')" style="display:none;position:fixed;z-index:1000;background:white;border:1px solid #e2e8f0;border-radius:8px;box-shadow:0 4px 16px rgba(15,23,42,0.12);min-width:130px;padding:4px">`)
	for _, item := range items {
		b.WriteString(item)
	}
	b.WriteString(`</div></div>`)
	return b.String()
}

func sdBtn(label, onclick string) string {
	return `<button type="button" onclick="` + onclick + `" ` +
		`style="display:block;width:100%;text-align:left;padding:6px 10px;border-radius:6px;font-size:12px;font-weight:500;color:#334155;background:none;border:none;cursor:pointer;white-space:nowrap" ` +
		`onmouseover="this.style.background='#f1f5f9'" onmouseout="this.style.background=''">` + template.HTMLEscapeString(label) + `</button>`
}

func sdBtnData(label, dataAttr, dataVal, onclick string) string {
	return `<button type="button" ` + dataAttr + `="` + template.HTMLEscapeString(dataVal) + `" onclick="` + onclick + `" ` +
		`style="display:block;width:100%;text-align:left;padding:6px 10px;border-radius:6px;font-size:12px;font-weight:500;color:#334155;background:none;border:none;cursor:pointer;white-space:nowrap" ` +
		`onmouseover="this.style.background='#f1f5f9'" onmouseout="this.style.background=''">` + template.HTMLEscapeString(label) + `</button>`
}

func paginationBar(page int, total int64, perPage int, baseURL string) string {
	if total <= int64(perPage) {
		return ""
	}
	pages := int((total + int64(perPage) - 1) / int64(perPage))
	sep := "?"
	if strings.Contains(baseURL, "?") {
		sep = "&"
	}
	var b strings.Builder
	b.WriteString(`<div class="mt-4 flex items-center gap-3 text-sm">`)
	if page > 1 {
		b.WriteString(fmt.Sprintf(`<a href="%s%spage=%d" class="inline-flex h-8 items-center rounded-lg px-3 bg-white border border-slate-300 text-slate-700 hover:bg-slate-50">← Назад</a>`, baseURL, sep, page-1))
	}
	b.WriteString(fmt.Sprintf(`<span class="text-slate-500">Стр. %d / %d (всего %d)</span>`, page, pages, total))
	if page < pages {
		b.WriteString(fmt.Sprintf(`<a href="%s%spage=%d" class="inline-flex h-8 items-center rounded-lg px-3 bg-white border border-slate-300 text-slate-700 hover:bg-slate-50">Вперёд →</a>`, baseURL, sep, page+1))
	}
	b.WriteString(`</div>`)
	return b.String()
}

func jsSearch(placeholder, tableID string) string {
	return `<div class="mb-4">` +
		`<input type="text" placeholder="` + placeholder + `" ` +
		`oninput="_jsSearch('` + tableID + `',this.value)" ` +
		`class="h-9 rounded-xl border border-slate-300 px-4 text-sm w-full max-w-xs focus:outline-none focus:ring-2 focus:ring-indigo-500">` +
		`</div>` +
		`<script>if(!window._jsSearch){window._jsSearch=function(tbl,q){q=q.toLowerCase();` +
		`document.querySelectorAll('#'+tbl+' tbody tr[data-search]').forEach(function(r){` +
		`r.style.display=r.dataset.search.toLowerCase().includes(q)?'':'none';});};}</script>`
}
