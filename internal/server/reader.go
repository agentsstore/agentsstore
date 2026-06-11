package server

import (
	"log"
	"net/http"
	"os"
	"strings"

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
		log.Printf("read aggregated marketplace: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read aggregated marketplace"})
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
	rel = strings.TrimPrefix(rel, "/")
	full, err := r.Store.ResolvePluginPath(src, rel)
	if err != nil {
		log.Printf("plugin path rejected: src=%q rel=%q err=%v", src, rel, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plugin path"})
		return
	}
	// Use Lstat to detect symlinks (Stat follows them)
	linfo, err := os.Lstat(full)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if linfo.Mode()&os.ModeSymlink != 0 {
		log.Printf("plugin path rejected: symlink src=%q rel=%q", src, rel)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plugin path"})
		return
	}
	info, err := os.Stat(full)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if info.IsDir() {
		c.Status(http.StatusNotFound)
		return
	}
	c.File(full)
}
