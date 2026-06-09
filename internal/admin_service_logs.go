package internal

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/saiset-co/sai-service/admin"
	"github.com/saiset-co/sai-service/sai"
	saiTypes "github.com/saiset-co/sai-service/types"
	"go.uber.org/zap/zapcore"
)

func (p *AdminPanel) pageServiceLogs(_ *saiTypes.RequestCtx) (*admin.PageData, error) {
	html := `<div style="display:flex;align-items:center;gap:12px;margin-bottom:12px">` +
		`<button onclick="_svcLogsRefresh()" class="inline-flex h-9 items-center rounded-xl bg-indigo-600 px-4 text-sm font-semibold text-white hover:bg-indigo-500">Обновить</button>` +
		`<span id="svcLogStatus" style="font-size:12px;color:#94a3b8"></span>` +
		`</div>` +
		`<div id="svcLogPanel" style="font-family:monospace;font-size:12px;line-height:1.7;background:#0f172a;border-radius:10px;padding:16px;min-height:200px;max-height:72vh;overflow-y:auto">` +
		`<span style="color:#64748b">Загрузка...</span>` +
		`</div>` +
		`<script>if(!window._svcLogInit){window._svcLogInit=true;` +
		`window._svcLogsRefresh=function(){` +
		`fetch(window.location.origin+'/admin/ajax/service-logs',{headers:{'X-Requested-With':'fetch'}})` +
		`.then(function(r){return r.text();})` +
		`.then(function(h){` +
		`document.getElementById('svcLogPanel').innerHTML=h;` +
		`document.getElementById('svcLogStatus').textContent='Обновлено: '+new Date().toLocaleTimeString();` +
		`}).catch(function(){` +
		`document.getElementById('svcLogStatus').textContent='Ошибка загрузки';` +
		`});};` +
		`_svcLogsRefresh();` +
		`setInterval(_svcLogsRefresh,60000);` +
		`}</script>`

	return &admin.PageData{
		Sections: []admin.Section{
			{Title: "Логи сервиса", ContentHTML: template.HTML(html)},
		},
	}, nil
}

func (p *AdminPanel) handleAjaxServiceLogs(ctx *saiTypes.RequestCtx) {
	ctx.SetContentType("text/html; charset=utf-8")

	buf := sai.LogBuffer()
	if buf == nil {
		ctx.Response.SetBodyString(`<span style="color:#64748b">Буфер не инициализирован</span>`)
		return
	}

	entries := buf.GetAll()
	if len(entries) == 0 {
		ctx.Response.SetBodyString(`<span style="color:#64748b">Нет записей</span>`)
		return
	}

	var sb strings.Builder
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		color := "#94a3b8"
		switch e.Level {
		case zapcore.ErrorLevel, zapcore.FatalLevel, zapcore.PanicLevel:
			color = "#f87171"
		case zapcore.WarnLevel:
			color = "#fbbf24"
		case zapcore.InfoLevel:
			color = "#e2e8f0"
		}
		sb.WriteString(fmt.Sprintf(
			`<div style="color:%s;padding:1px 0;border-bottom:1px solid #1e293b">%s</div>`,
			color,
			template.HTMLEscapeString(e.Text),
		))
	}
	ctx.Response.SetBodyString(sb.String())
}
