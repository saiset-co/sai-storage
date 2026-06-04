package types

import "time"

type CollectionStats struct {
	Name        string `json:"name"`
	Count       int64  `json:"count"`
	StorageSize int64  `json:"storage_size"`
	IndexSize   int64  `json:"index_size"`
	NumIndexes  int    `json:"num_indexes"`
}

type IndexInfo struct {
	Name   string         `json:"name"`
	Fields map[string]int `json:"fields"`
	Unique bool           `json:"unique"`
	Sparse bool           `json:"sparse"`
}

type SlowQuery struct {
	Op           string    `json:"op"`
	Namespace    string    `json:"namespace"`
	Collection   string    `json:"collection"`
	DurationMs   int64     `json:"duration_ms"`
	KeysExamined int64     `json:"keys_examined"`
	DocsExamined int64     `json:"docs_examined"`
	PlanSummary  string    `json:"plan_summary"`
	FilterKeys   []string  `json:"filter_keys"`
	Timestamp    time.Time `json:"timestamp"`
}

type QueryStat struct {
	InternalID        string   `json:"internal_id"`
	Collection        string   `json:"collection"`
	Operation         string   `json:"operation"`
	FilterFingerprint string   `json:"filter_fingerprint"`
	FilterKeys        []string `json:"filter_keys"`
	Count             int64    `json:"count"`
	LastSeen          int64    `json:"last_seen"`
}

type CustomQuery struct {
	InternalID  string                 `json:"internal_id"`
	Name        string                 `json:"name"`
	Collection  string                 `json:"collection"`
	Operation   string                 `json:"operation"`
	Body        map[string]interface{} `json:"body"`
	Description string                 `json:"description"`
	CrTime      int64                  `json:"cr_time"`
}

type CreateIndexRequest struct {
	Collection string         `json:"collection"`
	Keys       map[string]int `json:"keys"`
	Unique     bool           `json:"unique"`
	Sparse     bool           `json:"sparse"`
	Name       string         `json:"name"`
}

type RestoreRequest struct {
	Collection         string `json:"collection"`
	ArchiveOperationID string `json:"archive_operation_id"`
}

type ArchiveGroup struct {
	OperationID string      `json:"operation_id"`
	ArchiveTime int64       `json:"archive_time"`
	Count       int64       `json:"count"`
	Filter      interface{} `json:"filter,omitempty"`
	Update      interface{} `json:"update,omitempty"`
	RestoredAt  int64       `json:"restored_at,omitempty"`
}
