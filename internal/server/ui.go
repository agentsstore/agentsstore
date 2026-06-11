package server

import (
	"html/template"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/wu/agentsstore/internal/config"
	"github.com/wu/agentsstore/internal/ui"
)

type UI struct {
	Server *Server
	tpl    *template.Template
}

func NewUI(s *Server) *UI {
	tpl := template.Must(template.ParseFS(ui.Templates, "templates/*.html"))
	return &UI{Server: s, tpl: tpl}
}

func (u *UI) Index(c *gin.Context) {
	states, err := u.Server.Manager.List()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	type src struct {
		Name, Type, URL, Ref, LastError, LastRefresh string
		Enabled                                      bool
	}
	cfg, _ := u.Server.Manager.Snapshot()
	byName := map[string]config.Source{}
	for _, s := range cfg.Sources {
		byName[s.Name] = s
	}
	rows := []src{}
	for _, st := range states {
		s := byName[st.Name]
		rows = append(rows, src{
			Name: s.Name, Type: s.Type, URL: s.URL, Ref: s.Ref,
			Enabled: s.Enabled, LastError: st.LastError, LastRefresh: st.LastRefresh,
		})
	}
	u.render(c, "index.html", gin.H{"Title": "Sources", "Sources": rows})
}

func (u *UI) NewForm(c *gin.Context) {
	u.render(c, "new.html", gin.H{"Title": "Add source"})
}

func (u *UI) EditForm(c *gin.Context) {
	name := c.Param("name")
	s, err := u.Server.Manager.Get(name)
	if err != nil {
		c.String(http.StatusNotFound, err.Error())
		return
	}
	u.render(c, "edit.html", gin.H{"Title": "Edit " + name, "Source": s})
}

func (u *UI) Preview(c *gin.Context) {
	data, err := os.ReadFile(u.Server.Store.AggregatedDir() + "/marketplace.json")
	if err != nil {
		u.render(c, "preview.html", gin.H{"Title": "Preview", "Aggregated": "(no aggregated marketplace yet)"})
		return
	}
	u.render(c, "preview.html", gin.H{"Title": "Preview", "Aggregated": string(data)})
}

func (u *UI) render(c *gin.Context, name string, data gin.H) {
	data["Page"] = name
	c.Status(http.StatusOK)
	if err := u.tpl.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}
}
