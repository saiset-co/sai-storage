package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/saiset-co/sai-service/sai"

	saiTypes "github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-storage/types"
)

type Repository struct {
	client *Client
}

func NewRepository() (types.StorageRepository, error) {
	var redisConfig types.RedisConfig
	err := sai.Config().GetAs("storage.redis", &redisConfig)
	if err != nil {
		return nil, err
	}

	client, err := NewClient(redisConfig)
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

	ids := make([]string, len(request.Data))
	now := time.Now().UnixNano()

	for i, data := range request.Data {
		// Generate internal_id if not provided
		dataMap, err := normalizeDocumentMap(data)
		if err != nil {
			return nil, err
		}

		var internalID string
		if existingID, exists := dataMap["internal_id"]; exists && existingID != nil {
			if existingIDStr, ok := existingID.(string); ok && existingIDStr != "" {
				internalID = existingIDStr
			}
		} else {
			internalID = uuid.New().String()
			dataMap["internal_id"] = internalID
		}
		dataMap["cr_time"] = now
		dataMap["ch_time"] = now

		// Convert to JSON for storage
		jsonData, err := json.Marshal(dataMap)
		if err != nil {
			return nil, saiTypes.WrapError(err, "failed to marshal document")
		}

		// Store document with key: collection:internal_id
		key := r.documentKey(request.Collection, internalID)

		// Check if TTL is specified in data
		var ttl time.Duration
		if ttlValue, exists := dataMap["ttl"]; exists {
			if ttlSeconds, ok := ttlValue.(float64); ok {
				ttl = time.Duration(ttlSeconds) * time.Second
			}
		}

		err = r.client.Set(ctx, key, jsonData, ttl)
		if err != nil {
			return nil, saiTypes.WrapError(err, "failed to store document")
		}

		// Add to collection index
		err = r.addToCollectionIndex(ctx, request.Collection, internalID)
		if err != nil {
			return nil, saiTypes.WrapError(err, "failed to add to collection index")
		}

		ids[i] = internalID
	}

	return ids, nil
}

func (r *Repository) ReadDocuments(ctx context.Context, request types.ReadDocumentsRequest) ([]map[string]interface{}, int64, error) {
	// Get all documents in collection
	pattern := r.documentPattern(request.Collection)
	keys, err := r.client.Keys(ctx, pattern)
	if err != nil {
		return nil, 0, saiTypes.WrapError(err, "failed to get collection keys")
	}

	var results []map[string]interface{}

	for _, key := range keys {
		jsonData, err := r.client.Get(ctx, key)
		if err != nil {
			continue // Skip missing keys (might have expired)
		}

		var doc map[string]interface{}
		if err := json.Unmarshal([]byte(jsonData), &doc); err != nil {
			continue // Skip malformed documents
		}

		// Apply filter if provided
		if request.Filter != nil && !r.matchesFilter(doc, request.Filter) {
			continue
		}

		// Apply field filtering if specified
		if len(request.Fields) > 0 {
			filteredDoc := make(map[string]interface{})
			for _, field := range request.Fields {
				if value, exists := doc[field]; exists {
					filteredDoc[field] = value
				}
			}
			results = append(results, filteredDoc)
		} else {
			results = append(results, doc)
		}
	}

	totalFull := int64(len(results))

	// Apply sorting if specified
	if request.Sort != nil && len(request.Sort) > 0 {
		r.sortDocuments(results, request.Sort)
	}

	// Apply pagination
	if request.Skip > 0 {
		if request.Skip >= len(results) {
			if request.Count > 0 {
				return []map[string]interface{}{}, totalFull, nil
			}
			return []map[string]interface{}{}, 0, nil
		}
		results = results[request.Skip:]
	}

	if request.Limit > 0 && request.Limit < len(results) {
		results = results[:request.Limit]
	}

	if request.Count > 0 {
		return results, totalFull, nil
	}

	return results, int64(len(results)), nil
}

func (r *Repository) AggregateDocuments(ctx context.Context, request types.AggregateDocumentsRequest) ([]map[string]interface{}, int64, error) {
	// Read all docs with filter for aggregation
	readRequest := types.ReadDocumentsRequest{
		Collection: request.Collection,
		Filter:     request.Filter,
	}

	docs, _, err := r.ReadDocuments(ctx, readRequest)
	if err != nil {
		return nil, 0, err
	}

	// If no aggregation specified, fall back to ReadDocuments semantics
	if len(request.GroupBy) == 0 && len(request.Aggregates) == 0 {
		readRequest.Fields = request.Fields
		readRequest.Sort = request.Sort
		readRequest.Limit = request.Limit
		readRequest.Skip = request.Skip
		return r.ReadDocuments(ctx, readRequest)
	}

	groups := make(map[string]*aggregateGroup)

	for _, doc := range docs {
		groupKey := "__all__"
		groupValues := make(map[string]interface{})

		if len(request.GroupBy) > 0 {
			keyParts := make([]string, 0, len(request.GroupBy))
			for _, field := range request.GroupBy {
				val, _ := r.getFieldValue(doc, field)
				groupValues[field] = val
				keyParts = append(keyParts, fmt.Sprintf("%v", val))
			}
			groupKey = strings.Join(keyParts, "|")
		}

		group := groups[groupKey]
		if group == nil {
			group = newAggregateGroup(groupValues, request.Aggregates)
			groups[groupKey] = group
		}

		group.apply(doc, r)
	}

	results := make([]map[string]interface{}, 0, len(groups))
	for _, group := range groups {
		results = append(results, group.result())
	}

	if request.Sort != nil && len(request.Sort) > 0 {
		r.sortDocuments(results, request.Sort)
	}

	totalFull := int64(len(results))

	// Apply pagination
	if request.Skip > 0 {
		if request.Skip >= len(results) {
			if request.Count > 0 {
				return []map[string]interface{}{}, totalFull, nil
			}
			return []map[string]interface{}{}, 0, nil
		}
		results = results[request.Skip:]
	}

	if request.Limit > 0 && request.Limit < len(results) {
		results = results[:request.Limit]
	}

	// Apply field filtering if specified
	if len(request.Fields) > 0 {
		filtered := make([]map[string]interface{}, 0, len(results))
		for _, doc := range results {
			item := make(map[string]interface{})
			for _, field := range request.Fields {
				if value, exists := doc[field]; exists {
					item[field] = value
				}
			}
			filtered = append(filtered, item)
		}
		results = filtered
	}

	if request.Count > 0 {
		return results, totalFull, nil
	}

	return results, int64(len(results)), nil
}

func (r *Repository) UpdateDocuments(ctx context.Context, request types.UpdateDocumentsRequest) (int64, error) {
	// Get documents to update
	readRequest := types.ReadDocumentsRequest{
		Collection: request.Collection,
		Filter:     request.Filter,
	}

	docs, _, err := r.ReadDocuments(ctx, readRequest)
	if err != nil {
		return 0, err
	}

	if len(docs) == 0 && !request.Upsert {
		return 0, nil
	}

	var updatedCount int64
	now := time.Now().UnixNano()

	// Handle upsert case
	if len(docs) == 0 && request.Upsert {
		// Create new document
		newDoc := make(map[string]interface{})

		// Apply update operations
		if err := r.applyUpdateOperations(newDoc, request.Data); err != nil {
			return 0, err
		}

		// Set internal_id if not already present
		if newDoc["internal_id"] == nil || newDoc["internal_id"].(string) == "" {
			newDoc["internal_id"] = uuid.New().String()
		}
		newDoc["cr_time"] = now
		newDoc["ch_time"] = now

		// Store new document
		jsonData, err := json.Marshal(newDoc)
		if err != nil {
			return 0, saiTypes.WrapError(err, "failed to marshal upserted document")
		}

		key := r.documentKey(request.Collection, newDoc["internal_id"].(string))
		err = r.client.Set(ctx, key, jsonData, 0)
		if err != nil {
			return 0, err
		}

		err = r.addToCollectionIndex(ctx, request.Collection, newDoc["internal_id"].(string))
		if err != nil {
			return 0, err
		}

		return 1, nil
	}

	// Update existing documents
	for _, doc := range docs {
		// Apply update operations
		if err := r.applyUpdateOperations(doc, request.Data); err != nil {
			continue
		}

		// Update timestamp
		doc["ch_time"] = now

		// Store updated document
		jsonData, err := json.Marshal(doc)
		if err != nil {
			continue
		}

		key := r.documentKey(request.Collection, doc["internal_id"].(string))
		err = r.client.Set(ctx, key, jsonData, 0)
		if err != nil {
			continue
		}

		updatedCount++
	}

	return updatedCount, nil
}

func (r *Repository) DeleteDocuments(ctx context.Context, request types.DeleteDocumentsRequest) (int64, error) {
	// Get documents to delete
	readRequest := types.ReadDocumentsRequest{
		Collection: request.Collection,
		Filter:     request.Filter,
	}

	docs, _, err := r.ReadDocuments(ctx, readRequest)
	if err != nil {
		return 0, err
	}

	var deletedCount int64

	for _, doc := range docs {
		internalID, ok := doc["internal_id"].(string)
		if !ok {
			continue
		}

		key := r.documentKey(request.Collection, internalID)
		err := r.client.Del(ctx, key)
		if err != nil {
			continue
		}

		// Remove from collection index
		err = r.removeFromCollectionIndex(ctx, request.Collection, internalID)
		if err != nil {
			continue
		}

		deletedCount++
	}

	return deletedCount, nil
}

func (r *Repository) Close(ctx context.Context) error {
	return r.client.Close()
}

// Helper methods

func (r *Repository) documentKey(collection, id string) string {
	return fmt.Sprintf("doc:%s:%s", collection, id)
}

func (r *Repository) documentPattern(collection string) string {
	return fmt.Sprintf("doc:%s:*", collection)
}

func (r *Repository) collectionIndexKey(collection string) string {
	return fmt.Sprintf("idx:%s", collection)
}

func (r *Repository) addToCollectionIndex(ctx context.Context, collection, id string) error {
	indexKey := r.collectionIndexKey(collection)
	return r.client.HSet(ctx, indexKey, id, time.Now().Unix())
}

func (r *Repository) removeFromCollectionIndex(ctx context.Context, collection, id string) error {
	indexKey := r.collectionIndexKey(collection)
	return r.client.HDel(ctx, indexKey, id)
}

func (r *Repository) matchesFilter(doc map[string]interface{}, filter map[string]interface{}) bool {
	for key, value := range filter {
		if !r.matchesField(doc, key, value) {
			return false
		}
	}
	return true
}

func (r *Repository) matchesField(doc map[string]interface{}, key string, filterValue interface{}) bool {
	// Handle nested keys (e.g., "user.id")
	keys := strings.Split(key, ".")
	current := doc

	for i, k := range keys {
		if i == len(keys)-1 {
			// Last key, compare value
			docValue, exists := current[k]
			if !exists {
				return false
			}
			return r.compareValues(docValue, filterValue)
		} else {
			// Navigate deeper
			next, exists := current[k]
			if !exists {
				return false
			}
			if nextMap, ok := next.(map[string]interface{}); ok {
				current = nextMap
			} else {
				return false
			}
		}
	}

	return false
}

func (r *Repository) compareValues(docValue, filterValue interface{}) bool {
	// Handle different comparison types
	switch filter := filterValue.(type) {
	case map[string]interface{}:
		// MongoDB-style operators
		for op, value := range filter {
			switch op {
			case "$eq":
				return docValue == value
			case "$ne":
				return docValue != value
			case "$gt":
				return r.compareNumbers(docValue, value, ">")
			case "$gte":
				return r.compareNumbers(docValue, value, ">=")
			case "$lt":
				return r.compareNumbers(docValue, value, "<")
			case "$lte":
				return r.compareNumbers(docValue, value, "<=")
			case "$in":
				if arr, ok := value.([]interface{}); ok {
					for _, v := range arr {
						if docValue == v {
							return true
						}
					}
				}
				return false
			case "$nin":
				if arr, ok := value.([]interface{}); ok {
					for _, v := range arr {
						if docValue == v {
							return false
						}
					}
				}
				return true
			}
		}
		return false
	default:
		// Direct equality comparison
		return docValue == filterValue
	}
}

func (r *Repository) compareNumbers(a, b interface{}, op string) bool {
	aVal, aOk := r.toFloat64(a)
	bVal, bOk := r.toFloat64(b)

	if !aOk || !bOk {
		return false
	}

	switch op {
	case ">":
		return aVal > bVal
	case ">=":
		return aVal >= bVal
	case "<":
		return aVal < bVal
	case "<=":
		return aVal <= bVal
	}
	return false
}

func (r *Repository) toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func (r *Repository) sortDocuments(docs []map[string]interface{}, sort map[string]int) {
	// Simple sorting implementation - for production, consider using a more robust sorting library
	// This is a basic implementation for demonstration
}

func (r *Repository) getFieldValue(doc map[string]interface{}, field string) (interface{}, bool) {
	keys := strings.Split(field, ".")
	current := doc

	for i, k := range keys {
		val, exists := current[k]
		if !exists {
			return nil, false
		}

		if i == len(keys)-1 {
			return val, true
		}

		next, ok := val.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current = next
	}

	return nil, false
}

type aggregateGroup struct {
	values map[string]interface{}
	aggs   map[string]*aggregateState
}

type aggregateState struct {
	op       string
	field    string
	count    int64
	sum      float64
	min      float64
	max      float64
	hasValue bool
}

func newAggregateGroup(values map[string]interface{}, aggregates []types.AggregateField) *aggregateGroup {
	aggs := make(map[string]*aggregateState)
	for _, agg := range aggregates {
		name := agg.As
		if name == "" {
			if agg.Op == "count" {
				name = "count"
			} else if agg.Field != "" {
				name = fmt.Sprintf("%s_%s", agg.Op, agg.Field)
			} else {
				name = agg.Op
			}
		}
		aggs[name] = &aggregateState{
			op:    strings.ToLower(agg.Op),
			field: agg.Field,
		}
	}

	return &aggregateGroup{
		values: values,
		aggs:   aggs,
	}
}

func (g *aggregateGroup) apply(doc map[string]interface{}, r *Repository) {
	for _, state := range g.aggs {
		switch state.op {
		case "count":
			state.count++
		case "sum", "avg", "min", "max":
			if state.field == "" {
				continue
			}
			val, ok := r.getFieldValue(doc, state.field)
			if !ok {
				continue
			}
			num, ok := r.toFloat64(val)
			if !ok {
				continue
			}
			state.count++
			state.sum += num
			if !state.hasValue {
				state.min = num
				state.max = num
				state.hasValue = true
				continue
			}
			if num < state.min {
				state.min = num
			}
			if num > state.max {
				state.max = num
			}
		}
	}
}

func (g *aggregateGroup) result() map[string]interface{} {
	out := make(map[string]interface{})
	for key, value := range g.values {
		out[key] = value
	}

	for name, state := range g.aggs {
		switch state.op {
		case "count":
			out[name] = state.count
		case "sum":
			out[name] = state.sum
		case "avg":
			if state.count == 0 {
				out[name] = 0
			} else {
				out[name] = state.sum / float64(state.count)
			}
		case "min":
			if state.hasValue {
				out[name] = state.min
			} else {
				out[name] = nil
			}
		case "max":
			if state.hasValue {
				out[name] = state.max
			} else {
				out[name] = nil
			}
		}
	}

	return out
}

func (r *Repository) applyUpdateOperations(doc map[string]interface{}, update interface{}) error {
	updateMap, err := normalizeDocumentMap(update)
	if err != nil {
		return saiTypes.NewError("update data must be a map")
	}

	for op, value := range updateMap {
		switch op {
		case "$set":
			if setMap, ok := value.(map[string]interface{}); ok {
				for key, val := range setMap {
					if key != "internal_id" {
						doc[key] = normalizeNestedValue(val)
					}
				}
			}
		case "$unset":
			if unsetMap, ok := value.(map[string]interface{}); ok {
				for key := range unsetMap {
					if key != "internal_id" && key != "cr_time" {
						delete(doc, key)
					}
				}
			}
		case "$inc":
			if incMap, ok := value.(map[string]interface{}); ok {
				for key, val := range incMap {
					if incVal, ok := r.toFloat64(val); ok {
						if current, exists := doc[key]; exists {
							if currentVal, ok := r.toFloat64(current); ok {
								doc[key] = currentVal + incVal
							}
						} else {
							doc[key] = incVal
						}
					}
				}
			}
		default:
			// Direct field assignment (protect internal_id)
			if op != "internal_id" {
				doc[op] = normalizeNestedValue(value)
			}
		}
	}

	return nil
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
