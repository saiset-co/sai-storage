package service

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/saiset-co/sai-service/sai"
	saiTypes "github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-storage/types"
	"go.uber.org/zap"
)

type StorageService struct {
	repo           types.StorageRepository
	validator      *validator.Validate
	logRequests    bool
	archiveChanges bool
}

func NewStorageService(repo types.StorageRepository, features types.StorageFeaturesConfig) *StorageService {
	return &StorageService{
		repo:           repo,
		validator:      validator.New(),
		logRequests:    features.LogRequests,
		archiveChanges: features.ArchiveChanges,
	}
}

func (s *StorageService) CreateDocuments(ctx context.Context, request types.CreateDocumentsRequest) (types.CreateDocumentsResponse, error) {
	if err := s.validator.Struct(request); err != nil {
		return types.CreateDocumentsResponse{}, saiTypes.WrapError(err, "validation failed")
	}

	createdIDs, err := s.repo.CreateDocuments(ctx, request)
	if err != nil {
		return types.CreateDocumentsResponse{}, saiTypes.WrapError(err, "failed to create documents")
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

	documents, total, err := s.repo.ReadDocuments(ctx, request)
	if err != nil {
		return types.ReadDocumentsResponse{}, saiTypes.WrapError(err, "failed to get documents")
	}

	return types.ReadDocumentsResponse{
		Data:  documents,
		Total: total,
	}, nil
}

func (s *StorageService) AggregateDocuments(ctx context.Context, request types.AggregateDocumentsRequest) (types.AggregateDocumentsResponse, error) {
	if err := s.validator.Struct(request); err != nil {
		return types.AggregateDocumentsResponse{}, saiTypes.WrapError(err, "validation failed")
	}

	documents, total, err := s.repo.AggregateDocuments(ctx, request)
	if err != nil {
		return types.AggregateDocumentsResponse{}, saiTypes.WrapError(err, "failed to aggregate documents")
	}

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

	updated, err := s.repo.UpdateDocuments(ctx, request)
	if err != nil {
		return types.UpdateDocumentsResponse{}, err
	}

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

	deleted, err := s.repo.DeleteDocuments(ctx, request)
	if err != nil {
		return types.DeleteDocumentsResponse{}, saiTypes.WrapError(err, "failed to delete documents")
	}

	return types.DeleteDocumentsResponse{
		Data:    []string{},
		Deleted: deleted,
	}, nil
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

func (s *StorageService) archiveForUpdate(ctx context.Context, request types.UpdateDocumentsRequest) error {
	if request.Collection == "" {
		return nil
	}

	readRequest := types.ReadDocumentsRequest{
		Collection: request.Collection,
		Filter:     request.Filter,
	}

	docs, _, err := s.repo.ReadDocuments(ctx, readRequest)
	if err != nil {
		return saiTypes.WrapError(err, "failed to read documents for update archive")
	}

	if len(docs) == 0 {
		return nil
	}

	return s.writeArchive(ctx, request.Collection, "update_archive", docs)
}

func (s *StorageService) archiveForDelete(ctx context.Context, request types.DeleteDocumentsRequest) error {
	if request.Collection == "" {
		return nil
	}

	readRequest := types.ReadDocumentsRequest{
		Collection: request.Collection,
		Filter:     request.Filter,
	}

	docs, _, err := s.repo.ReadDocuments(ctx, readRequest)
	if err != nil {
		return saiTypes.WrapError(err, "failed to read documents for delete archive")
	}

	if len(docs) == 0 {
		return nil
	}

	return s.writeArchive(ctx, request.Collection, "delete_archive", docs)
}

func (s *StorageService) writeArchive(ctx context.Context, collection, suffix string, docs []map[string]interface{}) error {
	archiveDocs := make([]interface{}, 0, len(docs))
	for _, doc := range docs {
		copyDoc := make(map[string]interface{}, len(doc))
		for k, v := range doc {
			copyDoc[k] = v
		}
		delete(copyDoc, "_id")
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

	return nil
}
