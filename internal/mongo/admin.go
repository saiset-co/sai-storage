package mongo

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/sync/errgroup"

	saiTypes "github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-storage/types"
)

var serviceCollectionSuffixes = []string{
	"_update_archive",
	"_delete_archive",
	"_request_logs",
}

var serviceCollectionPrefixes = []string{
	"_admin_",
	"system.",
}

func isServiceCollection(name string) bool {
	for _, prefix := range serviceCollectionPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	for _, suffix := range serviceCollectionSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func (r *Repository) GetAdminCollectionStats(ctx context.Context) ([]types.CollectionStats, error) {
	names, err := r.client.database.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return nil, saiTypes.WrapError(err, "failed to list collections")
	}

	userNames := make([]string, 0, len(names))
	for _, name := range names {
		if !isServiceCollection(name) {
			userNames = append(userNames, name)
		}
	}

	result := make([]types.CollectionStats, len(userNames))
	g, gctx := errgroup.WithContext(ctx)

	for i, name := range userNames {
		i, name := i, name
		g.Go(func() error {
			var raw bson.M
			cmd := bson.D{{Key: "collStats", Value: name}}
			if err := r.client.database.RunCommand(gctx, cmd).Decode(&raw); err != nil {
				result[i] = types.CollectionStats{Name: name}
				return nil
			}
			stats := types.CollectionStats{Name: name}
			if v, ok := raw["count"]; ok {
				stats.Count = toInt64(v)
			}
			if v, ok := raw["storageSize"]; ok {
				stats.StorageSize = toInt64(v)
			}
			if v, ok := raw["totalIndexSize"]; ok {
				stats.IndexSize = toInt64(v)
			}
			if v, ok := raw["nindexes"]; ok {
				stats.NumIndexes = int(toInt64(v))
			}
			result[i] = stats
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *Repository) ListCollectionNames(ctx context.Context) ([]string, error) {
	return r.client.database.ListCollectionNames(ctx, bson.D{})
}

func (r *Repository) ListIndexes(ctx context.Context, collection string) ([]types.IndexInfo, error) {
	col := r.client.GetCollection(collection)
	cursor, err := col.Indexes().List(ctx)
	if err != nil {
		return nil, saiTypes.WrapError(err, "failed to list indexes")
	}
	defer cursor.Close(ctx)

	var rawIndexes []bson.M
	if err := cursor.All(ctx, &rawIndexes); err != nil {
		return nil, saiTypes.WrapError(err, "failed to decode indexes")
	}

	result := make([]types.IndexInfo, 0, len(rawIndexes))
	for _, raw := range rawIndexes {
		info := types.IndexInfo{
			Fields: make(map[string]int),
		}
		if v, ok := raw["name"].(string); ok {
			info.Name = v
		}
		if v, ok := raw["unique"].(bool); ok {
			info.Unique = v
		}
		if v, ok := raw["sparse"].(bool); ok {
			info.Sparse = v
		}
		if key, ok := raw["key"].(bson.M); ok {
			for field, dir := range key {
				info.Fields[field] = int(toInt64(dir))
			}
		}
		result = append(result, info)
	}
	return result, nil
}

func (r *Repository) CreateIndex(ctx context.Context, req types.CreateIndexRequest) error {
	col := r.client.GetCollection(req.Collection)

	keys := make(bson.D, 0, len(req.Keys))
	for field, dir := range req.Keys {
		keys = append(keys, bson.E{Key: field, Value: dir})
	}

	model := mongo.IndexModel{
		Keys: keys,
		Options: options.Index().
			SetUnique(req.Unique).
			SetSparse(req.Sparse),
	}
	if req.Name != "" {
		model.Options.SetName(req.Name)
	}

	_, err := col.Indexes().CreateOne(ctx, model)
	if err != nil {
		return saiTypes.WrapError(err, "failed to create index")
	}
	return nil
}

func (r *Repository) GetSlowQueries(ctx context.Context, limit int) ([]types.SlowQuery, error) {
	col := r.client.database.Collection("_admin_slow_queries")

	opts := options.Find().
		SetSort(bson.D{{Key: "ts", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := col.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, saiTypes.WrapError(err, "failed to query slow queries")
	}
	defer cursor.Close(ctx)

	var raw []bson.M
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, saiTypes.WrapError(err, "failed to decode slow queries")
	}

	result := make([]types.SlowQuery, 0, len(raw))
	for _, entry := range raw {
		sq := types.SlowQuery{}
		if v, ok := entry["collection"].(string); ok {
			sq.Collection = v
		}
		if v, ok := entry["operation"].(string); ok {
			sq.Op = v
		}
		sq.DurationMs = toInt64(entry["duration_ms"])
		sq.Timestamp = time.Unix(0, toInt64(entry["ts"]))
		if fk, ok := entry["filter_keys"].(bson.A); ok {
			for _, k := range fk {
				if s, ok := k.(string); ok {
					sq.FilterKeys = append(sq.FilterKeys, s)
				}
			}
		}
		result = append(result, sq)
	}
	return result, nil
}

func (r *Repository) GetArchiveGroups(ctx context.Context, collection string, skip, limit int) ([]types.ArchiveGroup, int64, error) {
	col := r.client.database.Collection(collection)

	match := bson.D{{Key: "$match", Value: bson.D{
		{Key: "archive_operation_id", Value: bson.D{
			{Key: "$exists", Value: true},
			{Key: "$ne", Value: ""},
		}},
	}}}
	group := bson.D{{Key: "$group", Value: bson.D{
		{Key: "_id", Value: "$archive_operation_id"},
		{Key: "archive_time", Value: bson.D{{Key: "$max", Value: "$archive_time"}}},
		{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		{Key: "filter", Value: bson.D{{Key: "$first", Value: "$archive_filter"}}},
		{Key: "update_data", Value: bson.D{{Key: "$first", Value: "$archive_update"}}},
		{Key: "restored_at", Value: bson.D{{Key: "$max", Value: "$restored_at"}}},
	}}}
	sort := bson.D{{Key: "$sort", Value: bson.D{{Key: "archive_time", Value: -1}}}}

	var total int64
	countCursor, err := col.Aggregate(ctx, bson.A{match, group, bson.D{{Key: "$count", Value: "n"}}})
	if err == nil {
		var cr []bson.M
		if err2 := countCursor.All(ctx, &cr); err2 == nil && len(cr) > 0 {
			total = toInt64(cr[0]["n"])
		}
		countCursor.Close(ctx)
	}

	cursor, err := col.Aggregate(ctx, bson.A{
		match, group, sort,
		bson.D{{Key: "$skip", Value: int64(skip)}},
		bson.D{{Key: "$limit", Value: int64(limit)}},
	})
	if err != nil {
		return nil, 0, saiTypes.WrapError(err, "failed to aggregate archive groups")
	}
	defer cursor.Close(ctx)

	var raw []bson.M
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, 0, saiTypes.WrapError(err, "failed to decode archive groups")
	}

	result := make([]types.ArchiveGroup, 0, len(raw))
	for _, doc := range raw {
		opID := ""
		if v, ok := doc["_id"].(string); ok {
			opID = v
		}
		result = append(result, types.ArchiveGroup{
			OperationID: opID,
			ArchiveTime: toInt64(doc["archive_time"]),
			Count:       toInt64(doc["count"]),
			Filter:      doc["filter"],
			Update:      doc["update_data"],
			RestoredAt:  toInt64(doc["restored_at"]),
		})
	}
	return result, total, nil
}

func (r *Repository) LogSlowQuery(ctx context.Context, collection, operation string, durationMs, docsCount int64, filterKeys []string) error {
	col := r.client.database.Collection("_admin_slow_queries")
	_, err := col.InsertOne(ctx, bson.M{
		"collection":         collection,
		"operation":          operation,
		"duration_ms":        durationMs,
		"docs_count":         docsCount,
		"filter_keys":        filterKeys,
		"filter_fingerprint": slowQueryFingerprint(filterKeys),
		"ts":                 time.Now().UnixNano(),
	})
	return err
}

func slowQueryFingerprint(keys []string) string {
	data, _ := json.Marshal(keys)
	return fmt.Sprintf("%x", md5.Sum(data))
}

func toInt64(v interface{}) int64 {
	switch t := v.(type) {
	case int32:
		return int64(t)
	case int64:
		return t
	case float64:
		return int64(t)
	case int:
		return int64(t)
	}
	return 0
}
