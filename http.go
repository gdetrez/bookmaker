package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gdetrez/bookmaker/internal/catalog"
)

var (
	//go:embed templates/*
	files embed.FS
)

func Index(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Catalog []*catalog.Card
	}{
		Catalog: catalog.Cards(),
	}
	t := template.Must(template.ParseFS(files, "templates/index.html"))
	log.Print(t.Execute(w, data))
}

func Epub(w http.ResponseWriter, r *http.Request) {
	sid, ok := strings.CutPrefix(r.URL.Path, "/epub/")
	if !ok {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	id, err := strconv.ParseUint(sid, 10, 64)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	card := catalog.Cards()[id]
	f, err := os.Open(card.File())
	defer f.Close()
	if err != nil {
		http.Error(w, fmt.Sprintf("%s", err), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/epub+zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(card.File())))
	io.Copy(w, f)
}

func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var file string
	var err error
	if r.Header.Get("X-Miniflux-Event-Type") != "save_entry" {
		return
	}
	var event struct {
		Entry Entry `json:"entry"`
	}
	ctx, card := catalog.StartCard(r.Context())
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		card.SetError(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	card.SetTitle(fmt.Sprintf("%s: %s", event.Entry.Feed.Title, event.Entry.Title))
	file, err = SaveEntry(ctx, event.Entry)
	card.SetFile(file)
	if err != nil {
		card.SetError(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	return
}

func SaveEntry(ctx context.Context, e Entry) (string, error) {
	catalog.Printf(ctx, "Entry: %s (%s)", e.Title, e.URL)

	tmp, err := ioutil.TempDir("", "BOOK")
	if err != nil {
		return "", err
	}
	// defer os.RemoveAll(tmp) // clean up

	file, err := GenerateEpub(ctx, e, tmp)
	if err != nil {
		return file, err
	}

	err = SendToReMarkable(ctx, file)
	if err != nil {
		return file, err
	}
	return file, nil
}

func SendToReMarkable(ctx context.Context, path string) error {
	err := rmapi(ctx, "refresh")
	if err != nil {
		return err
	}
	err = rmapi(ctx, "put", path, "/@Inbox")
	if err != nil {
		return err
	}
	catalog.Printf(ctx, "File sent to reMarkable cloud: %s", path)
	return nil
}

func rmapi(ctx context.Context, args ...string) error {
	command := "rmapi"
	cmd := exec.CommandContext(ctx, command, args...)
	catalog.Printf(ctx, "Running: %v", cmd)
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		catalog.Printf(ctx, "output:\n%s", output)
	}
	if err != nil {
		return fmt.Errorf("rmapi: %w: %s", err, output)
	}
	return nil
}
