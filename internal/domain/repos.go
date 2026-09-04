package domain

import "context"

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id string) error
}

type WorkspaceRepository interface {
	Create(ctx context.Context, ws *Workspace) error
	GetByID(ctx context.Context, id string) (*Workspace, error)
	ListByOwner(ctx context.Context, ownerID string) ([]Workspace, error)
	ListByOrg(ctx context.Context, orgID string) ([]Workspace, error)
	Update(ctx context.Context, ws *Workspace) error
	DeleteByOwner(ctx context.Context, ownerID string) error
}

type ProjectRepository interface {
	Create(ctx context.Context, project *Project) error
	GetByID(ctx context.Context, id string) (*Project, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]Project, error)
	CountByWorkspaceIDs(ctx context.Context, workspaceIDs []string) (int64, error)
	AddFile(ctx context.Context, projectID string, file ProjectFile) (*Project, error)
	DeleteByWorkspaceIDs(ctx context.Context, workspaceIDs []string) error
}

type InvestigationRepository interface {
	Create(ctx context.Context, inv *Investigation) error
	GetByID(ctx context.Context, id string) (*Investigation, error)
	ListByProject(ctx context.Context, projectID string) ([]Investigation, error)
	Update(ctx context.Context, inv *Investigation) error
	StatsByWorkspace(ctx context.Context, workspaceID string) (total, completed, failed int, avgConfidence float64, err error)
	DeleteByWorkspaceIDs(ctx context.Context, workspaceIDs []string) error
	ListByWorkspace(ctx context.Context, workspaceID string) ([]Investigation, error)
}

type ModelConfigRepository interface {
	GetByWorkspace(ctx context.Context, workspaceID string) (*ModelConfig, error)
	Upsert(ctx context.Context, cfg *ModelConfig) error
	DeleteByWorkspaceIDs(ctx context.Context, workspaceIDs []string) error
}

type AuditRepository interface {
	Insert(ctx context.Context, log *AuditLog) error
	ListByUser(ctx context.Context, userID string, limit int) ([]AuditLog, error)
	DeleteByUser(ctx context.Context, userID string) error
}

type ConsentRepository interface {
	Upsert(ctx context.Context, rec *ConsentRecord) error
	GetLatest(ctx context.Context, userID, purpose string) (*ConsentRecord, error)
	ListByUser(ctx context.Context, userID string) ([]ConsentRecord, error)
	DeleteByUser(ctx context.Context, userID string) error
}

type FileVersionRepository interface {
	Insert(ctx context.Context, v *FileVersion) error
	List(ctx context.Context, projectID, fileName string) ([]FileVersion, error)
	Get(ctx context.Context, projectID, fileName string, version int) (*FileVersion, error)
	NextVersion(ctx context.Context, projectID, fileName string) (int, error)
	DeleteByProjectIDs(ctx context.Context, projectIDs []string) error
}

type FineTuneRepository interface {
	Create(ctx context.Context, job *FineTuneJob) error
	Update(ctx context.Context, job *FineTuneJob) error
	GetByID(ctx context.Context, id string) (*FineTuneJob, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]FineTuneJob, error)
	DeleteByWorkspaceIDs(ctx context.Context, workspaceIDs []string) error
}

type OrganizationRepository interface {
	Create(ctx context.Context, org *Organization) error
	GetByID(ctx context.Context, id string) (*Organization, error)
	GetByMemberEmail(ctx context.Context, email string) (*Organization, error)
	Update(ctx context.Context, org *Organization) error
	DeleteByOwner(ctx context.Context, ownerID string) error
}
