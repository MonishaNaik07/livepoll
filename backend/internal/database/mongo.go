package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectMongo(ctx context.Context, uri, dbName string) (*mongo.Client, *mongo.Database, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetServerSelectionTimeout(5*time.Second))
	if err != nil {
		return nil, nil, fmt.Errorf("mongo connect: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, nil, fmt.Errorf("mongo ping: %w", err)
	}
	db := client.Database(dbName)
	if err := EnsureIndexes(ctx, db); err != nil {
		return nil, nil, err
	}
	return client, db, nil
}

// EnsureIndexes is idempotent and runs on every boot.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection("users").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true).SetName("uniq_email"),
	})
	if err != nil {
		return fmt.Errorf("users index: %w", err)
	}
	_, err = db.Collection("polls").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "ownerId", Value: 1}, {Key: "createdAt", Value: -1}}, Options: options.Index().SetName("owner_created"),
	})
	if err != nil {
		return fmt.Errorf("polls index: %w", err)
	}
	_, err = db.Collection("votes").Indexes().CreateMany(ctx, []mongo.IndexModel{
		// Duplicate-vote guarantee: one vote per voter per poll, enforced by the DB.
		{Keys: bson.D{{Key: "pollId", Value: 1}, {Key: "voterId", Value: 1}}, Options: options.Index().SetUnique(true).SetName("uniq_poll_voter")},
		{Keys: bson.D{{Key: "pollId", Value: 1}, {Key: "optionId", Value: 1}}, Options: options.Index().SetName("poll_option")},
		{Keys: bson.D{{Key: "applied", Value: 1}, {Key: "createdAt", Value: 1}}, Options: options.Index().SetName("unapplied_scan")},
	})
	if err != nil {
		return fmt.Errorf("votes indexes: %w", err)
	}
	return nil
}
