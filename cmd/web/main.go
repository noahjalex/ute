package main

import (
	"bufio"
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
			data := struct{ Title string }{"Put linky here:"}
			tmpl := `<html><body>
			<h1>{{ .Title }}</h1>
			<form method="POST">
			  <input type="text" name="link" />
			  <input type="submit" value="Submit"/>
			</form>
			</body></html>`
			t := template.Must(template.New("").Parse(tmpl))
			w.Header().Add("Content-Type", "text/html; charset=utf-8")
			_ = t.Execute(w, data)
			return
		}

		if r.Method == "POST" {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "could not parse form", http.StatusBadRequest)
				return
			}
			link := r.FormValue("link")
			fmt.Printf("Got link: %s\n", link)

			// enable streaming
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming not supported", http.StatusInternalServerError)
				return
			}

			// preCmd := exec.Command("yt-dlp", link, "--print", "%(title)s.%(ext)s", "--skip-download")
			cmd := exec.Command("yt-dlp", link, "-P", "./shared/", "-o", "%(title)s.%(ext)s")

			stdout, _ := cmd.StdoutPipe()
			stderr, _ := cmd.StderrPipe()
			cmd.Start()

			scannerOut := bufio.NewScanner(stdout)
			scannerErr := bufio.NewScanner(stderr)

			for scannerOut.Scan() {
				fmt.Fprintf(w, "%s\n", scannerOut.Text())
				flusher.Flush()
			}
			for scannerErr.Scan() {
				fmt.Fprintf(w, "%s\n", scannerErr.Text())
				flusher.Flush()
			}

			if err := cmd.Wait(); err != nil {
				fmt.Fprintf(w, "Error: %v\n", err)
				return
			}
			// once done, redirect
			w.Header().Set("Refresh", "2; url=/videos/") // auto redirect in 2 sec
			fmt.Fprintf(w, "\nDone. Redirecting...\n<script>setTimeout(function(){window.location='/videos/%s'},2000);</script>", "")
			flusher.Flush()
			return
		}

		http.Error(w, "method not supported", http.StatusBadRequest)
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
