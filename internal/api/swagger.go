package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    body { margin: 0; background: #fafafa; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function () {
      SwaggerUIBundle({
        url: "/swagger/openapi.json",
        dom_id: "#swagger-ui",
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis],
      });
    };
  </script>
</body>
</html>`

func (s *Server) setupSwagger() {
	s.router.GET("/swagger", s.swaggerUI)
	s.router.GET("/swagger/openapi.json", s.swaggerSpec)
}

func (s *Server) swaggerUI(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, swaggerUIHTML)
}

func (s *Server) swaggerSpec(c *gin.Context) {
	spec := s.buildOpenAPISpec(c.Request)
	c.JSON(http.StatusOK, spec)
}

func (s *Server) WriteOpenAPISpec(filePath string) error {
	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf("swagger output path is empty")
	}

	spec := s.buildOpenAPISpec(nil)
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(filePath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return os.WriteFile(filePath, data, 0o644)
}

type openAPISpec struct {
	OpenAPI string                     `json:"openapi"`
	Info    openAPIInfo                `json:"info"`
	Servers []openAPIServer            `json:"servers,omitempty"`
	Paths   map[string]openAPIPathItem `json:"paths"`
	Tags    []openAPITag               `json:"tags,omitempty"`
}

type openAPIInfo struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

type openAPIServer struct {
	URL string `json:"url"`
}

type openAPITag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type openAPIPathItem map[string]openAPIOperation

type openAPIOperation struct {
	Summary   string                     `json:"summary,omitempty"`
	Tags      []string                   `json:"tags,omitempty"`
	Responses map[string]openAPIResponse `json:"responses"`
}

type openAPIResponse struct {
	Description string `json:"description"`
}

func (s *Server) buildOpenAPISpec(req *http.Request) openAPISpec {
	paths := map[string]openAPIPathItem{}
	tagSet := map[string]struct{}{}

	for _, route := range s.router.Routes() {
		if !strings.HasPrefix(route.Path, "/api/") {
			continue
		}

		path := toOpenAPIPath(route.Path)
		method := strings.ToLower(route.Method)
		item := paths[path]
		if item == nil {
			item = openAPIPathItem{}
		}

		tag := routeTag(path)
		var tags []string
		if tag != "" {
			tags = []string{tag}
			tagSet[tag] = struct{}{}
		}

		item[method] = openAPIOperation{
			Summary:   fmt.Sprintf("%s %s", route.Method, route.Path),
			Tags:      tags,
			Responses: map[string]openAPIResponse{"200": {Description: "OK"}},
		}
		paths[path] = item
	}

	tags := sortedTags(tagSet)

	spec := openAPISpec{
		OpenAPI: "3.0.3",
		Info: openAPIInfo{
			Title:       "sing-box-manager API",
			Version:     s.version,
			Description: "Auto-generated from gin routes. Models and request schemas are placeholders.",
		},
		Paths: paths,
		Tags:  tags,
	}

	if req != nil {
		spec.Servers = []openAPIServer{{URL: serverURL(req)}}
	}

	return spec
}

func toOpenAPIPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = "{" + strings.TrimPrefix(part, ":") + "}"
			continue
		}
		if strings.HasPrefix(part, "*") {
			parts[i] = "{" + strings.TrimPrefix(part, "*") + "}"
		}
	}
	return strings.Join(parts, "/")
}

func routeTag(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/")
	if trimmed == path {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func sortedTags(tagSet map[string]struct{}) []openAPITag {
	if len(tagSet) == 0 {
		return nil
	}

	keys := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		keys = append(keys, tag)
	}
	sort.Strings(keys)

	tags := make([]openAPITag, 0, len(keys))
	for _, tag := range keys {
		tags = append(tags, openAPITag{Name: tag})
	}

	return tags
}

func serverURL(req *http.Request) string {
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	if forwarded := req.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = forwarded
	}

	host := req.Host
	if host == "" {
		host = "localhost"
	}

	return fmt.Sprintf("%s://%s", scheme, host)
}
