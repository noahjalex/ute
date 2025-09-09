package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	protocol := flag.String("protocol", "http", "protocol to use (default: 'http')")
	addr := flag.String("addr", ":8080", "port to host on (default: ':8080')")
	host := flag.String("host", "localhost", "host name (default: 'localhost')")

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "" || r.Method == "GET" {
			data := struct {
				Title string
			}{
				"Put linky here:",
			}
			tmpl, err := template.New("").Parse("<html><head></head><body><h1>{{ .Title }}</h1><form method=\"POST\"><input type=\"text\" name=\"link\" /><input type=\"submit\" value=\"Submit\"/></form></body></html>")
			if err != nil {
				log.Fatalf("error parsing template: %w\n", err)
			}
			w.Header().Add("Content-Type", "text/html; charset=utf-8")
			err = tmpl.Execute(w, data)
			if err != nil {
				log.Fatalf("error writing template: %w\n", err)
			}
		} else if r.Method == "POST" {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "could not parse form", http.StatusBadRequest)
				return
			}
			link := r.FormValue("link")
			fmt.Printf("Got link: %s\n", link)

			out, err := download(link)
			if err != nil {
				log.Fatal(err)
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(out))
		} else {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(fmt.Sprintf("method %s not supported", r.Method)))
		}
	})

	mux.HandleFunc("/videos/", func(w http.ResponseWriter, r *http.Request) {
		// Base directory to serve from
		baseDir := "./shared"

		// Clean the path and join with baseDir
		relPath := strings.TrimPrefix(r.URL.Path, "/videos/")
		targetDir := filepath.Join(baseDir, relPath)

		fi, err := os.Stat(targetDir)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		// If it’s a file -> serve directly for download
		if !fi.IsDir() {
			w.Header().Set("Content-Disposition", "attachment; filename="+fi.Name())
			http.ServeFile(w, r, targetDir)
			return
		}

		// Otherwise list the directory contents
		entries, err := os.ReadDir(targetDir)
		if err != nil {
			http.Error(w, "cannot read directory", http.StatusInternalServerError)
			return
		}

		data := struct {
			Title   string
			Path    string
			Entries []os.DirEntry
		}{
			Title:   "Folder Viewer",
			Path:    relPath,
			Entries: entries,
		}

		tmpl := `
	<html>
	  <head>
	    <title>{{ .Title }}</title>
	    <style>
	      body { font-family: sans-serif; padding: 20px; }
	      h1 { margin-bottom: 1em; }
	      ul { list-style: none; padding: 0; }
	      li { margin: 0.5em 0; }
	      a { text-decoration: none; color: #007acc; }
	      a:hover { text-decoration: underline; }
	    </style>
	  </head>
	  <body>
	    <h1>{{ if .Path }}Index of /{{ .Path }}{{ else }}Root Directory{{ end }}</h1>
	    <ul>
	      {{ if .Path }}
		<li><a href="../">⬅️ Parent Directory</a></li>
	      {{ end }}
	      {{ range .Entries }}
		<li>
		  {{ if .IsDir }}
		    📁 <a href="{{ .Name }}/">{{ .Name }}/</a>
		  {{ else }}
		    📄 <a href="{{ .Name }}">{{ .Name }}</a>
		  {{ end }}
		</li>
	      {{ end }}
	    </ul>
	  </body>
	</html>`

		t, err := template.New("dir").Parse(tmpl)
		if err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := t.Execute(w, data); err != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
	})

	fmt.Printf("Listening on %s://%s%s\n", *protocol, *host, *addr)
	switch *protocol {
	case "http":
		if err := http.ListenAndServe(*addr, mux); err != nil {
			log.Fatalf("server error: %w", err)
		}
	case "https":
		if err := http.ListenAndServeTLS(*addr, "", "", mux); err != nil {
			log.Fatalf("server error: %w", err)
		}
	}
}

func download(url string) (string, error) {
	cmd := exec.Command("yt-dlp", url, "-P", "./shared/")

	// Capture standard output and error
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()
	if err != nil {
		return "", errors.New(fmt.Sprintf("Command execution failed: %v\nStderr: %s", err, stderr.String()))
	}

	// Print the output
	return fmt.Sprintf("Command Output:\n%s", out.String()), nil
}
