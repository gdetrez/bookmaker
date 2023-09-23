package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"path"
	"strings"

	"github.com/bmaupin/go-epub"
	"github.com/gdetrez/bookmaker/ctxlog"
	"github.com/skip2/go-qrcode"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/html"
)

func GenerateEpub(ctx context.Context, a article, outdir string) (string, error) {
	var err error
	log := ctxlog.LoggerFromContext(ctx)
	filepath := path.Join(outdir, fmt.Sprintf("%s.epub", strings.ReplaceAll(a.Title, "/", "-")))
	e := epub.NewEpub(a.Title)
	content := AddImages(ctx, a.Content, e)
	e.AddSection(fmt.Sprintf("<h1>%s</h1>", a.Title)+content, a.Title, "", "")
	AddQRCode(ctx, a.URL, e)
	err = e.Write(filepath)
	if err != nil {
		return "", err
	}
	log.Printf("Epub writen to %s", filepath)
	return filepath, nil
}

func AddImages(ctx context.Context, content string, e *epub.Epub) string {
	log := ctxlog.LoggerFromContext(ctx)
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
					span.AddEvent(fmt.Sprintf("Added image %s", epubSrc), trace.WithAttributes(
						attribute.String("src.original", originalSrc),
						attribute.String("src.cleaned", cleanedSrc),
						attribute.String("src.epub", epubSrc)))
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

func AddQRCode(ctx context.Context, url string, e *epub.Epub) {
	span := trace.SpanFromContext(ctx)
	png, err := qrcode.Encode(url, qrcode.Medium, -1)
	if err != nil {
		span.RecordError(err)
		return
	}
	b64 := make([]byte, base64.StdEncoding.EncodedLen(len(png)))
	base64.StdEncoding.Encode(b64, png)
	e.AddSection(
		fmt.Sprintf(`
    <center>
      <p><img src="data:image/png;base64,%s" />
      <p><a href="%s">%s</a>
    </center>`, string(b64), url, url),
		"Source",
		"", "",
	)
}
