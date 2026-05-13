package types

import (
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson"
)

// OrderedPipeline preserves key order inside each aggregation stage.
// This avoids nondeterministic $sort key priority caused by map iteration.
type OrderedPipeline []bson.D

func (p *OrderedPipeline) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	out := make([]bson.D, 0, len(raw))
	for _, r := range raw {
		var doc bson.D
		if err := bson.UnmarshalExtJSON(r, false, &doc); err != nil {
			return err
		}
		out = append(out, doc)
	}
	*p = out
	return nil
}

func (p OrderedPipeline) MarshalJSON() ([]byte, error) {
	if p == nil {
		return []byte("null"), nil
	}
	raw := make([]json.RawMessage, 0, len(p))
	for _, doc := range p {
		b, err := bson.MarshalExtJSON(doc, false, false)
		if err != nil {
			return nil, err
		}
		raw = append(raw, json.RawMessage(b))
	}
	return json.Marshal(raw)
}

type CreateDocumentsRequest struct {
	Collection string        `json:"collection"`
	Data       []interface{} `json:"data"`
}

type ReadDocumentsRequest struct {
	Collection string                 `json:"collection" validate:"required"`
	Filter     map[string]interface{} `json:"filter,omitempty"`
	Sort       map[string]int         `json:"sort,omitempty"`
	Limit      int                    `json:"limit,omitempty"`
	Skip       int                    `json:"skip,omitempty"`
	Count      int                    `json:"count,omitempty"`
	Fields     []string               `json:"fields,omitempty"`
}

type UpdateDocumentsRequest struct {
	Collection string                 `json:"collection"`
	Filter     map[string]interface{} `json:"filter"`
	Data       interface{}            `json:"data"`
	Upsert     bool                   `json:"upsert,omitempty"`
}

type DeleteDocumentsRequest struct {
	Collection string                 `json:"collection"`
	Filter     map[string]interface{} `json:"filter"`
}

type AggregateField struct {
	Field string `json:"field,omitempty"`
	Op    string `json:"op"`
	As    string `json:"as,omitempty"`
}

type AggregateDocumentsRequest struct {
	Collection string                 `json:"collection" validate:"required"`
	Pipeline   OrderedPipeline        `json:"pipeline,omitempty"`
	Filter     map[string]interface{} `json:"filter,omitempty"`
	GroupBy    []string               `json:"group_by,omitempty"`
	Aggregates []AggregateField       `json:"aggregates,omitempty"`
	Sort       map[string]int         `json:"sort,omitempty"`
	Limit      int                    `json:"limit,omitempty"`
	Skip       int                    `json:"skip,omitempty"`
	Fields     []string               `json:"fields,omitempty"`
	Count      int                    `json:"count,omitempty"`
}
