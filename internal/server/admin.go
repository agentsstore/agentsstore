package server

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/wu/agentsstore/internal/config"
	"github.com/wu/agentsstore/internal/source"
)

// sourceReq is the JSON request body for AddSource and UpdateSource.
// Enabled is a *bool so we can distinguish "omitted" (default to true)
// from an explicit `false` value.
type sourceReq struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	URL     string `json:"url"`
	Ref     string `json:"ref"`
	Enabled *bool  `json:"enabled"`
}

func (r sourceReq) toSource() config.Source {
	s := config.Source{
		Name: r.Name,
		Type: r.Type,
		URL:  r.URL,
		Ref:  r.Ref,
	}
	if r.Enabled != nil {
		s.Enabled = *r.Enabled
	} else {
		s.Enabled = true // default when omitted
	}
	return s
}

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
	var req sourceReq
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s := req.toSource()
	if err := a.Server.Manager.Add(s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"source": s})
}

func (a *Admin) UpdateSource(c *gin.Context) {
	name := c.Param("name")
	var req sourceReq
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s := req.toSource()
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
	cfg, _ := a.Server.Manager.Snapshot()
	names := []string{}
	for _, s := range cfg.Sources {
		if s.Enabled {
			names = append(names, s.Name)
		}
	}
	if err := a.Server.Aggregator.Refresh(names); err != nil {
		// Don't update LastRefresh; record the aggregator failure
		a.Server.Manager.SetState(name, func(st *source.State) {
			st.LastError = "aggregator: " + err.Error()
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.Server.Manager.SetState(name, func(st *source.State) {
		st.LastError = ""
		st.LastRefresh = nowRFC3339()
	})
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
			a.Server.Manager.SetState(s.Name, func(st *source.State) {
				st.LastError = "build: " + err.Error()
			})
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
