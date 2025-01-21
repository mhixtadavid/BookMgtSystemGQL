package database

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var DB *mongo.Database

func Connect() (*mongo.Client, error) {
	mongoURI := os.Getenv("MONGO_DB_URI")
	if mongoURI == "" {
		return nil, fmt.Errorf("MONGO_DB_URI is not set")
	}

	databaseName := os.Getenv("DATABASE")
	if databaseName == "" {
		return nil, fmt.Errorf("DATABASE name is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	DB = client.Database(databaseName)
	fmt.Println("Successfully connected to the database")
	return client, nil
}
