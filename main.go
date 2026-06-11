package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/wu/agentsstore/internal/server"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	s := &server.Server{Engine: r}
	s.RegisterRoutes()
	log.Println("agentsstore listening on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
