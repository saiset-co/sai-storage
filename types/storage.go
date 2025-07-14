package types

import (
	"context"
)

type StorageRepository interface {
	CreateDocuments(ctx context.Context, request CreateDocumentsRequest) ([]string, error)
	ReadDocuments(ctx context.Context, request ReadDocumentsRequest) ([]map[string]interface{}, int64, error)
	UpdateDocuments(ctx context.Context, request UpdateDocumentsRequest) (int64, error)
	DeleteDocuments(ctx context.Context, request DeleteDocumentsRequest) (int64, error)
	Close(ctx context.Context) error
}
