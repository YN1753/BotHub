package server

import (
	"net/http"
	"path"
	"strings"
)

func (s *Server) spaFileServer() http.Handler {
	fileServer := http.FileServer(http.FS(s.staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}
		if f, err := s.staticFS.Open(cleanPath); err == nil {
			fi, statErr := f.Stat()
			_ = f.Close()
			if statErr == nil && fi.IsDir() {
				// Prevent directory listing on /assets/ or arbitrary folders
				if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/assets") {
					http.NotFound(w, r)
					return
				}
			} else {
				if strings.HasSuffix(cleanPath, ".html") {
					w.Header().Set("Cache-Control", "no-cache")
				} else if strings.HasPrefix(cleanPath, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				} else {
					w.Header().Set("Cache-Control", "public, max-age=86400")
				}
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// Never masquerade missing API routes, hashed asset bundles, or non-HTML asset files as 200 index.html
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/assets") {
			http.NotFound(w, r)
			return
		}
		if ext := strings.ToLower(path.Ext(cleanPath)); ext != "" && ext != ".html" && ext != ".htm" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
