package main

import (
	"context"
	"flag"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/sklog"
	jobs "go.skia.org/infra/pinpoint/go/frontend/service"
	jobstore "go.skia.org/infra/pinpoint/go/sql/jobs_store"
)

var (
	port             = flag.String("port", ":8080", "The port to listen on for HTTP traffic.")
	connectionString = flag.String("connection_string", "postgresql://root@localhost:5432/natnael-test-database?sslmode=disable",
		"The connection string for the Pairwise backend database.")
	prodAssetsDir = flag.String("prod_assest_dir", "pinpoint/ui/pages/production",
		"The resource/assets directory")
)

func main() {
	flag.Parse()
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(*connectionString)
	if err != nil {
		sklog.Fatalf("failed to parse database config: %s", err)
	}
	pool, err := pgxpool.ConnectConfig(ctx, cfg)
	if err != nil {
		sklog.Fatalf("failed to connect to database: %s", err)
	}
	js := jobstore.NewJobStore(pool)

	service := jobs.New(ctx, js, *prodAssetsDir)

	router := chi.NewRouter()
	service.RegisterHandlers(router)

	// /dist/ is the assets_serving_path defined in the BUILD for the landing page
	router.Handle("/dist/*", http.StripPrefix("/dist/", http.FileServer(http.Dir(*prodAssetsDir))))

	sklog.Infof("http://localhost%s", *port)

	server := &http.Server{
		Addr:    *port,
		Handler: router,
	}
	sklog.Fatal(server.ListenAndServe())
}
