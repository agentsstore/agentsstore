package server

import "github.com/gin-gonic/gin"

type Server struct {
	Engine *gin.Engine
}

func (s *Server) RegisterRoutes() {
	s.Engine.GET("/healthz", func(c *gin.Context) {
		c.String(200, "ok")
	})
}
