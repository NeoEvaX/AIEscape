package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

//go:embed editor.html
var editorHTML []byte

const (
	networkPath = "network.json"
	storyPath   = "story.json"
	addr        = ":8765"
)

func main() {
	// Verify we can find the data files.
	for _, p := range []string{networkPath, storyPath} {
		if _, err := os.Stat(p); err != nil {
			log.Fatalf("Cannot find %s — run the editor from the repo root directory.\n", p)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", serveEditor)
	mux.HandleFunc("/api/data", handleData)
	mux.HandleFunc("/api/save/network", handleSave(networkPath))
	mux.HandleFunc("/api/save/story", handleSave(storyPath))

	url := "http://localhost" + addr
	fmt.Println("AI Escape Editor →", url)
	fmt.Println("Press Ctrl+C to stop.")

	// Best-effort browser open after a short delay.
	go func() {
		time.Sleep(300 * time.Millisecond)
		openBrowser(url)
	}()

	log.Fatal(http.ListenAndServe(addr, mux))
}

func serveEditor(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(editorHTML)
}

func handleData(w http.ResponseWriter, r *http.Request) {
	networkData, err := os.ReadFile(networkPath)
	if err != nil {
		http.Error(w, "cannot read network.json: "+err.Error(), http.StatusInternalServerError)
		return
	}
	storyData, err := os.ReadFile(storyPath)
	if err != nil {
		// Story file is optional; return an empty collection.
		storyData = []byte(`{"events":[]}`)
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"network":%s,"story":%s}`, networkData, storyData)
}

func handleSave(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "cannot read body: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Validate JSON and re-format.
		var v interface{}
		if err := json.Unmarshal(body, &v); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		pretty, _ := json.MarshalIndent(v, "", "  ")
		if err := os.WriteFile(path, pretty, 0644); err != nil {
			http.Error(w, "cannot write file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	exec.Command(cmd, args...).Start() //nolint — best-effort
}
