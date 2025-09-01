package mongo

import (
	"context"
	"github.com/saiset-co/sai-service/sai"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	saiTypes "github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-storage/types"
)

type Repository struct {
	client *Client
}

func NewRepository() (types.StorageRepository, error) {
	var mongoConfig types.StorageConfig
	err := sai.Config().GetAs("storage.mongo", &mongoConfig)
	if err != nil {
		return nil, err
	}

	client, err := NewClient(mongoConfig)
	if err != nil {
		return nil, err
	}

	uuid.EnableRandPool()

	return &Repository{
		client: client,
	}, nil
}

func (r *Repository) CreateDocuments(ctx context.Context, request types.CreateDocumentsRequest) ([]string, error) {
	if len(request.Data) == 0 {
		return []string{}, nil
	}

	coll := r.client.GetCollection(request.Collection)

	var counter int64

	for _, data := range request.Data {
		now := time.Now().UnixNano() + atomic.AddInt64(&counter, 1)
		data.(map[string]interface{})["internal_id"] = uuid.New().String()
		data.(map[string]interface{})["cr_time"] = now
		data.(map[string]interface{})["ch_time"] = now
	}

	result, err := coll.InsertMany(ctx, request.Data)
	if err != nil {
		return nil, saiTypes.WrapError(err, "failed to insert documents")
	}

	ids := make([]string, len(result.InsertedIDs))
	for i, id := range result.InsertedIDs {
		switch v := id.(type) {
		case primitive.ObjectID:
			ids[i] = v.Hex()
		case string:
			ids[i] = v
		default:
			return nil, saiTypes.NewErrorf("unsupported inserted ID type: %T", id)
		}
	}

	return ids, nil
}

func (r *Repository) ReadDocuments(ctx context.Context, request types.ReadDocumentsRequest) ([]map[string]interface{}, int64, error) {
	coll := r.client.GetCollection(request.Collection)

	total, err := coll.CountDocuments(ctx, request.Filter)
	if err != nil {
		return nil, 0, saiTypes.WrapError(err, "failed to count documents")
	}

	findOptions := options.Find()

	if request.Sort != nil && len(request.Sort) > 0 {
		findOptions.SetSort(request.Sort)
	}

	if request.Limit > 0 {
		findOptions.SetLimit(int64(request.Limit))
	}

	if request.Skip > 0 {
		findOptions.SetSkip(int64(request.Skip))
	}

	if len(request.Fields) > 0 {
		projection := make(map[string]int)
		for _, field := range request.Fields {
			projection[field] = 1
		}
		findOptions.SetProjection(projection)
	}

	cursor, err := coll.Find(ctx, request.Filter, findOptions)
	if err != nil {
		return nil, 0, saiTypes.WrapError(err, "failed to find documents")
	}
	defer cursor.Close(ctx)

	var results []map[string]interface{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, saiTypes.WrapError(err, "failed to decode documents")
	}

	return results, total, nil
}

func (r *Repository) UpdateDocuments(ctx context.Context, request types.UpdateDocumentsRequest) (int64, error) {
	coll := r.client.GetCollection(request.Collection)
	_options := &options.UpdateOptions{}

	if request.Upsert {
		_options.SetUpsert(true)
	}

	var counter int64

	if data, ok := request.Data.(map[string]interface{}); ok {
		if setOp, exists := data["$set"]; exists {
			if setMap, ok := setOp.(map[string]interface{}); ok {
				setMap["ch_time"] = time.Now().UnixNano() + atomic.AddInt64(&counter, 1)
			}
		} else {
			data["$set"] = map[string]interface{}{
				"ch_time": time.Now().UnixNano() + atomic.AddInt64(&counter, 1),
			}
		}
	}

	result, err := coll.UpdateMany(ctx, request.Filter, request.Data, _options)
	if err != nil {
		return 0, saiTypes.WrapError(err, "mongo failed to update documents")
	}

	return result.ModifiedCount, nil
}

func (r *Repository) DeleteDocuments(ctx context.Context, request types.DeleteDocumentsRequest) (int64, error) {
	coll := r.client.GetCollection(request.Collection)

	result, err := coll.DeleteMany(ctx, request.Filter)
	if err != nil {
		return 0, saiTypes.WrapError(err, "failed to delete documents")
	}

	return result.DeletedCount, nil
}

func (r *Repository) Close(ctx context.Context) error {
	return r.client.Close(ctx)
}
