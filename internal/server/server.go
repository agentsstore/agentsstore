package server

import (
	"github.com/gin-gonic/gin"
	"github.com/wu/agentsstore/internal/aggregator"
	"github.com/wu/agentsstore/internal/store"
)

type Server struct {
	Engine     *gin.Engine
	Store      *store.Store
	Aggregator *aggregator.Aggregator
	BaseURL    string
}

func (s *Server) RegisterRoutes() {
	s.Engine.GET("/healthz", func(c *gin.Context) {
		c.String(200, "ok")
	})

	reader := &Reader{Store: s.Store, Aggregator: s.Aggregator, BaseURL: s.BaseURL}
	s.Engine.GET("/marketplace.json", reader.Marketplace)
	s.Engine.GET("/plugins/:source/*path", reader.PluginFile)
}
