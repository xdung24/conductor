package handlers

import (
	_ "embed"
	"html/template"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

//go:embed docs.md
var docsMarkdown []byte

// renderDocsMarkdown converts the embedded docs.md to safe HTML once at startup.
func renderDocsMarkdown() template.HTML {
	md := goldmark.New(
		goldmark.WithExtensions(extension.Table, extension.Strikethrough),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
	var buf strings.Builder
	if err := md.Convert(docsMarkdown, &buf); err != nil {
		return template.HTML("<p>Failed to render documentation.</p>")
	}
	return template.HTML(buf.String())
}

// DocsPage renders the usage documentation page.
func (h *Handler) DocsPage(c *gin.Context) {
	c.HTML(http.StatusOK, "docs.html", h.pageData(c, gin.H{
		"Content": h.docsHTML,
	}))
}
