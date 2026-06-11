package server

import (
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wu/agentsstore/internal/aggregator"
	"github.com/wu/agentsstore/internal/source"
	"github.com/wu/agentsstore/internal/store"
	"github.com/wu/agentsstore/internal/ui"
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

	staticSub, _ := fs.Sub(ui.Static, "static")
	s.Engine.StaticFS("/static", http.FS(staticSub))

	uih := NewUI(s)
	adminUI := s.Engine.Group("/admin")
	adminUI.GET("/", uih.Index)
	adminUI.GET("/sources/new", uih.NewForm)
	adminUI.GET("/sources/:name/edit", uih.EditForm)
	adminUI.GET("/preview", uih.Preview)

	admin := &Admin{Server: s}
	api := s.Engine.Group("/admin/api")
	api.GET("/sources", admin.ListSources)
	api.POST("/sources", admin.AddSource)
	api.PUT("/sources/:name", admin.UpdateSource)
	api.DELETE("/sources/:name", admin.DeleteSource)
	api.POST("/sources/:name/refresh", admin.RefreshOne)
	api.POST("/refresh", admin.RefreshAll)
	api.GET("/aggregated", admin.Aggregated)
}
