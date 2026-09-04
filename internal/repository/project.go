package repository

import (
	"context"
	"errors"
	"time"

	"github.com/ayse-solmaz/azula/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProjectRepo struct {
	col *mongo.Collection
}

func NewProjectRepo(db *mongo.Database) *ProjectRepo {
	return &ProjectRepo{col: db.Collection("Projects")}
}

type projectDoc struct {
	ID          string            `bson:"_id"`
	WorkspaceID string            `bson:"workspaceId"`
	Name        string            `bson:"name"`
	IsSample    bool              `bson:"isSample"`
	Files       []projectFileDoc  `bson:"files"`
	CreatedAt   time.Time         `bson:"createdAt"`
	UpdatedAt   time.Time         `bson:"updatedAt"`
}

type projectFileDoc struct {
	Name       string    `bson:"name"`
	Path       string    `bson:"path"`
	MimeType   string    `bson:"mimeType"`
	UploadedAt time.Time `bson:"uploadedAt"`
}

func toProject(d projectDoc) *domain.Project {
	files := make([]domain.ProjectFile, 0, len(d.Files))
	for _, f := range d.Files {
		files = append(files, domain.ProjectFile{Name: f.Name, Path: f.Path, MimeType: f.MimeType, UploadedAt: f.UploadedAt})
	}
	return &domain.Project{
		ID: d.ID, WorkspaceID: d.WorkspaceID, Name: d.Name, IsSample: d.IsSample,
		Files: files, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

func (r *ProjectRepo) Create(ctx context.Context, project *domain.Project) error {
	files := make([]projectFileDoc, 0, len(project.Files))
	for _, f := range project.Files {
		files = append(files, projectFileDoc{Name: f.Name, Path: f.Path, MimeType: f.MimeType, UploadedAt: f.UploadedAt})
	}
	_, err := r.col.InsertOne(ctx, projectDoc{
		ID: project.ID, WorkspaceID: project.WorkspaceID, Name: project.Name, IsSample: project.IsSample,
		Files: files, CreatedAt: project.CreatedAt, UpdatedAt: project.UpdatedAt,
	})
	return err
}

func (r *ProjectRepo) GetByID(ctx context.Context, id string) (*domain.Project, error) {
	var d projectDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toProject(d), nil
}

func (r *ProjectRepo) ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.Project, error) {
	cur, err := r.col.Find(ctx, bson.M{"workspaceId": workspaceID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.Project
	for cur.Next(ctx) {
		var d projectDoc
		if err := cur.Decode(&d); err != nil {
			return nil, err
		}
		out = append(out, *toProject(d))
	}
	if out == nil {
		out = []domain.Project{}
	}
	return out, cur.Err()
}

func (r *ProjectRepo) CountByWorkspaceIDs(ctx context.Context, workspaceIDs []string) (int64, error) {
	if len(workspaceIDs) == 0 {
		return 0, nil
	}
	return r.col.CountDocuments(ctx, bson.M{"workspaceId": bson.M{"$in": workspaceIDs}})
}

func (r *ProjectRepo) AddFile(ctx context.Context, projectID string, file domain.ProjectFile) (*domain.Project, error) {
	now := time.Now().UTC()
	fd := projectFileDoc{Name: file.Name, Path: file.Path, MimeType: file.MimeType, UploadedAt: file.UploadedAt}
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": projectID, "files.name": file.Name}, bson.M{
		"$set": bson.M{"files.$": fd, "updatedAt": now},
	})
	if err != nil {
		return nil, err
	}
	if res.MatchedCount == 0 {
		_, err = r.col.UpdateOne(ctx, bson.M{"_id": projectID}, bson.M{
			"$push": bson.M{"files": fd},
			"$set":  bson.M{"updatedAt": now},
		})
		if err != nil {
			return nil, err
		}
	}
	return r.GetByID(ctx, projectID)
}

func (r *ProjectRepo) DeleteByWorkspaceIDs(ctx context.Context, workspaceIDs []string) error {
	if len(workspaceIDs) == 0 {
		return nil
	}
	_, err := r.col.DeleteMany(ctx, bson.M{"workspaceId": bson.M{"$in": workspaceIDs}})
	return err
}
