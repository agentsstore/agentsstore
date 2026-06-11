package main

import (
	"log"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/wu/agentsstore/internal/aggregator"
	"github.com/wu/agentsstore/internal/config"
	"github.com/wu/agentsstore/internal/server"
	"github.com/wu/agentsstore/internal/source"
	"github.com/wu/agentsstore/internal/store"
)

func main() {
	cfgPath := envOr("AGENTSSTORE_CONFIG", "./config.yaml")
	cfg, err := loadOrInitConfig(cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	st := store.New(cfg.Server.DataDir)
	if err := os.MkdirAll(st.AggregatedDir(), 0o755); err != nil {
		log.Fatal(err)
	}

	mu := &sync.Mutex{}
	mgr := source.NewManager(cfgPath, st, mu)

	reg := source.NewRegistry()
	reg.Register("git", func(s config.Source) source.Source {
		out, err := source.NewGitSource(s)
		if err != nil {
			log.Fatalf("git source: %v", err)
		}
		return out
	})
	reg.Register("http", func(s config.Source) source.Source {
		out, err := source.NewHTTPSource(s)
		if err != nil {
			log.Fatalf("http source: %v", err)
		}
		return out
	})

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	srv := &server.Server{
		Engine:     r,
		Store:      st,
		Aggregator: aggregator.New(st, baseURL(cfg)),
		Manager:    mgr,
		Registry:   reg,
		BaseURL:    baseURL(cfg),
		CfgPath:    cfgPath,
	}
	srv.RegisterRoutes()

	log.Printf("agentsstore listening on %s (data=%s)", cfg.Server.Listen, cfg.Server.DataDir)
	if err := r.Run(cfg.Server.Listen); err != nil {
		log.Fatal(err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func baseURL(cfg *config.Config) string {
	if cfg.Server.BaseURL != "" {
		return cfg.Server.BaseURL
	}
	return "http://localhost" + cfg.Server.Listen
}

func loadOrInitConfig(path string) (*config.Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		c := &config.Config{
			Server:  config.ServerConfig{Listen: ":8080", DataDir: "./data"},
			Sources: []config.Source{},
		}
		if err := c.Save(path); err != nil {
			return nil, err
		}
		return c, nil
	}
	return config.Load(path)
}
