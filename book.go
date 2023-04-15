package main

import (
	"context"
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/bmaupin/go-epub"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/html"
)

func GenerateEpub(ctx context.Context, a article, outdir string) (string, error) {
	var err error
	filepath := path.Join(outdir, fmt.Sprintf("%s.epub", strings.ReplaceAll(a.Title, "/", "-")))
	e := epub.NewEpub(a.Title)
	content := AddImages(ctx, a.Content, e)
	e.AddSection(fmt.Sprintf("<h1>%s</h1>", a.Title)+content, a.Title, "", "")
	err = e.Write(filepath)
	if err != nil {
		return "", err
	}
	log.Printf("Epub writen to %s", filepath)
	return filepath, nil
}

func AddImages(ctx context.Context, content string, e *epub.Epub) string {
	span := trace.SpanFromContext(ctx)
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		span.RecordError(err)
		return content
	}
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			for i, a := range n.Attr {
				if a.Key == "src" {
					originalSrc := a.Val
					cleanedSrc := strings.Split(originalSrc, "?")[0] // epub don't like file name with a query part…
					epubSrc, err := e.AddImage(cleanedSrc, "")
					if err != nil {
						span.RecordError(err)
						log.Printf("Error: couldn't add image %s: %v", cleanedSrc, err)
						continue
					}
					n.Attr[i] = html.Attribute{Namespace: a.Namespace, Key: a.Key, Val: epubSrc}
					span.AddEvent("ImageAdded", trace.WithAttributes(
						attribute.String("src.original", originalSrc),
						attribute.String("src.cleaned", cleanedSrc),
						attribute.String("src.epub", epubSrc)))
					span.End()
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	var b strings.Builder
	html.Render(&b, doc)
	return b.String()
}
