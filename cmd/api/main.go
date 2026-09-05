package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
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
	"github.com/ayse-solmaz/azula/internal/billing"
	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/finetune"
	"github.com/ayse-solmaz/azula/internal/gdpr"
	"github.com/ayse-solmaz/azula/internal/httpx"
	"github.com/ayse-solmaz/azula/internal/investigation"
	"github.com/ayse-solmaz/azula/internal/llm"
	"github.com/ayse-solmaz/azula/internal/loop"
	"github.com/ayse-solmaz/azula/internal/mail"
	"github.com/ayse-solmaz/azula/internal/mcp"
	azmongo "github.com/ayse-solmaz/azula/internal/mongo"
	"github.com/ayse-solmaz/azula/internal/org"
	"github.com/ayse-solmaz/azula/internal/projectsvc"
	"github.com/ayse-solmaz/azula/internal/repository"
	"github.com/ayse-solmaz/azula/internal/sso"
	"github.com/joho/godotenv"
	"github.com/vektah/gqlparser/v2/ast"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config: %v", err)
	}

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
	gens := repository.NewGenerationRepo(db.Database())
	evals := repository.NewEvaluationRepo(db.Database())
	files := mcp.NewFilesConnector(cfg.MCPFileRoot)
	git := mcp.NewGit(files)
	router := llm.NewRouter(cfg)
	invSvc := investigation.New(projects, invs, modelCfgs, files, router, cfg)
	authSvc := auth.NewWithAudit(users, workspaces, cfg, audits, mail.New(cfg))
	orgSvc := org.New(orgs, users, workspaces)
	authSvc.SetJoiner(orgSvc)
	billSvc := billing.New(cfg, users, invs)
	invSvc.SetGate(billSvc)
	invSvc.SetAudit(audits)
	projectSvc := projectsvc.New(workspaces, projects, files, cfg.FreeTierMaxProjects, cfg.SamplePipeline)
	projectSvc.SetAccess(orgSvc)
	projectSvc.SetCaps(billSvc)
	projectSvc.SetGit(git)
	gdprSvc := gdpr.New(users, workspaces, projects, invs, modelCfgs, audits, consents, versions, jobs, orgs, files)
	tuneSvc := finetune.New(jobs, modelCfgs, cfg)
	loopSvc := loop.New(projects, invs, gens, evals, files, router, invSvc)
	ssoH := sso.New(cfg, authSvc)

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
		Billing:      billSvc,
		Loop:         loopSvc,
		Git:          git,
	}}))
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{
		MaxMemory:     50 << 20,
		MaxUploadSize: 50 << 20,
	})
	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	srv.Use(extension.FixedComplexityLimit(400))
	if cfg.GraphQLPlayground {
		srv.Use(extension.Introspection{})
	}
	srv.Use(extension.AutomaticPersistedQuery{Cache: lru.New[string](100)})

	mux := http.NewServeMux()
	gql := httpx.RateLimit(120, httpx.AuthOpLimit(20, httpx.RestoreGraphQLCamelCase(auth.Middleware(cfg.JWTSecret, srv))))
	mux.Handle("/graphql", gql)
	if cfg.GraphQLPlayground {
		mux.Handle("/", playground.Handler("Azula GraphQL", "/graphql"))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			http.NotFound(w, nil)
		})
	}
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/auth/oidc/start", ssoH.Start)
	mux.HandleFunc("/auth/oidc/callback", ssoH.Callback)
	mux.HandleFunc("/billing/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}
		if err := billSvc.HandleWebhook(r.Context(), body, r.Header.Get("Stripe-Signature")); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received":true}`))
	})

	httpSrv := &http.Server{
		Addr:              ":" + cfg.APIPort,
		Handler:           httpx.SecurityHeaders(httpx.MaxBodyBytes(2<<20, withCORS(cfg.WebURL, mux))),
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
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			if origin != "" && origin != "null" && !strings.HasPrefix(origin, "file://") {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func originAllowed(webURL, origin string) bool {
	if origin == webURL ||
		origin == "http://localhost:3000" || origin == "http://127.0.0.1:3000" ||
		origin == "http://localhost:3001" || origin == "http://127.0.0.1:3001" ||
		origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" {
		return true
	}
	if webURL != "" && origin != "" {
		wu, werr := url.Parse(webURL)
		ou, oerr := url.Parse(origin)
		if werr == nil && oerr == nil && wu.Scheme != "" && wu.Host != "" && wu.Scheme == ou.Scheme && wu.Host == ou.Host {
			return true
		}
	}
	// Packaged Electron loadFile uses a null / file origin while calling the local API.
	if origin == "" || origin == "null" || strings.HasPrefix(origin, "file://") {
		return true
	}
	return false
}
