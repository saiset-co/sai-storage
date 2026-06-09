package service

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"sort"
	"sync/atomic"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/saiset-co/sai-service/sai"
	saiTypes "github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-storage/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
)

type StorageService struct {
	repo                 types.StorageRepository
	validator            *validator.Validate
	logRequests          bool
	archiveChanges       bool
	trackQueryStats      bool
	slowQueryThresholdMs atomic.Int64
}

func NewStorageService(repo types.StorageRepository, features types.StorageFeaturesConfig) *StorageService {
	s := &StorageService{
		repo:            repo,
		validator:       validator.New(),
		logRequests:     features.LogRequests,
		archiveChanges:  features.ArchiveChanges,
		trackQueryStats: features.TrackQueryStats,
	}
	s.slowQueryThresholdMs.Store(int64(features.SlowQueryThresholdMs))
	return s
}

func (s *StorageService) GetSlowQueryThreshold() int64 {
	return s.slowQueryThresholdMs.Load()
}

func (s *StorageService) SetSlowQueryThreshold(ms int) {
	s.slowQueryThresholdMs.Store(int64(ms))
	go s.repo.UpdateDocuments(context.Background(), types.UpdateDocumentsRequest{
		Collection: "_admin_settings",
		Filter:     map[string]interface{}{"key": "slow_query_threshold_ms"},
		Data:       map[string]interface{}{"key": "slow_query_threshold_ms", "value": ms},
		Upsert:     true,
	})
}

func (s *StorageService) LoadSettings(ctx context.Context) {
	docs, _, err := s.repo.ReadDocuments(ctx, types.ReadDocumentsRequest{
		Collection: "_admin_settings",
		Filter:     map[string]interface{}{"key": "slow_query_threshold_ms"},
		Limit:      1,
	})
	if err != nil || len(docs) == 0 {
		return
	}
	switch t := docs[0]["value"].(type) {
	case int64:
		s.slowQueryThresholdMs.Store(t)
	case int32:
		s.slowQueryThresholdMs.Store(int64(t))
	case float64:
		s.slowQueryThresholdMs.Store(int64(t))
	}
}

func (s *StorageService) CreateDocuments(ctx context.Context, request types.CreateDocumentsRequest) (types.CreateDocumentsResponse, error) {
	if err := s.validator.Struct(request); err != nil {
		return types.CreateDocumentsResponse{}, saiTypes.WrapError(err, "validation failed")
	}

	t := time.Now()
	createdIDs, err := s.repo.CreateDocuments(ctx, request)
	if err != nil {
		return types.CreateDocumentsResponse{}, saiTypes.WrapError(err, "failed to create documents")
	}
	s.afterOp(request.Collection, "create", time.Since(t), int64(len(createdIDs)), nil)

	return types.CreateDocumentsResponse{
		Data:    createdIDs,
		Created: len(createdIDs),
	}, nil
}

func (s *StorageService) ReadDocuments(ctx context.Context, request types.ReadDocumentsRequest) (types.ReadDocumentsResponse, error) {
	if err := s.validator.Struct(request); err != nil {
		return types.ReadDocumentsResponse{}, saiTypes.WrapError(err, "validation failed")
	}

	t := time.Now()
	documents, total, err := s.repo.ReadDocuments(ctx, request)
	if err != nil {
		return types.ReadDocumentsResponse{}, saiTypes.WrapError(err, "failed to get documents")
	}
	s.afterOp(request.Collection, "find", time.Since(t), int64(len(documents)), mergeKeys(filterKeys(request.Filter), sortKeyList(request.Sort)))

	return types.ReadDocumentsResponse{
		Data:  documents,
		Total: total,
	}, nil
}

func (s *StorageService) AggregateDocuments(ctx context.Context, request types.AggregateDocumentsRequest) (types.AggregateDocumentsResponse, error) {
	if err := s.validator.Struct(request); err != nil {
		return types.AggregateDocumentsResponse{}, saiTypes.WrapError(err, "validation failed")
	}

	t := time.Now()
	documents, total, err := s.repo.AggregateDocuments(ctx, request)
	if err != nil {
		return types.AggregateDocumentsResponse{}, saiTypes.WrapError(err, "failed to aggregate documents")
	}
	s.afterOp(request.Collection, "aggregate", time.Since(t), int64(len(documents)), mergeKeys(filterKeys(request.Filter), matchKeys(request.Pipeline), sortKeyList(request.Sort)))

	return types.AggregateDocumentsResponse{
		Data:  documents,
		Total: total,
	}, nil
}

func (s *StorageService) UpdateDocuments(ctx context.Context, request types.UpdateDocumentsRequest) (types.UpdateDocumentsResponse, error) {
	if err := s.validator.Struct(request); err != nil {
		return types.UpdateDocumentsResponse{}, saiTypes.WrapError(err, "validation failed")
	}

	if s.archiveChanges {
		if err := s.archiveForUpdate(ctx, request); err != nil {
			return types.UpdateDocumentsResponse{}, err
		}
	}

	t := time.Now()
	updated, err := s.repo.UpdateDocuments(ctx, request)
	if err != nil {
		return types.UpdateDocumentsResponse{}, err
	}
	s.afterOp(request.Collection, "update", time.Since(t), updated, filterKeys(request.Filter))

	return types.UpdateDocumentsResponse{
		Data:    []string{},
		Updated: updated,
	}, nil
}

func (s *StorageService) DeleteDocuments(ctx context.Context, request types.DeleteDocumentsRequest) (types.DeleteDocumentsResponse, error) {
	if err := s.validator.Struct(request); err != nil {
		return types.DeleteDocumentsResponse{}, saiTypes.WrapError(err, "validation failed")
	}

	if s.archiveChanges {
		if err := s.archiveForDelete(ctx, request); err != nil {
			return types.DeleteDocumentsResponse{}, err
		}
	}

	t := time.Now()
	deleted, err := s.repo.DeleteDocuments(ctx, request)
	if err != nil {
		return types.DeleteDocumentsResponse{}, saiTypes.WrapError(err, "failed to delete documents")
	}
	s.afterOp(request.Collection, "delete", time.Since(t), deleted, filterKeys(request.Filter))

	return types.DeleteDocumentsResponse{
		Data:    []string{},
		Deleted: deleted,
	}, nil
}

func (s *StorageService) afterOp(collection, operation string, elapsed time.Duration, docsCount int64, allKeys []string) {
	if s.trackQueryStats && len(allKeys) > 0 {
		s.upsertQueryStat(context.Background(), collection, operation, allKeys)
	}
	threshold := s.slowQueryThresholdMs.Load()
	if threshold > 0 && elapsed.Milliseconds() >= threshold && !isAdminCollection(collection) && len(allKeys) > 0 {
		go s.repo.LogSlowQuery(context.Background(), collection, operation, elapsed.Milliseconds(), docsCount, allKeys)
	}
}

func isAdminCollection(name string) bool {
	return len(name) > 0 && (name[0] == '_' || len(name) > 7 && name[:7] == "system.")
}

func (s *StorageService) Close(ctx context.Context) error {
	return s.repo.Close(ctx)
}

func (s *StorageService) LogRequest(ctx context.Context, collection string, data map[string]interface{}) {
	if !s.logRequests {
		return
	}

	if collection == "" {
		collection = "unknown"
	}

	req := types.CreateDocumentsRequest{
		Collection: fmt.Sprintf("%s_request_logs", collection),
		Data:       []interface{}{data},
	}

	if _, err := s.repo.CreateDocuments(ctx, req); err != nil {
		sai.Logger().Warn("Failed to log request", zap.Error(err))
	}
}

func (s *StorageService) GetRepo() types.StorageRepository {
	return s.repo
}


func (s *StorageService) archiveForUpdate(ctx context.Context, request types.UpdateDocumentsRequest) error {
	if request.Collection == "" {
		return nil
	}

	docs, _, err := s.repo.ReadDocuments(ctx, types.ReadDocumentsRequest{
		Collection: request.Collection,
		Filter:     request.Filter,
	})
	if err != nil {
		return saiTypes.WrapError(err, "failed to read documents for update archive")
	}

	if len(docs) == 0 {
		return nil
	}

	return s.writeArchive(ctx, request.Collection, "update_archive", docs, map[string]interface{}{
		"archive_filter": request.Filter,
		"archive_update": request.Data,
	})
}

func (s *StorageService) archiveForDelete(ctx context.Context, request types.DeleteDocumentsRequest) error {
	if request.Collection == "" {
		return nil
	}

	docs, _, err := s.repo.ReadDocuments(ctx, types.ReadDocumentsRequest{
		Collection: request.Collection,
		Filter:     request.Filter,
	})
	if err != nil {
		return saiTypes.WrapError(err, "failed to read documents for delete archive")
	}

	if len(docs) == 0 {
		return nil
	}

	return s.writeArchive(ctx, request.Collection, "delete_archive", docs, map[string]interface{}{
		"archive_filter": request.Filter,
	})
}

func (s *StorageService) writeArchive(ctx context.Context, collection, suffix string, docs []map[string]interface{}, meta map[string]interface{}) error {
	operationID := uuid.New().String()
	archiveTime := time.Now().UnixNano()

	archiveDocs := make([]interface{}, 0, len(docs))
	for _, doc := range docs {
		copyDoc := make(map[string]interface{}, len(doc)+3+len(meta))
		for k, v := range doc {
			copyDoc[k] = v
		}
		delete(copyDoc, "_id")
		copyDoc["archive_operation_id"] = operationID
		copyDoc["archive_time"] = archiveTime
		copyDoc["source_collection"] = collection
		for k, v := range meta {
			copyDoc[k] = v
		}
		archiveDocs = append(archiveDocs, copyDoc)
	}

	req := types.CreateDocumentsRequest{
		Collection: fmt.Sprintf("%s_%s", collection, suffix),
		Data:       archiveDocs,
	}

	_, err := s.repo.CreateDocuments(ctx, req)
	if err != nil {
		return saiTypes.WrapError(err, "failed to archive documents")
	}

	go s.repo.CreateIndex(context.Background(), types.CreateIndexRequest{
		Collection: req.Collection,
		Keys:       map[string]int{"archive_operation_id": 1, "archive_time": -1},
		Name:       "archive_op_idx",
	})

	return nil
}

func (s *StorageService) upsertQueryStat(_ context.Context, collection, operation string, keys []string) {
	if keys == nil {
		keys = []string{}
	}
	fingerprint := filterFingerprint(keys)
	now := time.Now().UnixNano()

	go func() {
		req := types.UpdateDocumentsRequest{
			Collection: "_admin_query_stats",
			Filter: map[string]interface{}{
				"collection":         collection,
				"operation":          operation,
				"filter_fingerprint": fingerprint,
			},
			Data: map[string]interface{}{
				"$set": map[string]interface{}{
					"collection":         collection,
					"operation":          operation,
					"filter_fingerprint": fingerprint,
					"filter_keys":        keys,
					"last_seen":          now,
				},
				"$inc": map[string]interface{}{
					"count": 1,
				},
			},
			Upsert: true,
		}
		if _, err := s.repo.UpdateDocuments(context.Background(), req); err != nil {
			sai.Logger().Warn("Failed to upsert query stat", zap.Error(err))
		}
	}()
}

func filterKeys(filter map[string]interface{}) []string {
	if len(filter) == 0 {
		return []string{}
	}
	keySet := make(map[string]struct{})
	extractFilterKeys(filter, keySet)
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func extractFilterKeys(filter map[string]interface{}, keySet map[string]struct{}) {
	for k, v := range filter {
		if len(k) > 0 && k[0] == '$' {
			if arr, ok := v.([]interface{}); ok {
				for _, item := range arr {
					if sub, ok := item.(map[string]interface{}); ok {
						extractFilterKeys(sub, keySet)
					}
				}
			}
		} else {
			keySet[k] = struct{}{}
		}
	}
}

func filterFingerprint(keys []string) string {
	data, _ := json.Marshal(keys)
	return fmt.Sprintf("%x", md5.Sum(data))
}

func matchKeys(pipeline types.OrderedPipeline) []string {
	keySet := make(map[string]struct{})
	for _, stage := range pipeline {
		for _, elem := range stage {
			if elem.Key == "$match" {
				if matchDoc, ok := elem.Value.(bson.D); ok {
					for _, field := range matchDoc {
						if field.Key != "" && field.Key[0] != '$' {
							keySet[field.Key] = struct{}{}
						}
					}
				}
			}
		}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortKeyList(sortMap map[string]int) []string {
	if len(sortMap) == 0 {
		return nil
	}
	keys := make([]string, 0, len(sortMap))
	for k := range sortMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func mergeKeys(slices ...[]string) []string {
	keySet := make(map[string]struct{})
	for _, s := range slices {
		for _, k := range s {
			if k != "" {
				keySet[k] = struct{}{}
			}
		}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
