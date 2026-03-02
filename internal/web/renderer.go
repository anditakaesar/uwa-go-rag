package web

import (
	"context"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
)

const baseTemplate = "base.html"

type Renderer struct {
	templates map[string]*template.Template
}

func NewRenderer() *Renderer {
	cache := make(map[string]*template.Template)

	files, _ := TemplatesFS.ReadDir("templates")

	for _, file := range files {
		name := file.Name()

		if name == baseTemplate || filepath.Ext(name) != ".html" {
			continue
		}

		t := template.Must(template.ParseFS(TemplatesFS,
			"templates/base.html",
			"templates/"+name))

		cache[name] = t
	}

	return &Renderer{templates: cache}
}

func (r *Renderer) Render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	tmpl, ok := r.templates[name]
	if !ok {
		http.Error(w, "template not found", http.StatusNotFound)
		return
	}

	err := tmpl.ExecuteTemplate(w, baseTemplate, data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (r *Renderer) Render2(ctx context.Context, w http.ResponseWriter, name string, data map[string]any) {
	_, ok := ctx.Value(domain.IdentityKey).(domain.Identity)
	data["IsLoggedIn"] = ok
	r.Render(w, name, data)
}
