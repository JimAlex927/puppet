package web

import (
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func NewHandler(dist fs.FS, apiTarget string) (http.Handler, error) {
	target, err := url.Parse(apiTarget)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	files := http.FileServer(http.FS(dist))

	mux := http.NewServeMux()
	mux.Handle("/api/", proxy)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			path := strings.TrimPrefix(r.URL.Path, "/")
			if _, err := fs.Stat(dist, path); err != nil {
				r.URL.Path = "/"
				http.ServeFileFS(w, r, dist, "index.html")
				return
			}
		}
		files.ServeHTTP(w, r)
	})
	return mux, nil
}
