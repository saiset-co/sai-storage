package redis

import (
	"context"

	"github.com/saiset-co/sai-storage/types"
)

func (r *Repository) GetAdminCollectionStats(ctx context.Context) ([]types.CollectionStats, error) {
	return nil, nil
}

func (r *Repository) ListCollectionNames(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (r *Repository) ListIndexes(ctx context.Context, collection string) ([]types.IndexInfo, error) {
	return nil, nil
}

func (r *Repository) CreateIndex(ctx context.Context, req types.CreateIndexRequest) error {
	return nil
}

func (r *Repository) GetSlowQueries(ctx context.Context, limit int) ([]types.SlowQuery, error) {
	return nil, nil
}

func (r *Repository) LogSlowQuery(ctx context.Context, collection, operation string, durationMs, docsCount int64, filterKeys []string) error {
	return nil
}

func (r *Repository) GetArchiveGroups(ctx context.Context, collection string, skip, limit int) ([]types.ArchiveGroup, int64, error) {
	return nil, 0, nil
}
