package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ayse-solmaz/azula/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AuditRepo struct{ col *mongo.Collection }

func NewAuditRepo(db *mongo.Database) *AuditRepo {
	return &AuditRepo{col: db.Collection("AuditLogs")}
}

func (r *AuditRepo) Insert(ctx context.Context, log *domain.AuditLog) error {
	_, err := r.col.InsertOne(ctx, log)
	return err
}

func (r *AuditRepo) ListByUser(ctx context.Context, userID string, limit int) ([]domain.AuditLog, error) {
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(int64(limit))
	cur, err := r.col.Find(ctx, bson.M{"userId": userID}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.AuditLog
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []domain.AuditLog{}
	}
	return out, nil
}

func (r *AuditRepo) DeleteByUser(ctx context.Context, userID string) error {
	_, err := r.col.DeleteMany(ctx, bson.M{"userId": userID})
	return err
}

type ConsentRepo struct{ col *mongo.Collection }

func NewConsentRepo(db *mongo.Database) *ConsentRepo {
	return &ConsentRepo{col: db.Collection("ConsentRecords")}
}

func (r *ConsentRepo) Upsert(ctx context.Context, rec *domain.ConsentRecord) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"userId": rec.UserID, "purpose": rec.Purpose}, rec, options.Replace().SetUpsert(true))
	return err
}

func (r *ConsentRepo) GetLatest(ctx context.Context, userID, purpose string) (*domain.ConsentRecord, error) {
	var rec domain.ConsentRecord
	err := r.col.FindOne(ctx, bson.M{"userId": userID, "purpose": purpose}).Decode(&rec)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *ConsentRepo) ListByUser(ctx context.Context, userID string) ([]domain.ConsentRecord, error) {
	cur, err := r.col.Find(ctx, bson.M{"userId": userID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.ConsentRecord
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []domain.ConsentRecord{}
	}
	return out, nil
}

func (r *ConsentRepo) DeleteByUser(ctx context.Context, userID string) error {
	_, err := r.col.DeleteMany(ctx, bson.M{"userId": userID})
	return err
}

type FileVersionRepo struct{ col *mongo.Collection }

func NewFileVersionRepo(db *mongo.Database) *FileVersionRepo {
	return &FileVersionRepo{col: db.Collection("FileVersions")}
}

func (r *FileVersionRepo) Insert(ctx context.Context, v *domain.FileVersion) error {
	_, err := r.col.InsertOne(ctx, v)
	return err
}

func (r *FileVersionRepo) List(ctx context.Context, projectID, fileName string) ([]domain.FileVersion, error) {
	opts := options.Find().SetSort(bson.D{{Key: "version", Value: -1}})
	cur, err := r.col.Find(ctx, bson.M{"projectId": projectID, "fileName": fileName}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.FileVersion
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []domain.FileVersion{}
	}
	return out, nil
}

func (r *FileVersionRepo) Get(ctx context.Context, projectID, fileName string, version int) (*domain.FileVersion, error) {
	var v domain.FileVersion
	err := r.col.FindOne(ctx, bson.M{"projectId": projectID, "fileName": fileName, "version": version}).Decode(&v)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrVersionNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *FileVersionRepo) NextVersion(ctx context.Context, projectID, fileName string) (int, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}})
	var v domain.FileVersion
	err := r.col.FindOne(ctx, bson.M{"projectId": projectID, "fileName": fileName}, opts).Decode(&v)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	return v.Version + 1, nil
}

func (r *FileVersionRepo) DeleteByProjectIDs(ctx context.Context, projectIDs []string) error {
	if len(projectIDs) == 0 {
		return nil
	}
	_, err := r.col.DeleteMany(ctx, bson.M{"projectId": bson.M{"$in": projectIDs}})
	return err
}

type FineTuneRepo struct{ col *mongo.Collection }

func NewFineTuneRepo(db *mongo.Database) *FineTuneRepo {
	return &FineTuneRepo{col: db.Collection("FineTuneJobs")}
}

func (r *FineTuneRepo) Create(ctx context.Context, job *domain.FineTuneJob) error {
	_, err := r.col.InsertOne(ctx, job)
	return err
}

func (r *FineTuneRepo) GetByID(ctx context.Context, id string) (*domain.FineTuneJob, error) {
	var job domain.FineTuneJob
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&job)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *FineTuneRepo) Update(ctx context.Context, job *domain.FineTuneJob) error {
	job.UpdatedAt = time.Now().UTC()
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": job.ID}, job)
	return err
}

func (r *FineTuneRepo) ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.FineTuneJob, error) {
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cur, err := r.col.Find(ctx, bson.M{"workspaceId": workspaceID}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []domain.FineTuneJob
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []domain.FineTuneJob{}
	}
	return out, nil
}

func (r *FineTuneRepo) DeleteByWorkspaceIDs(ctx context.Context, workspaceIDs []string) error {
	if len(workspaceIDs) == 0 {
		return nil
	}
	_, err := r.col.DeleteMany(ctx, bson.M{"workspaceId": bson.M{"$in": workspaceIDs}})
	return err
}

type OrgRepo struct{ col *mongo.Collection }

func NewOrgRepo(db *mongo.Database) *OrgRepo {
	return &OrgRepo{col: db.Collection("Organizations")}
}

func (r *OrgRepo) Create(ctx context.Context, org *domain.Organization) error {
	_, err := r.col.InsertOne(ctx, org)
	return err
}

func (r *OrgRepo) GetByID(ctx context.Context, id string) (*domain.Organization, error) {
	var org domain.Organization
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&org)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &org, nil
}

func (r *OrgRepo) GetByMemberEmail(ctx context.Context, email string) (*domain.Organization, error) {
	var org domain.Organization
	err := r.col.FindOne(ctx, bson.M{"members.email": strings.ToLower(email)}).Decode(&org)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &org, nil
}

func (r *OrgRepo) Update(ctx context.Context, org *domain.Organization) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": org.ID}, org)
	return err
}

func (r *OrgRepo) DeleteByOwner(ctx context.Context, ownerID string) error {
	_, err := r.col.DeleteMany(ctx, bson.M{"ownerId": ownerID})
	return err
}
