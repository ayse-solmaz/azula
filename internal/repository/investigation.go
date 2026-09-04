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

type InvestigationRepo struct {
	col *mongo.Collection
}

func NewInvestigationRepo(db *mongo.Database) *InvestigationRepo {
	return &InvestigationRepo{col: db.Collection("Investigations")}
}

func (r *InvestigationRepo) Create(ctx context.Context, inv *domain.Investigation) error {
	_, err := r.col.InsertOne(ctx, inv)
	return err
}

func (r *InvestigationRepo) GetByID(ctx context.Context, id string) (*domain.Investigation, error) {
	var inv domain.Investigation
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&inv)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *InvestigationRepo) ListByProject(ctx context.Context, projectID string) ([]domain.Investigation, error) {
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cur, err := r.col.Find(ctx, bson.M{"projectId": projectID}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.Investigation
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []domain.Investigation{}
	}
	return out, nil
}

func (r *InvestigationRepo) Update(ctx context.Context, inv *domain.Investigation) error {
	inv.UpdatedAt = time.Now().UTC()
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": inv.ID}, inv)
	return err
}

func (r *InvestigationRepo) StatsByWorkspace(ctx context.Context, workspaceID string) (int, int, int, float64, error) {
	cur, err := r.col.Find(ctx, bson.M{"workspaceId": workspaceID})
	if err != nil {
		return 0, 0, 0, 0, err
	}
	defer cur.Close(ctx)
	total, completed, failed := 0, 0, 0
	sum := 0.0
	nConf := 0
	for cur.Next(ctx) {
		var inv domain.Investigation
		if err := cur.Decode(&inv); err != nil {
			return 0, 0, 0, 0, err
		}
		total++
		switch inv.Status {
		case domain.StatusCompleted:
			completed++
			if inv.CouncilResult != nil {
				sum += inv.CouncilResult.FinalJudgment.Confidence
				nConf++
			}
		case domain.StatusFailed:
			failed++
		}
	}
	avg := 0.0
	if nConf > 0 {
		avg = sum / float64(nConf)
	}
	return total, completed, failed, avg, cur.Err()
}

func (r *InvestigationRepo) ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.Investigation, error) {
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cur, err := r.col.Find(ctx, bson.M{"workspaceId": workspaceID}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.Investigation
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []domain.Investigation{}
	}
	return out, nil
}

func (r *InvestigationRepo) DeleteByWorkspaceIDs(ctx context.Context, workspaceIDs []string) error {
	if len(workspaceIDs) == 0 {
		return nil
	}
	_, err := r.col.DeleteMany(ctx, bson.M{"workspaceId": bson.M{"$in": workspaceIDs}})
	return err
}
