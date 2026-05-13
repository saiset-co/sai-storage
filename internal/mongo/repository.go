package mongo

import (
	"context"
	"encoding/json"
	"github.com/saiset-co/sai-service/sai"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
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

	for i, data := range request.Data {
		dataMap, err := normalizeDocumentMap(data)
		if err != nil {
			return nil, err
		}

		now := time.Now().UnixNano() + atomic.AddInt64(&counter, 1)
		if dataMap["internal_id"] == nil || dataMap["internal_id"] == "" {
			dataMap["internal_id"] = uuid.New().String()
		}
		dataMap["cr_time"] = now
		dataMap["ch_time"] = now
		request.Data[i] = dataMap
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

	findOptions := options.Find()

	if len(request.Sort) > 0 {
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

	var total int64
	if request.Count > 0 {
		count, err := coll.CountDocuments(ctx, request.Filter)
		if err != nil {
			return nil, 0, saiTypes.WrapError(err, "failed to count documents")
		}
		total = count
	} else {
		total = int64(len(results))
	}

	return results, total, nil
}

func (r *Repository) AggregateDocuments(ctx context.Context, request types.AggregateDocumentsRequest) ([]map[string]interface{}, int64, error) {
	if len(request.Pipeline) == 0 {
		return nil, 0, saiTypes.NewError("pipeline is required for mongo aggregate")
	}

	coll := r.client.GetCollection(request.Collection)

	pipeline := make([]interface{}, 0, len(request.Pipeline)+2)
	for _, stage := range request.Pipeline {
		pipeline = append(pipeline, stage)
	}

	if request.Skip > 0 {
		pipeline = append(pipeline, bson.M{"$skip": request.Skip})
	}

	if request.Limit > 0 {
		pipeline = append(pipeline, bson.M{"$limit": request.Limit})
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, saiTypes.WrapError(err, "failed to aggregate documents")
	}
	defer cursor.Close(ctx)

	var results []map[string]interface{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, saiTypes.WrapError(err, "failed to decode aggregation results")
	}

	var total int64
	if request.Count > 0 {
		countPipeline := make([]interface{}, 0, len(request.Pipeline)+1)
		for _, stage := range request.Pipeline {
			countPipeline = append(countPipeline, stage)
		}
		countPipeline = append(countPipeline, bson.M{"$count": "total"})
		countCursor, err := coll.Aggregate(ctx, countPipeline)
		if err != nil {
			return nil, 0, saiTypes.WrapError(err, "failed to count aggregation results")
		}
		defer countCursor.Close(ctx)

		var countResult []map[string]interface{}
		if err := countCursor.All(ctx, &countResult); err != nil {
			return nil, 0, saiTypes.WrapError(err, "failed to decode aggregation count")
		}
		if len(countResult) > 0 {
			switch v := countResult[0]["total"].(type) {
			case int32:
				total = int64(v)
			case int64:
				total = v
			case float64:
				total = int64(v)
			default:
				total = int64(len(results))
			}
		}
	} else {
		total = int64(len(results))
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

	data, err := normalizeDocumentMap(request.Data)
	if err != nil {
		return 0, saiTypes.NewError("update data must be a map")
	}

	if setOp, exists := data["$set"]; exists {
		if setMap, ok := setOp.(map[string]interface{}); ok {
			delete(setMap, "internal_id")
			setMap["ch_time"] = time.Now().UnixNano() + atomic.AddInt64(&counter, 1)
			if request.Upsert {
				if setMap["internal_id"] == nil || setMap["internal_id"] == "" {
					setMap["internal_id"] = uuid.New().String()
				}
				if setMap["cr_time"] == nil {
					setMap["cr_time"] = setMap["ch_time"]
				}
			}
		}
	} else {
		setMap := data
		delete(setMap, "internal_id")
		setMap["ch_time"] = time.Now().UnixNano() + atomic.AddInt64(&counter, 1)
		if request.Upsert {
			if setMap["internal_id"] == nil || setMap["internal_id"] == "" {
				setMap["internal_id"] = uuid.New().String()
			}
			if setMap["cr_time"] == nil {
				setMap["cr_time"] = setMap["ch_time"]
			}
		}
		data = map[string]interface{}{"$set": setMap}
	}

	if setOnInsert, exists := data["$setOnInsert"]; exists {
		if setOnInsertMap, ok := setOnInsert.(map[string]interface{}); ok {
			delete(setOnInsertMap, "internal_id")
		}
	}

	result, err := coll.UpdateMany(ctx, request.Filter, data, _options)
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

func normalizeDocumentMap(data interface{}) (map[string]interface{}, error) {
	if data == nil {
		return nil, saiTypes.NewError("data must be a map")
	}

	if dataMap, ok := data.(map[string]interface{}); ok {
		return normalizeNestedMap(dataMap), nil
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return nil, saiTypes.WrapError(err, "failed to marshal document")
	}

	var dataMap map[string]interface{}
	if err := json.Unmarshal(raw, &dataMap); err != nil {
		return nil, saiTypes.NewError("data must be a map")
	}

	return normalizeNestedMap(dataMap), nil
}

func normalizeNestedMap(data map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{}, len(data))
	for key, value := range data {
		normalized[key] = normalizeNestedValue(value)
	}
	return normalized
}

func normalizeNestedValue(value interface{}) interface{} {
	raw, err := json.Marshal(value)
	if err != nil {
		return value
	}

	var normalized interface{}
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return value
	}

	return normalized
}
