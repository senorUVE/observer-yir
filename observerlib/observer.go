package observer

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/exp/slog"
	"google.golang.org/grpc"
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

func (o *Observer) LogError(ctx context.Context, requestID, errorMsg, details string) error {
	logEntry := ErrorLog{
		RequestID: requestID,
		ErrorMsg:  errorMsg,
		Timestamp: time.Now(),
		Details:   details,
	}

	_, err := o.collection.InsertOne(ctx, logEntry)
	if err != nil {
		o.logging.Error("Error logging to MongoDB", "error", err)
		return err
	}
	return nil
}

func (o *Observer) LogMetrics(ctx context.Context, requestID string, service string, duration time.Duration, statusCode int) {
	metrics := bson.M{
		"request_id":  requestID,
		"service":     service,
		"duration":    duration.Seconds(),
		"status_code": statusCode,
		"timestamp":   time.Now(),
	}

	_, err := o.collection.InsertOne(ctx, metrics)
	if err != nil {
		o.logging.Error("Error logging metrics to MongoDB", "error", err)
	}
}

func (o *Observer) ObserverMiddleware() func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		requestID := fmt.Sprintf("%d", start.UnixNano())
		resp, err := handler(ctx, req)
		if err != nil {
			o.LogError(ctx, requestID, err.Error(), fmt.Sprintf("Request failed: %v", err))
		}
		o.LogMetrics(ctx, requestID, info.FullMethod, time.Since(start), 200)
		return resp, err
	}
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
