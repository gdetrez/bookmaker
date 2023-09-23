package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
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

	"github.com/gdetrez/bookmaker/ctxlog"
	"github.com/gdetrez/bookmaker/internal/catalog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	//go:embed templates/*
	files embed.FS
)

func Index(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Catalog []catalog.Entry
	}{
		Catalog: catalog.List(),
	}
	t := template.Must(template.ParseFS(files, "templates/index.html"))
	log.Print(t.Execute(w, data))
}

func Epub(w http.ResponseWriter, r *http.Request) {
	sid, ok := strings.CutPrefix(r.URL.Path, "/epub/")
	if !ok {
		http.Error(w, "Not Found", http.StatusNotFound)
	}
	id, err := strconv.ParseUint(sid, 10, 64)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
	}
	entry := catalog.List()[id]
	f, err := os.Open(entry.File)
	defer f.Close()
	if err != nil {
		http.Error(w, fmt.Sprintf("%s", err), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/epub+zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(entry.File)))
	io.Copy(w, f)
}

func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var logbuf bytes.Buffer
	var file string
	var err error
	var a article
	logger := log.New(io.MultiWriter(&logbuf, log.Default().Writer()), "", log.LstdFlags)
	defer func() { catalog.Rec(a.Title, file, logbuf.String(), err) }()
	ctx := ctxlog.WithLogger(r.Context(), logger)
	span := trace.SpanFromContext(ctx)

	span.SetAttributes(
		attribute.String("http.url", r.URL.String()),
	)

	contentType := r.Header["Content-Type"]
	span.SetAttributes(attribute.StringSlice("http.header.content-type", contentType))

	httpError := func(err error, status int) {
		span.RecordError(err)
		span.SetStatus(codes.Error, http.StatusText(status))
		http.Error(w, err.Error(), status)
	}

	if len(contentType) == 0 {
		httpError(errors.New("Missing Content-Type header"), http.StatusBadRequest)
		return
	}

	if contentType[0] == "application/x-www-form-urlencoded" {
		r.ParseForm()
		a.Title = r.Form["title"][0]
		a.URL = r.Form["url"][0]
		a.Content = r.Form["content"][0]
	} else if contentType[0] == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
			httpError(err, http.StatusBadRequest)
			return
		}
	} else {
		httpError(fmt.Errorf("Invalid Content-Type: %v", contentType[0]), http.StatusUnsupportedMediaType)
		return
	}

	file, err = SendArticle(ctx, a)
	if err != nil {
		httpError(err, http.StatusInternalServerError)
		return
	}
	return
}

func SendArticle(ctx context.Context, a article) (string, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("article.title", a.Title),
		attribute.String("article.url", a.URL),
	)

	tmp, err := ioutil.TempDir("", "BOOK")
	if err != nil {
		return "", err
	}
	// defer os.RemoveAll(tmp) // clean up

	file, err := GenerateEpub(ctx, a, tmp)
	if err != nil {
		return file, err
	}
	span.SetAttributes(attribute.String("epub.file", file))

	err = SendToReMarkable(ctx, file)
	if err != nil {
		return file, err
	}
	return file, nil
}

func SendToReMarkable(ctx context.Context, path string) error {
	err := rmapi(ctx, "refresh")
	lgr := ctxlog.LoggerFromContext(ctx)
	if err != nil {
		return err
	}
	err = rmapi(ctx, "put", path, "/@Inbox")
	if err != nil {
		return err
	}
	lgr.Printf("File sent to reMarkable cloud: %s", path)
	return nil
}

func rmapi(ctx context.Context, args ...string) error {
	ctx, span := tracer.Start(ctx, "rmapi")
	lgr := ctxlog.LoggerFromContext(ctx)
	defer span.End()
	command := "rmapi"
	span.SetAttributes(
		attribute.String("command", command),
		attribute.StringSlice("args", args))
	cmd := exec.CommandContext(ctx, command, args...)
	lgr.Printf("Running: %v", cmd)
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		lgr.Printf("output:\n%s", output)
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "")
		return fmt.Errorf("rmapi: %w: %s", err, output)
	}
	return nil
}
