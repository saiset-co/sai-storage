package service

import (
	"context"
	"crypto/md5"
	"fmt"
	"sort"
	"strings"
	"sync"
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
	indexedArchives      sync.Map
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
	s.afterOp(ctx, request.Collection, "create", time.Since(t), int64(len(createdIDs)), nil, nil)

	if s.archiveChanges {
		s.archiveForCreate(ctx, request.Collection, request.Data)
	}

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
	s.afterOp(ctx, request.Collection, "find", time.Since(t), int64(len(documents)), filterKeys(request.Filter), request.Sort)

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
	s.afterOp(ctx, request.Collection, "aggregate", time.Since(t), int64(len(documents)), matchKeys(request.Pipeline), nil)

	return types.AggregateDocumentsResponse{
		Data:  documents,
		Total: total,
	}, nil
}

func (s *StorageService) UpdateDocuments(ctx context.Context, request types.UpdateDocumentsRequest) (types.UpdateDocumentsResponse, error) {
	if err := s.validator.Struct(request); err != nil {
		return types.UpdateDocumentsResponse{}, saiTypes.WrapError(err, "validation failed")
	}

	preExisted := false
	if s.archiveChanges {
		var archErr error
		preExisted, archErr = s.archiveForUpdate(ctx, request)
		if archErr != nil {
			return types.UpdateDocumentsResponse{}, archErr
		}
	}

	t := time.Now()
	updated, err := s.repo.UpdateDocuments(ctx, request)
	if err != nil {
		return types.UpdateDocumentsResponse{}, err
	}

	if s.archiveChanges && request.Upsert && !preExisted {
		s.archiveUpsertInsert(ctx, request)
	}

	s.afterOp(ctx, request.Collection, "update", time.Since(t), updated, filterKeys(request.Filter), nil)

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
	s.afterOp(ctx, request.Collection, "delete", time.Since(t), deleted, filterKeys(request.Filter), nil)

	return types.DeleteDocumentsResponse{
		Data:    []string{},
		Deleted: deleted,
	}, nil
}

func extractOperationID(ctx context.Context) string {
	if reqCtx, ok := ctx.(*saiTypes.RequestCtx); ok {
		if v := reqCtx.UserValue("operation_id"); v != nil {
			if id, ok2 := v.(string); ok2 {
				return id
			}
		}
	}
	return ""
}

func (s *StorageService) afterOp(ctx context.Context, collection, operation string, elapsed time.Duration, docsCount int64, fKeys []string, sortKeys map[string]int) {
	operationID := extractOperationID(ctx)
	if s.trackQueryStats && (len(fKeys) > 0 || len(sortKeys) > 0) {
		s.upsertQueryStat(collection, operation, fKeys, sortKeys, operationID)
	}
	threshold := s.slowQueryThresholdMs.Load()
	if threshold > 0 && elapsed.Milliseconds() >= threshold && !isAdminCollection(collection) && (len(fKeys) > 0 || len(sortKeys) > 0) {
		go s.repo.LogSlowQuery(context.Background(), collection, operation, elapsed.Milliseconds(), docsCount, fKeys, sortKeys, operationID)
	}
}

func isAdminCollection(name string) bool {
	return len(name) > 0 && (name[0] == '_' || len(name) > 7 && name[:7] == "system.")
}

func (s *StorageService) Close(ctx context.Context) error {
	return s.repo.Close(ctx)
}

func (s *StorageService) LogRequest(_ context.Context, collection string, data map[string]interface{}) {
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

	go func() {
		if _, err := s.repo.CreateDocuments(context.Background(), req); err != nil {
			sai.Logger().Warn("Failed to log request", zap.Error(err))
		}
	}()
}

func (s *StorageService) GetRepo() types.StorageRepository {
	return s.repo
}


func (s *StorageService) archiveForUpdate(ctx context.Context, request types.UpdateDocumentsRequest) (bool, error) {
	if request.Collection == "" {
		return false, nil
	}

	docs, _, err := s.repo.ReadDocuments(ctx, types.ReadDocumentsRequest{
		Collection: request.Collection,
		Filter:     request.Filter,
	})
	if err != nil {
		return false, saiTypes.WrapError(err, "failed to read documents for update archive")
	}

	if len(docs) == 0 {
		return false, nil
	}

	return true, s.writeArchive(ctx, request.Collection, "update_archive", docs, map[string]interface{}{
		"archive_filter": request.Filter,
		"archive_update": request.Data,
	})
}

func (s *StorageService) archiveUpsertInsert(ctx context.Context, request types.UpdateDocumentsRequest) {
	docs, _, err := s.repo.ReadDocuments(ctx, types.ReadDocumentsRequest{
		Collection: request.Collection,
		Filter:     request.Filter,
	})
	if err != nil || len(docs) == 0 {
		return
	}
	s.writeArchive(ctx, request.Collection, "update_archive", docs, map[string]interface{}{
		"archive_filter": request.Filter,
		"archive_update": request.Data,
		"upsert_insert":  true,
	})
}

func (s *StorageService) archiveForCreate(ctx context.Context, collection string, data []interface{}) {
	if len(data) == 0 {
		return
	}
	docs := make([]map[string]interface{}, 0, len(data))
	ids := make([]string, 0, len(data))
	for _, d := range data {
		m, ok := d.(map[string]interface{})
		if !ok {
			continue
		}
		docs = append(docs, m)
		if id, ok := m["internal_id"].(string); ok && id != "" {
			ids = append(ids, id)
		}
	}
	if len(docs) == 0 {
		return
	}
	s.writeArchive(ctx, collection, "create_archive", docs, map[string]interface{}{
		"archive_filter": map[string]interface{}{"internal_id": map[string]interface{}{"$in": ids}},
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
	operationID := extractOperationID(ctx)
	if operationID == "" {
		operationID = uuid.New().String()
	}
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

	if _, exists := s.indexedArchives.LoadOrStore(req.Collection, true); !exists {
		go s.repo.CreateIndex(context.Background(), types.CreateIndexRequest{
			Collection: req.Collection,
			Keys:       map[string]int{"archive_operation_id": 1, "archive_time": -1},
			Name:       "archive_op_idx",
		})
	}

	return nil
}

func (s *StorageService) upsertQueryStat(collection, operation string, fKeys []string, sortKeys map[string]int, operationID string) {
	if fKeys == nil {
		fKeys = []string{}
	}
	fingerprint := filterFingerprint(fKeys)
	now := time.Now().UnixNano()

	go func() {
		setData := map[string]interface{}{
			"collection":         collection,
			"operation":          operation,
			"filter_fingerprint": fingerprint,
			"filter_keys":        fKeys,
			"sort_keys":          sortKeys,
			"last_seen":          now,
		}
		if operationID != "" {
			setData["last_operation_id"] = operationID
		}
		req := types.UpdateDocumentsRequest{
			Collection: "_admin_query_stats",
			Filter: map[string]interface{}{
				"collection":         collection,
				"operation":          operation,
				"filter_fingerprint": fingerprint,
			},
			Data: map[string]interface{}{
				"$set": setData,
				"$inc": map[string]interface{}{"count": 1},
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
	return fmt.Sprintf("%x", md5.Sum([]byte(strings.Join(keys, "|"))))
}

func matchKeys(pipeline types.OrderedPipeline) []string {
	computed := make(map[string]struct{})
	keySet := make(map[string]struct{})
	for _, stage := range pipeline {
		for _, elem := range stage {
			switch elem.Key {
			case "$addFields", "$set":
				if doc, ok := elem.Value.(bson.D); ok {
					for _, f := range doc {
						computed[f.Key] = struct{}{}
					}
				}
			case "$match":
				if doc, ok := elem.Value.(bson.D); ok {
					extractBsonDKeys(doc, keySet)
				}
			}
		}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		if _, isComputed := computed[k]; !isComputed {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

func extractBsonDKeys(doc bson.D, keySet map[string]struct{}) {
	for _, field := range doc {
		if field.Key == "" {
			continue
		}
		if field.Key[0] == '$' {
			if arr, ok := field.Value.(bson.A); ok {
				for _, item := range arr {
					if sub, ok := item.(bson.D); ok {
						extractBsonDKeys(sub, keySet)
					}
				}
			}
		} else {
			keySet[field.Key] = struct{}{}
		}
	}
}

