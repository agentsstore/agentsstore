package server

import (
	"github.com/gin-gonic/gin"
	"github.com/wu/agentsstore/internal/aggregator"
	"github.com/wu/agentsstore/internal/source"
	"github.com/wu/agentsstore/internal/store"
)

type Server struct {
	Engine     *gin.Engine
	Store      *store.Store
	Aggregator *aggregator.Aggregator
	Manager    *source.Manager
	Registry   *source.Registry
	BaseURL    string
	CfgPath    string
}

func (s *Server) RegisterRoutes() {
	s.Engine.GET("/healthz", func(c *gin.Context) {
		c.String(200, "ok")
	})

	reader := &Reader{Store: s.Store, Aggregator: s.Aggregator, BaseURL: s.BaseURL}
	s.Engine.GET("/marketplace.json", reader.Marketplace)
	s.Engine.GET("/plugins/:source/*path", reader.PluginFile)

	admin := &Admin{Server: s}
	g := s.Engine.Group("/admin/api")
	g.GET("/sources", admin.ListSources)
	g.POST("/sources", admin.AddSource)
	g.PUT("/sources/:name", admin.UpdateSource)
	g.DELETE("/sources/:name", admin.DeleteSource)
	g.POST("/sources/:name/refresh", admin.RefreshOne)
	g.POST("/refresh", admin.RefreshAll)
	g.GET("/aggregated", admin.Aggregated)
}
