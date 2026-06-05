package internal

import (
	"fmt"
	"html/template"
	"strings"
)

type twoColItem struct {
	Label string
	URL   string
}

func twoColPage(items []twoColItem, panelID, scripts string) string {
	var sidebar strings.Builder
	for _, item := range items {
		sidebar.WriteString(fmt.Sprintf(
			`<button class="tc-item" style="display:block;width:100%%;text-align:left;padding:8px 12px;font-size:13px;font-family:monospace;border:none;border-radius:8px;cursor:pointer;background:transparent;color:#475569;word-break:break-all;margin-bottom:2px" `+
				`data-url="%s" data-panel="%s" onclick="_loadPanel('%s','%s',this)">%s</button>`,
			template.HTMLEscapeString(item.URL), panelID,
			template.JSEscapeString(item.URL), panelID,
			template.HTMLEscapeString(item.Label),
		))
	}

	emptyMsg := ""
	if len(items) == 0 {
		emptyMsg = `<p style="padding:12px;font-size:13px;color:#94a3b8">Нет данных</p>`
	}

	initScript := fmt.Sprintf(
		`<script>document.addEventListener('DOMContentLoaded',function(){`+
			`var lu=sessionStorage.getItem('tc_%s');`+
			`var btn=lu?document.querySelector('.tc-item[data-url="'+lu+'"]'):null;`+
			`if(!btn)btn=document.querySelector('.tc-item');`+
			`if(btn)_loadPanel(btn.dataset.url,'%s',btn);});</script>`,
		panelID, panelID,
	)

	return `<div style="display:flex;gap:0;min-height:400px;border:1px solid #e2e8f0;border-radius:12px;overflow:hidden">` +
		`<div style="width:220px;flex:0 0 220px;border-right:1px solid #e2e8f0;overflow-y:auto;padding:8px;background:#f8fafc">` +
		emptyMsg + sidebar.String() +
		`</div>` +
		`<div id="` + panelID + `" style="flex:1;min-width:0;padding:20px;overflow-x:auto">` +
		`<p style="font-size:13px;color:#94a3b8">Выберите коллекцию.</p>` +
		`</div></div>` +
		scripts + initScript
}

func twoColScript() string {
	return `<script>if(!window._tcInit){window._tcInit=true;` +
		`window._loadPanel=function(url,panelID,btn){` +
		`sessionStorage.setItem('tc_'+panelID,url);` +
		`if(btn){document.querySelectorAll('.tc-item').forEach(function(b){` +
		`b.style.background='transparent';b.style.color='#475569';b.style.fontWeight='normal';});` +
		`btn.style.background='#eef2ff';btn.style.color='#4338ca';btn.style.fontWeight='600';}` +
		`var panel=document.getElementById(panelID);` +
		`panel.innerHTML='<p style="font-size:13px;color:#94a3b8">Загрузка...</p>';` +
		`fetch(window.location.origin+url,{headers:{'X-Requested-With':'fetch'}})` +
		`.then(function(r){return r.text();})` +
		`.then(function(h){panel.innerHTML=h;})` +
		`.catch(function(){panel.innerHTML='<p style="font-size:13px;color:#ef4444">Ошибка загрузки</p>';});};` +
		`}</script>`
}

func ajaxPaginationBar(page int, total int64, perPage int, baseURL, panelID string) string {
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
		url := fmt.Sprintf("%s%spage=%d", baseURL, sep, page-1)
		b.WriteString(fmt.Sprintf(
			`<button onclick="_loadPanel('%s','%s',null)" class="inline-flex h-8 items-center rounded-lg px-3 bg-white border border-slate-300 text-slate-700 hover:bg-slate-50">← Назад</button>`,
			template.JSEscapeString(url), panelID,
		))
	}
	b.WriteString(fmt.Sprintf(`<span class="text-slate-500">Стр. %d / %d (всего %d)</span>`, page, pages, total))
	if page < pages {
		url := fmt.Sprintf("%s%spage=%d", baseURL, sep, page+1)
		b.WriteString(fmt.Sprintf(
			`<button onclick="_loadPanel('%s','%s',null)" class="inline-flex h-8 items-center rounded-lg px-3 bg-white border border-slate-300 text-slate-700 hover:bg-slate-50">Вперёд →</button>`,
			template.JSEscapeString(url), panelID,
		))
	}
	b.WriteString(`</div>`)
	return b.String()
}
