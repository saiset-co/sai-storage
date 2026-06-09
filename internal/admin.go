package internal

import (
	"context"

	"github.com/saiset-co/sai-service/sai"
	"github.com/saiset-co/sai-storage/internal/handlers"
	"github.com/saiset-co/sai-storage/internal/service"
	"github.com/saiset-co/sai-storage/types"
)


type AdminPanel struct {
	service *service.StorageService
	handler *handlers.Handler
}

func SetupAdmin(storageService *service.StorageService, handler *handlers.Handler) {
	sai.InstallLogBuffer()

	panel := &AdminPanel{
		service: storageService,
		handler: handler,
	}

	_ = storageService.GetRepo().CreateIndex(context.Background(), types.CreateIndexRequest{
		Collection: "_admin_query_stats",
		Keys:       map[string]int{"collection": 1, "operation": 1, "filter_fingerprint": 1},
		Unique:     true,
		Name:       "admin_query_stats_unique",
	})
	_ = storageService.GetRepo().CreateIndex(context.Background(), types.CreateIndexRequest{
		Collection: "_admin_slow_queries",
		Keys:       map[string]int{"ts": -1},
		Name:       "admin_slow_queries_ts",
	})

	adminGroup := sai.Router().Group("/admin").WithAuthProvider("basic")
	adminGroup.GET("/archive/docs", panel.handleArchiveDocs)
	adminGroup.GET("/ajax/collection-browse", panel.handleAjaxCollectionBrowse)
	adminGroup.GET("/ajax/indexes", panel.handleAjaxIndexes)
	adminGroup.GET("/ajax/create-archive", panel.handleAjaxCreateArchive)
	adminGroup.GET("/ajax/update-archive", panel.handleAjaxUpdateArchive)
	adminGroup.GET("/ajax/delete-archive", panel.handleAjaxDeleteArchive)
	adminGroup.GET("/ajax/request-logs", panel.handleAjaxRequestLogs)
	adminGroup.GET("/ajax/request-log-body", panel.handleAjaxRequestLogBody)
	adminGroup.GET("/ajax/request-log-info", panel.handleAjaxRequestLogInfo)
	adminGroup.GET("/ajax/service-logs", panel.handleAjaxServiceLogs)
	adminGroup.GET("/custom-queries/run", panel.handleRunCustomQuery)
	adminGroup.POST("/indexes", handler.CreateIndexFromForm)
	adminGroup.POST("/restore/create", handler.RestoreCreate)
	adminGroup.POST("/restore/update", handler.RestoreUpdate)
	adminGroup.POST("/restore/delete", handler.RestoreDelete)
	adminGroup.POST("/slow-queries/threshold", handler.SetSlowQueryThreshold)
	adminGroup.POST("/slow-queries/clear", handler.ClearSlowQueries)
	adminGroup.POST("/query-stats/clear", handler.ClearQueryStats)
	adminGroup.POST("/custom-queries", handler.SaveCustomQuery)
	adminGroup.POST("/custom-queries/update", handler.UpdateCustomQuery)
	adminGroup.POST("/custom-queries/delete", handler.DeleteCustomQuery)

	sai.Admin(adminGroup).
		WithTitle("SAI Storage").
		WithSubtitle("Управление хранилищем данных").
		WithAuthProvider("basic").
		Group("База").
		WithHomePage("Коллекции", "Список коллекций и их статистика", panel.pageCollections).
		Page("indexes", "Индексы", panel.pageIndexes).
		Page("custom-queries", "Запросы", panel.pageCustomQueries).
		Group("Аналитика").
		Page("slow-queries", "Медленные", panel.pageSlowQueries).
		Page("query-stats", "Частые", panel.pageQueryStats).
		Group("Логи").
		Page("request-logs", "Запросы", panel.pageRequestLogs).
		Page("create-archive", "Создания", panel.pageCreateArchive).
		Page("update-archive", "Обновления", panel.pageUpdateArchive).
		Page("delete-archive", "Удаления", panel.pageDeleteArchive).
		Page("service-logs", "Сервис", panel.pageServiceLogs).
		Mount()
}
