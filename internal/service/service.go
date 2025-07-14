package service

import (
	"context"
	"github.com/go-playground/validator/v10"
	saiTypes "github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-storage/types"
)

type StorageService struct {
	repo      types.StorageRepository
	validator *validator.Validate
}

func NewStorageService(repo types.StorageRepository) *StorageService {
	return &StorageService{
		repo:      repo,
		validator: validator.New(),
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

func (s *StorageService) UpdateDocuments(ctx context.Context, request types.UpdateDocumentsRequest) (types.UpdateDocumentsResponse, error) {
	if err := s.validator.Struct(request); err != nil {
		return types.UpdateDocumentsResponse{}, saiTypes.WrapError(err, "validation failed")
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
