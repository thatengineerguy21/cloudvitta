package spa

import (
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

func init() {
	_ = mime.AddExtensionType(".js", "application/javascript")
	_ = mime.AddExtensionType(".mjs", "application/javascript")
	_ = mime.AddExtensionType(".css", "text/css; charset=utf-8")
	_ = mime.AddExtensionType(".svg", "image/svg+xml")
	_ = mime.AddExtensionType(".json", "application/json")
	_ = mime.AddExtensionType(".wasm", "application/wasm")
}

//go:embed all:dist
var defaultEmbedFS embed.FS

// RFC7807Problem represents an RFC 7807 Problem Details response.
type RFC7807Problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
}

// Handler serves static frontend assets from an fs.FS and falls back to index.html for client-side routing.
// It also enforces a strict API guard returning RFC 7807 JSON 404 responses for API, MCP, docs, and probe paths.
type Handler struct {
	fileSystem fs.FS
}

// NewHandler creates an HTTP handler backed by the default embedded dist directory.
func NewHandler() http.Handler {
	subFS, err := fs.Sub(defaultEmbedFS, "dist")
	if err != nil {
		return NewHandlerWithFS(defaultEmbedFS)
	}
	return NewHandlerWithFS(subFS)
}

// NewHandlerWithFS creates an HTTP handler backed by the provided file system.
func NewHandlerWithFS(fileSystem fs.FS) http.Handler {
	return &Handler{
		fileSystem: fileSystem,
	}
}

// ServeHTTP handles requests by checking API route guards, serving static assets, or falling back to index.html.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set baseline security headers on all responses
	setSecurityHeaders(w)

	// 1. Strict API Guard: return RFC 7807 JSON 404 for unhandled API/MCP/docs/probe paths
	if isAPIRoute(r.URL.Path) {
		writeJSONNotFound(w, r)
		return
	}

	// Only allow GET and HEAD requests for static SPA assets
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	reqPath := path.Clean("/" + r.URL.Path)
	filePath := strings.TrimPrefix(reqPath, "/")

	// If root request, serve index.html
	if filePath == "" || filePath == "." {
		h.serveIndexHTML(w, r)
		return
	}

	// Attempt to open the requested static asset
	file, err := h.fileSystem.Open(filePath)
	if err != nil {
		// File does not exist: fall back to index.html for client-side SPA routing
		h.serveIndexHTML(w, r)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil || stat.IsDir() {
		// Directory requested: fall back to index.html
		h.serveIndexHTML(w, r)
		return
	}

	// Static asset exists and is a file
	h.serveStaticFile(w, r, filePath, stat, file)
}

func (h *Handler) serveStaticFile(w http.ResponseWriter, r *http.Request, filePath string, stat fs.FileInfo, file fs.File) {
	if strings.HasPrefix(filePath, "assets/") {
		// Vite content-hashed static assets can be cached aggressively and immutably
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else if filePath == "index.html" {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	} else {
		// Other static root files (e.g. favicon.svg, robots.txt)
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}

	ext := filepath.Ext(filePath)
	if ctype := mime.TypeByExtension(ext); ctype != "" {
		w.Header().Set("Content-Type", ctype)
	}

	if rs, ok := file.(io.ReadSeeker); ok {
		http.ServeContent(w, r, stat.Name(), stat.ModTime(), rs)
		return
	}

	data, readErr := io.ReadAll(file)
	if readErr != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) serveIndexHTML(w http.ResponseWriter, r *http.Request) {
	file, err := h.fileSystem.Open("index.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		http.Error(w, "index.html not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		http.Error(w, "index.html inaccessible", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if rs, ok := file.(io.ReadSeeker); ok {
		http.ServeContent(w, r, "index.html", stat.ModTime(), rs)
		return
	}

	data, readErr := io.ReadAll(file)
	if readErr != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func isAPIRoute(p string) bool {
	clean := path.Clean("/" + p)
	prefixes := []string{
		"/api",
		"/mcp",
		"/docs",
		"/healthz",
		"/readyz",
		"/metrics",
	}
	for _, prefix := range prefixes {
		if clean == prefix || strings.HasPrefix(clean, prefix+"/") {
			return true
		}
	}
	return false
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
}

func writeJSONNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusNotFound)

	instance := r.URL.RequestURI()
	if instance == "" {
		instance = r.URL.Path
	}

	resp := RFC7807Problem{
		Type:     "https://cloudvitta.dev/errors/not-found",
		Title:    "Not Found",
		Status:   http.StatusNotFound,
		Detail:   "The requested API endpoint does not exist.",
		Instance: instance,
	}

	_ = json.NewEncoder(w).Encode(resp)
}
