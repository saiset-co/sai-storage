package types

type StorageConfig struct {
	ConnectionString string `yaml:"connection_string" json:"connection_string"`
	Database         string `yaml:"database" json:"database"`
	Timeout          int    `yaml:"timeout" json:"timeout"`
	MaxPoolSize      int    `yaml:"max_pool_size" json:"max_pool_size"`
	MinPoolSize      int    `yaml:"min_pool_size" json:"min_pool_size"`
	SelectTimeout    int    `yaml:"select_timeout" json:"select_timeout"`
	IdleTimeout      int    `yaml:"idle_timeout" json:"idle_timeout"`
	SocketTimeout    int    `yaml:"socket_timeout" json:"socket_timeout"`
}
