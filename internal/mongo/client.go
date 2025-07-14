package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	saiTypes "github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-storage/types"
)

type Client struct {
	client   *mongo.Client
	database *mongo.Database
	config   types.StorageConfig
}

func NewClient(config types.StorageConfig) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Timeout)*time.Second)
	defer cancel()

	clientOptions := options.Client().
		ApplyURI(config.ConnectionString).
		SetMaxPoolSize(uint64(config.MaxPoolSize)).
		SetMinPoolSize(uint64(config.MinPoolSize)).
		SetMaxConnIdleTime(time.Duration(config.IdleTimeout) * time.Second).
		SetServerSelectionTimeout(time.Duration(config.SelectTimeout) * time.Second).
		SetSocketTimeout(time.Duration(config.SocketTimeout) * time.Second)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, saiTypes.WrapError(err, "failed to connect to MongoDB")
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, saiTypes.WrapError(err, "failed to ping MongoDB")
	}

	database := client.Database(config.Database)

	return &Client{
		client:   client,
		database: database,
		config:   config,
	}, nil
}

func (c *Client) GetCollection(name string) *mongo.Collection {
	return c.database.Collection(name)
}

func (c *Client) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx, readpref.Primary()); err != nil {
		return saiTypes.WrapError(err, "failed to ping MongoDB")
	}
	return nil
}

func (c *Client) Close(ctx context.Context) error {
	if err := c.client.Disconnect(ctx); err != nil {
		return saiTypes.WrapError(err, "failed to disconnect MongoDB")
	}
	return nil
}

func (c *Client) ListCollectionNames(ctx context.Context) ([]string, error) {
	names, err := c.database.ListCollectionNames(ctx, map[string]interface{}{})
	if err != nil {
		return nil, saiTypes.WrapError(err, "failed to list collection names")
	}
	return names, nil
}

func (c *Client) CreateIndexes(ctx context.Context, collectionName string, indexes []mongo.IndexModel) error {
	collection := c.GetCollection(collectionName)

	if len(indexes) == 0 {
		return nil
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return saiTypes.WrapError(err, "failed to create indexes")
	}

	return nil
}

func (c *Client) DropCollection(ctx context.Context, collectionName string) error {
	collection := c.GetCollection(collectionName)

	if err := collection.Drop(ctx); err != nil {
		return saiTypes.WrapError(err, "failed to drop collection")
	}

	return nil
}

func (c *Client) GetStats(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}

	err := c.database.RunCommand(ctx, map[string]interface{}{"dbStats": 1}).Decode(&result)
	if err != nil {
		return nil, saiTypes.WrapError(err, "failed to get stats")
	}

	return result, nil
}
