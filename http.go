package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	span := trace.SpanFromContext(r.Context())

	span.SetAttributes(
		attribute.String("http.url", r.URL.String()),
	)

	var a article
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

	if err := SendArticle(r.Context(), a); err != nil {
		httpError(err, http.StatusInternalServerError)
		return
	}
	return
}

func SendArticle(ctx context.Context, a article) error {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("article.title", a.Title),
		attribute.String("article.url", a.URL),
	)

	tmp, err := ioutil.TempDir("", "example")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp) // clean up

	file, err := GenerateEpub(ctx, a, tmp)
	if err != nil {
		return err
	}
	span.SetAttributes(attribute.String("epub.file", file))

	err = SendToReMarkable(ctx, file)
	if err != nil {
		return err
	}
	return nil
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
	log.Printf("File sent to reMarkable cloud: %s", path)
	return nil
}

func rmapi(ctx context.Context, args ...string) error {
	ctx, span := tracer.Start(ctx, "rmapi")
	defer span.End()
	command := "rmapi"
	span.SetAttributes(
		attribute.String("command", command),
		attribute.StringSlice("args", args))
	cmd := exec.CommandContext(ctx, command, args...)
  log.Printf("Running: %v", cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "")
		return fmt.Errorf("rmapi: %w: %s", err, output)
	}
	return nil
}
