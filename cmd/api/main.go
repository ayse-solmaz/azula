package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/ayse-solmaz/azula/graph"
	"github.com/ayse-solmaz/azula/internal/auth"
	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/finetune"
	"github.com/ayse-solmaz/azula/internal/gdpr"
	"github.com/ayse-solmaz/azula/internal/httpx"
	"github.com/ayse-solmaz/azula/internal/investigation"
	"github.com/ayse-solmaz/azula/internal/llm"
	"github.com/ayse-solmaz/azula/internal/mail"
	"github.com/ayse-solmaz/azula/internal/mcp"
	azmongo "github.com/ayse-solmaz/azula/internal/mongo"
	"github.com/ayse-solmaz/azula/internal/org"
	"github.com/ayse-solmaz/azula/internal/projectsvc"
	"github.com/ayse-solmaz/azula/internal/repository"
	"github.com/joho/godotenv"
	"github.com/vektah/gqlparser/v2/ast"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := azmongo.Connect(ctx, cfg.MongoURI)
	if err != nil {
		log.Fatalf("mongodb: %v", err)
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = db.Disconnect(c)
	}()

	users := repository.NewUserRepo(db.Database())
	workspaces := repository.NewWorkspaceRepo(db.Database())
	projects := repository.NewProjectRepo(db.Database())
	invs := repository.NewInvestigationRepo(db.Database())
	modelCfgs := repository.NewModelConfigRepo(db.Database())
	audits := repository.NewAuditRepo(db.Database())
	consents := repository.NewConsentRepo(db.Database())
	versions := repository.NewFileVersionRepo(db.Database())
	jobs := repository.NewFineTuneRepo(db.Database())
	orgs := repository.NewOrgRepo(db.Database())
	files := mcp.NewFilesConnector(cfg.MCPFileRoot)
	router := llm.NewRouter(cfg)
	invSvc := investigation.New(projects, invs, modelCfgs, files, router, cfg)
	authSvc := auth.NewWithAudit(users, workspaces, cfg, audits, mail.New(cfg))
	orgSvc := org.New(orgs, users, workspaces)
	authSvc.SetJoiner(orgSvc)
	projectSvc := projectsvc.New(workspaces, projects, files, cfg.FreeTierMaxProjects, cfg.SamplePipeline)
	projectSvc.SetAccess(orgSvc)
	gdprSvc := gdpr.New(users, workspaces, projects, invs, modelCfgs, audits, consents, versions, jobs, orgs, files)
	tuneSvc := finetune.New(jobs, modelCfgs, cfg)

	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: &graph.Resolver{
		Cfg:          cfg,
		Auth:         authSvc,
		Spaces:       workspaces,
		Projects:     projects,
		ProjectSvc:   projectSvc,
		Inv:          invSvc,
		MCP:          files,
		ModelConfigs: modelCfgs,
		Router:       router,
		GDPR:         gdprSvc,
		Tune:         tuneSvc,
		Org:          orgSvc,
	}}))
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{
		MaxMemory:     50 << 20,
		MaxUploadSize: 50 << 20,
	})
	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{Cache: lru.New[string](100)})

	mux := http.NewServeMux()
	mux.Handle("/graphql", httpx.RateLimit(120, auth.Middleware(cfg.JWTSecret, srv)))
	mux.Handle("/", playground.Handler("Azula GraphQL", "/graphql"))
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	httpSrv := &http.Server{
		Addr:              ":" + cfg.APIPort,
		Handler:           withCORS(cfg.WebURL, mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      4 * time.Minute,
	}

	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(c)
	}()

	log.Printf("azula api listening on :%s modelA=%s modelB=%s slots=%d", cfg.APIPort, cfg.ModelAName, cfg.ModelBName, cfg.WorkerSlots)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func withCORS(webURL string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if originAllowed(webURL, origin) {
			allow := origin
			if allow == "" {
				allow = "null"
			}
			w.Header().Set("Access-Control-Allow-Origin", allow)
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func originAllowed(webURL, origin string) bool {
	if origin == webURL || origin == "http://localhost:3000" || origin == "http://localhost:5173" {
		return true
	}
	// Packaged Electron loadFile uses a null / file origin while calling the local API.
	if origin == "" || origin == "null" || strings.HasPrefix(origin, "file://") {
		return true
	}
	return false
}
