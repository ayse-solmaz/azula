package repository

import (
	"context"
	"errors"
	"time"

	"github.com/ayse-solmaz/azula/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ModelConfigRepo struct {
	col *mongo.Collection
}

func NewModelConfigRepo(db *mongo.Database) *ModelConfigRepo {
	return &ModelConfigRepo{col: db.Collection("ModelConfigs")}
}

func (r *ModelConfigRepo) GetByWorkspace(ctx context.Context, workspaceID string) (*domain.ModelConfig, error) {
	var cfg domain.ModelConfig
	err := r.col.FindOne(ctx, bson.M{"workspaceId": workspaceID}).Decode(&cfg)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *ModelConfigRepo) Upsert(ctx context.Context, cfg *domain.ModelConfig) error {
	cfg.UpdatedAt = time.Now().UTC()
	_, err := r.col.ReplaceOne(ctx, bson.M{"workspaceId": cfg.WorkspaceID}, cfg, options.Replace().SetUpsert(true))
	return err
}

func (r *ModelConfigRepo) DeleteByWorkspaceIDs(ctx context.Context, workspaceIDs []string) error {
	if len(workspaceIDs) == 0 {
		return nil
	}
	_, err := r.col.DeleteMany(ctx, bson.M{"workspaceId": bson.M{"$in": workspaceIDs}})
	return err
}
