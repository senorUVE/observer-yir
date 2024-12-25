package observer

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/exp/slog"
)

type Observer struct {
	client     *mongo.Client
	logging    *slog.Logger
	db         *mongo.Database
	collection *mongo.Collection
}

type ErrorLog struct {
	RequestID string    `bson:"request_id"`
	ErrorMsg  string    `bson:"error_msg"`
	Timestamp time.Time `bson:"timestamp"`
	Details   string    `bson:"details"`
}

func NewObserver(mongoURI, dbName, collectionName string) (*Observer, error) {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	db := client.Database(dbName)
	collection := db.Collection(collectionName)

	logging := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{}))

	return &Observer{
		client:     client,
		db:         db,
		collection: collection,
		logging:    logging,
	}, nil
}

func (o *Observer) PingMongo() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := o.client.Ping(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}
	return nil
}
