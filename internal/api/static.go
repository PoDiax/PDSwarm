package api

import (
	"bytes"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

func init() {
	_ = mime.AddExtensionType(".js", "application/javascript")
	_ = mime.AddExtensionType(".mjs", "application/javascript")
	_ = mime.AddExtensionType(".css", "text/css; charset=utf-8")
	_ = mime.AddExtensionType(".html", "text/html; charset=utf-8")
	_ = mime.AddExtensionType(".json", "application/json")
	_ = mime.AddExtensionType(".svg", "image/svg+xml")
	_ = mime.AddExtensionType(".png", "image/png")
	_ = mime.AddExtensionType(".jpg", "image/jpeg")
	_ = mime.AddExtensionType(".jpeg", "image/jpeg")
	_ = mime.AddExtensionType(".gif", "image/gif")
	_ = mime.AddExtensionType(".ico", "image/x-icon")
	_ = mime.AddExtensionType(".woff", "font/woff")
	_ = mime.AddExtensionType(".woff2", "font/woff2")
	_ = mime.AddExtensionType(".ttf", "font/ttf")
	_ = mime.AddExtensionType(".wasm", "application/wasm")
}

// ServeStatic creates an http.Handler serving files from the embedded filesystem,
// setting appropriate Content-Type headers, caching headers, and falling back to index.html
// for SPA client-side routing on any non-/api route.
func ServeStatic(staticFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		// Unknown /api routes must return 404, not fall back to SPA index.html
		if strings.HasPrefix(r.URL.Path, "/api") {
			http.NotFound(w, r)
			return
		}

		if staticFS == nil {
			http.NotFound(w, r)
			return
		}

		cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if cleanPath == "" || cleanPath == "." {
			cleanPath = "index.html"
		}

		// Try opening the requested file
		file, err := staticFS.Open(cleanPath)
		if err != nil {
			serveSPAIndex(w, r, staticFS)
			return
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil {
			serveSPAIndex(w, r, staticFS)
			return
		}

		if stat.IsDir() {
			dirIndex := path.Join(cleanPath, "index.html")
			subFile, err := staticFS.Open(dirIndex)
			if err == nil {
				defer subFile.Close()
				if subStat, err := subFile.Stat(); err == nil {
					serveContent(w, r, subFile, subStat, "text/html; charset=utf-8", false)
					return
				}
			}
			serveSPAIndex(w, r, staticFS)
			return
		}

		contentType := mime.TypeByExtension(path.Ext(cleanPath))
		isHashedAsset := strings.HasPrefix(cleanPath, "assets/")
		serveContent(w, r, file, stat, contentType, isHashedAsset)
	}
}

func serveSPAIndex(w http.ResponseWriter, r *http.Request, staticFS fs.FS) {
	file, err := staticFS.Open("index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}

	serveContent(w, r, file, stat, "text/html; charset=utf-8", false)
}

func serveContent(w http.ResponseWriter, r *http.Request, file fs.File, stat fs.FileInfo, contentType string, isImmutable bool) {
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}

	if isImmutable {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else if strings.HasSuffix(stat.Name(), ".html") {
		w.Header().Set("Cache-Control", "no-cache")
	}

	if rs, ok := file.(io.ReadSeeker); ok {
		http.ServeContent(w, r, stat.Name(), stat.ModTime(), rs)
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), bytes.NewReader(data))
}
