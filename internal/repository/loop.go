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

type GenerationRepo struct{ col *mongo.Collection }

func NewGenerationRepo(db *mongo.Database) *GenerationRepo {
	return &GenerationRepo{col: db.Collection("Generations")}
}

func (r *GenerationRepo) Create(ctx context.Context, g *domain.Generation) error {
	_, err := r.col.InsertOne(ctx, g)
	return err
}

func (r *GenerationRepo) Update(ctx context.Context, g *domain.Generation) error {
	g.UpdatedAt = time.Now().UTC()
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": g.ID}, g)
	return err
}

func (r *GenerationRepo) GetByID(ctx context.Context, id string) (*domain.Generation, error) {
	var g domain.Generation
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&g)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *GenerationRepo) ListByProject(ctx context.Context, projectID string) ([]domain.Generation, error) {
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cur, err := r.col.Find(ctx, bson.M{"projectId": projectID}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.Generation
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []domain.Generation{}
	}
	return out, nil
}

func (r *GenerationRepo) DeleteByWorkspaceIDs(ctx context.Context, workspaceIDs []string) error {
	if len(workspaceIDs) == 0 {
		return nil
	}
	_, err := r.col.DeleteMany(ctx, bson.M{"workspaceId": bson.M{"$in": workspaceIDs}})
	return err
}

type EvaluationRepo struct{ col *mongo.Collection }

func NewEvaluationRepo(db *mongo.Database) *EvaluationRepo {
	return &EvaluationRepo{col: db.Collection("Evaluations")}
}

func (r *EvaluationRepo) Create(ctx context.Context, e *domain.Evaluation) error {
	_, err := r.col.InsertOne(ctx, e)
	return err
}

func (r *EvaluationRepo) Update(ctx context.Context, e *domain.Evaluation) error {
	e.UpdatedAt = time.Now().UTC()
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": e.ID}, e)
	return err
}

func (r *EvaluationRepo) GetByID(ctx context.Context, id string) (*domain.Evaluation, error) {
	var e domain.Evaluation
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&e)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *EvaluationRepo) ListByProject(ctx context.Context, projectID string) ([]domain.Evaluation, error) {
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cur, err := r.col.Find(ctx, bson.M{"projectId": projectID}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.Evaluation
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []domain.Evaluation{}
	}
	return out, nil
}

func (r *EvaluationRepo) DeleteByWorkspaceIDs(ctx context.Context, workspaceIDs []string) error {
	if len(workspaceIDs) == 0 {
		return nil
	}
	_, err := r.col.DeleteMany(ctx, bson.M{"workspaceId": bson.M{"$in": workspaceIDs}})
	return err
}
