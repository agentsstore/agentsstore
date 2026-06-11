package server

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/wu/agentsstore/internal/config"
	"github.com/wu/agentsstore/internal/source"
)

type Admin struct {
	Server *Server
}

func (a *Admin) ListSources(c *gin.Context) {
	states, err := a.Server.Manager.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sources": states})
}

func (a *Admin) AddSource(c *gin.Context) {
	var s config.Source
	if err := c.BindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if s.Enabled == false {
		s.Enabled = true
	}
	if err := a.Server.Manager.Add(s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"source": s})
}

func (a *Admin) UpdateSource(c *gin.Context) {
	name := c.Param("name")
	var s config.Source
	if err := c.BindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.Server.Manager.Update(name, s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"source": s})
}

func (a *Admin) DeleteSource(c *gin.Context) {
	name := c.Param("name")
	if err := a.Server.Manager.Delete(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *Admin) RefreshOne(c *gin.Context) {
	name := c.Param("name")
	spec, err := a.Server.Manager.Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	src, err := a.Server.Registry.Build(spec)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	dest := a.Server.Store.SourceDir(name)
	if err := src.Fetch(c.Request.Context(), dest); err != nil {
		a.Server.Manager.SetState(name, func(st *source.State) {
			st.LastError = err.Error()
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.Server.Manager.SetState(name, func(st *source.State) {
		st.LastError = ""
		st.LastRefresh = nowRFC3339()
	})
	cfg, _ := a.Server.Manager.Snapshot()
	names := []string{}
	for _, s := range cfg.Sources {
		if s.Enabled {
			names = append(names, s.Name)
		}
	}
	if err := a.Server.Aggregator.Refresh(names); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"refreshed": name})
}

func (a *Admin) RefreshAll(c *gin.Context) {
	cfg, err := a.Server.Manager.Snapshot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	failed := map[string]string{}
	ok := []string{}
	for _, s := range cfg.Sources {
		if !s.Enabled {
			continue
		}
		src, err := a.Server.Registry.Build(s)
		if err != nil {
			failed[s.Name] = err.Error()
			continue
		}
		dest := a.Server.Store.SourceDir(s.Name)
		if err := src.Fetch(c.Request.Context(), dest); err != nil {
			a.Server.Manager.SetState(s.Name, func(st *source.State) { st.LastError = err.Error() })
			failed[s.Name] = err.Error()
			continue
		}
		a.Server.Manager.SetState(s.Name, func(st *source.State) {
			st.LastError = ""
			st.LastRefresh = nowRFC3339()
		})
		ok = append(ok, s.Name)
	}
	names := ok
	if err := a.Server.Aggregator.Refresh(names); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"refreshed": ok, "failed": failed})
}

func (a *Admin) Aggregated(c *gin.Context) {
	data, err := os.ReadFile(a.Server.Store.AggregatedDir() + "/marketplace.json")
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", data)
}
