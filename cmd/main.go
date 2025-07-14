package main

import (
	"context"
	"go.uber.org/zap"
	"log"

	"github.com/saiset-co/sai-service/sai"
	"github.com/saiset-co/sai-service/service"
	"github.com/saiset-co/sai-storage/internal/handlers"
	"github.com/saiset-co/sai-storage/internal/mongo"
	serviceLayer "github.com/saiset-co/sai-storage/internal/service"
	"github.com/saiset-co/sai-storage/types"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := service.NewService(ctx, "./config.yaml")
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	if err := initializeComponents(); err != nil {
		sai.Logger().Error("Failed to initialize components", zap.Error(err))
		cancel()
	}

	if err := srv.Start(); err != nil {
		sai.Logger().Error("Failed to start service", zap.Error(err))
		cancel()
	}

	return
}

func initializeComponents() (err error) {
	var repo types.StorageRepository

	switch sai.Config().GetValue("storage.type", "mongo").(string) {
	default:
		repo, err = mongo.NewRepository()
		if err != nil {
			return err
		}
	}

	storageService := serviceLayer.NewStorageService(repo)
	handler := handlers.NewHandler(storageService)

	documents := sai.Router().Group("/api/v1").Group("/documents")

	documents.POST("/", handler.CreateDocuments).
		WithDoc("Create Documents", "Create multiple documents in a collection", "documents", &types.CreateDocumentsRequest{}, &types.CreateDocumentsResponse{})

	documents.GET("/", handler.ReadDocuments).
		WithDoc("Get Documents", "Get documents with filtering and pagination. Add ?count=1 to include total count", "documents", &types.ReadDocumentsRequest{}, &types.ReadDocumentsResponse{})

	documents.PUT("/", handler.UpdateDocuments).
		WithDoc("Update Documents", "Update multiple documents by filter", "documents", &types.UpdateDocumentsRequest{}, &types.UpdateDocumentsResponse{})

	documents.DELETE("/", handler.DeleteDocuments).
		WithDoc("Delete Documents", "Delete multiple documents by filter", "documents", &types.DeleteDocumentsRequest{}, &types.DeleteDocumentsResponse{})

	return nil
}
