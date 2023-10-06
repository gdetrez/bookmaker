package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"path"
	"strings"

	"github.com/bmaupin/go-epub"
	"github.com/gdetrez/bookmaker/internal/catalog"
	"github.com/skip2/go-qrcode"
	"golang.org/x/net/html"
)

func GenerateEpub(ctx context.Context, entry Entry, outdir string) (string, error) {
	var err error
	title := fmt.Sprintf("%s %s (%s)", entry.PublishedAt.Format("06.01.02"), entry.Title, entry.Feed.Title)
	filepath := path.Join(outdir, fmt.Sprintf("%s.epub", strings.ReplaceAll(title, "/", "-")))
	e := epub.NewEpub(title)
	e.SetAuthor(entry.Author)
	content := AddImages(ctx, entry.Content, e)
	e.AddSection(fmt.Sprintf("<h1>%s</h1>", entry.Title)+content, entry.Title, "", "")
	AddQRCode(ctx, entry.URL, e)
	err = e.Write(filepath)
	if err != nil {
		return "", err
	}
	catalog.Printf(ctx, "Epub writen to %s", filepath)
	return filepath, nil
}

func AddImages(ctx context.Context, content string, e *epub.Epub) string {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		catalog.Printf(ctx, "Error parsing html: %s", err)
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
						catalog.Printf(ctx, "Error: couldn't add image %s: %v", cleanedSrc, err)
						continue
					}
					n.Attr[i] = html.Attribute{Namespace: a.Namespace, Key: a.Key, Val: epubSrc}
					catalog.Printf(ctx, "Added image %s as %s", originalSrc, epubSrc)
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
	png, err := qrcode.Encode(url, qrcode.Medium, -1)
	if err != nil {
		catalog.Printf(ctx, "Error generating QR code: %s", err)
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
