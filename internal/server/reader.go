package server

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/wu/agentsstore/internal/aggregator"
	"github.com/wu/agentsstore/internal/store"
)

type Reader struct {
	Store      *store.Store
	Aggregator *aggregator.Aggregator
	BaseURL    string
}

func (r *Reader) Marketplace(c *gin.Context) {
	data, err := os.ReadFile(r.Store.AggregatedDir() + "/marketplace.json")
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "no aggregated marketplace yet; add a source and refresh",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", data)
}

func (r *Reader) PluginFile(c *gin.Context) {
	src := c.Param("source")
	rel := c.Param("path")
	if rel == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing path"})
		return
	}
	if rel[0] == '/' {
		rel = rel[1:]
	}
	full, err := r.Store.ResolvePluginPath(src, rel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, err := os.Stat(full); err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.File(full)
}
