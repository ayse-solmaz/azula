package repository

import (
	"context"
	"errors"
	"time"

	"github.com/ayse-solmaz/azula/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type WorkspaceRepo struct {
	col *mongo.Collection
}

func NewWorkspaceRepo(db *mongo.Database) *WorkspaceRepo {
	return &WorkspaceRepo{col: db.Collection("Workspaces")}
}

type workspaceDoc struct {
	ID        string    `bson:"_id"`
	Name      string    `bson:"name"`
	OwnerID   string    `bson:"ownerId"`
	OrgID     string    `bson:"orgId,omitempty"`
	CreatedAt time.Time `bson:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt"`
}

func toWorkspace(d workspaceDoc) domain.Workspace {
	return domain.Workspace{ID: d.ID, Name: d.Name, OwnerID: d.OwnerID, OrgID: d.OrgID, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt}
}

func (r *WorkspaceRepo) Create(ctx context.Context, ws *domain.Workspace) error {
	_, err := r.col.InsertOne(ctx, workspaceDoc{
		ID: ws.ID, Name: ws.Name, OwnerID: ws.OwnerID, OrgID: ws.OrgID, CreatedAt: ws.CreatedAt, UpdatedAt: ws.UpdatedAt,
	})
	return err
}

func (r *WorkspaceRepo) GetByID(ctx context.Context, id string) (*domain.Workspace, error) {
	var d workspaceDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	w := toWorkspace(d)
	return &w, nil
}

func (r *WorkspaceRepo) ListByOwner(ctx context.Context, ownerID string) ([]domain.Workspace, error) {
	return r.list(ctx, bson.M{"ownerId": ownerID})
}

func (r *WorkspaceRepo) ListByOrg(ctx context.Context, orgID string) ([]domain.Workspace, error) {
	if orgID == "" {
		return []domain.Workspace{}, nil
	}
	return r.list(ctx, bson.M{"orgId": orgID})
}

func (r *WorkspaceRepo) list(ctx context.Context, filter bson.M) ([]domain.Workspace, error) {
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.Workspace
	for cur.Next(ctx) {
		var d workspaceDoc
		if err := cur.Decode(&d); err != nil {
			return nil, err
		}
		out = append(out, toWorkspace(d))
	}
	if out == nil {
		out = []domain.Workspace{}
	}
	return out, cur.Err()
}

func (r *WorkspaceRepo) Update(ctx context.Context, ws *domain.Workspace) error {
	ws.UpdatedAt = time.Now().UTC()
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": ws.ID}, workspaceDoc{
		ID: ws.ID, Name: ws.Name, OwnerID: ws.OwnerID, OrgID: ws.OrgID, CreatedAt: ws.CreatedAt, UpdatedAt: ws.UpdatedAt,
	})
	return err
}

func (r *WorkspaceRepo) DeleteByOwner(ctx context.Context, ownerID string) error {
	_, err := r.col.DeleteMany(ctx, bson.M{"ownerId": ownerID})
	return err
}
