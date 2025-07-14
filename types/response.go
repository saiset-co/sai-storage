package types

type CreateDocumentsResponse struct {
	Data    []string `json:"data"`
	Created int      `json:"created"`
}

type ReadDocumentsResponse struct {
	Data  []map[string]interface{} `json:"data"`
	Total int64                    `json:"total"`
}

type UpdateDocumentsResponse struct {
	Data    []string `json:"data"`
	Updated int64    `json:"updated"`
}

type DeleteDocumentsResponse struct {
	Data    []string `json:"data"`
	Deleted int64    `json:"deleted"`
}
