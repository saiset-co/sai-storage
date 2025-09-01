package types

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
