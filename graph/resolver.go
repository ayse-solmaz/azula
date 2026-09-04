package graph

import (
	"github.com/ayse-solmaz/azula/internal/auth"
	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/domain"
	"github.com/ayse-solmaz/azula/internal/finetune"
	"github.com/ayse-solmaz/azula/internal/gdpr"
	"github.com/ayse-solmaz/azula/internal/investigation"
	"github.com/ayse-solmaz/azula/internal/llm"
	"github.com/ayse-solmaz/azula/internal/mcp"
	"github.com/ayse-solmaz/azula/internal/org"
	"github.com/ayse-solmaz/azula/internal/projectsvc"
)

type Resolver struct {
	Cfg          config.Config
	Auth         *auth.Service
	Spaces       domain.WorkspaceRepository
	Projects     domain.ProjectRepository
	ProjectSvc   *projectsvc.Service
	Inv          *investigation.Service
	MCP          mcp.Connector
	ModelConfigs domain.ModelConfigRepository
	Router       *llm.Router
	GDPR         *gdpr.Service
	Tune         *finetune.Service
	Org          *org.Service
}
