package mongo

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DB struct {
	Client *mongo.Client
	Name   string
}

func Connect(ctx context.Context, uri string) (*DB, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	db := &DB{Client: client, Name: databaseFromURI(uri, "azula")}
	if err := db.ensureIndexes(ctx); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}
	return db, nil
}

func (d *DB) Database() *mongo.Database {
	return d.Client.Database(d.Name)
}

func (d *DB) Disconnect(ctx context.Context) error {
	return d.Client.Disconnect(ctx)
}

func (d *DB) ensureIndexes(ctx context.Context) error {
	_, err := d.Database().Collection("Users").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("users email index: %w", err)
	}

	_, err = d.Database().Collection("Workspaces").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "ownerId", Value: 1}},
	})
	if err != nil {
		return fmt.Errorf("workspaces owner index: %w", err)
	}

	_, err = d.Database().Collection("Projects").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "workspaceId", Value: 1}},
	})
	if err != nil {
		return fmt.Errorf("projects workspace index: %w", err)
	}

	_, err = d.Database().Collection("Investigations").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "projectId", Value: 1}, {Key: "createdAt", Value: -1}},
	})
	if err != nil {
		return fmt.Errorf("investigations index: %w", err)
	}
	_, err = d.Database().Collection("Workspaces").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "orgId", Value: 1}},
	})
	if err != nil {
		return fmt.Errorf("workspaces org index: %w", err)
	}

	_, err = d.Database().Collection("Organizations").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "members.email", Value: 1}},
	})
	if err != nil {
		return fmt.Errorf("org members email index: %w", err)
	}

	_, err = d.Database().Collection("FileVersions").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "projectId", Value: 1}, {Key: "fileName", Value: 1}, {Key: "version", Value: -1}},
	})
	if err != nil {
		return fmt.Errorf("fileVersions index: %w", err)
	}
	_, err = d.Database().Collection("AuditLogs").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}},
	})
	if err != nil {
		return fmt.Errorf("auditLogs index: %w", err)
	}
	_, err = d.Database().Collection("Generations").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "projectId", Value: 1}, {Key: "createdAt", Value: -1}},
	})
	if err != nil {
		return fmt.Errorf("generations index: %w", err)
	}
	_, err = d.Database().Collection("Evaluations").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "projectId", Value: 1}, {Key: "createdAt", Value: -1}},
	})
	if err != nil {
		return fmt.Errorf("evaluations index: %w", err)
	}
	return nil
}

func databaseFromURI(uri, fallback string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return fallback
	}
	name := strings.TrimPrefix(u.Path, "/")
	if name == "" {
		return fallback
	}
	return name
}
