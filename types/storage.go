package types

import (
	"context"
)

type StorageRepository interface {
	CreateDocuments(ctx context.Context, request CreateDocumentsRequest) ([]string, error)
	ReadDocuments(ctx context.Context, request ReadDocumentsRequest) ([]map[string]interface{}, int64, error)
	AggregateDocuments(ctx context.Context, request AggregateDocumentsRequest) ([]map[string]interface{}, int64, error)
	UpdateDocuments(ctx context.Context, request UpdateDocumentsRequest) (int64, error)
	DeleteDocuments(ctx context.Context, request DeleteDocumentsRequest) (int64, error)
	Close(ctx context.Context) error

	GetAdminCollectionStats(ctx context.Context) ([]CollectionStats, error)
	ListCollectionNames(ctx context.Context) ([]string, error)
	ListIndexes(ctx context.Context, collection string) ([]IndexInfo, error)
	CreateIndex(ctx context.Context, req CreateIndexRequest) error
	GetSlowQueries(ctx context.Context, limit int) ([]SlowQuery, error)
	LogSlowQuery(ctx context.Context, collection, operation string, durationMs, docsCount int64, filterKeys []string, sortKeys map[string]int, operationID string) error
	GetArchiveGroups(ctx context.Context, collection, search string, skip, limit int) ([]ArchiveGroup, int64, error)
}
