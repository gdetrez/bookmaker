package catalog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"sync"
)

type ctxKey int

const (
	ctxKeyLogger ctxKey = iota
	ctxKeyCard
)

type Card struct {
	id    string
	title string
	file  string
	err   error
	log   bytes.Buffer
}

func (c *Card) Id() string    { return c.id }
func (c *Card) Title() string { return c.title }
func (c *Card) File() string  { return c.file }
func (c *Card) Error() error  { return c.err }
func (c *Card) Log() string   { return c.log.String() }

func (c *Card) SetTitle(t string) { c.title = t }
func (c *Card) SetFile(f string)  { c.file = f }
func (c *Card) SetError(e error)  { c.err = e }

var catalog struct {
	mu    sync.Mutex
	cards []*Card
}

func Cards() []Card {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	result := make([]Card, 0, len(catalog.cards))
	for _, c := range catalog.cards {
		result = append(result, *c)
	}
	return result
}

func Get(id string) (Card, bool) {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	for _, c := range catalog.cards {
		if c.id == id {
			return *c, true
		}
	}
	return Card{}, false
}

func StartCard(ctx context.Context) (context.Context, *Card) {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	card := new(Card)
	card.id = fmt.Sprintf("%d", len(catalog.cards))
	catalog.cards = append(catalog.cards, card)
	logger := log.New(io.MultiWriter(&card.log, log.Default().Writer()), "", log.LstdFlags)
	ctx = context.WithValue(ctx, ctxKeyCard, card)
	ctx = context.WithValue(ctx, ctxKeyLogger, logger)
	return ctx, card
}

func logger(ctx context.Context) *log.Logger {
	v, ok := ctx.Value(ctxKeyLogger).(*log.Logger)
	if v != nil && ok {
		return v
	}
	return log.Default()
}

func Println(ctx context.Context, v ...any)               { logger(ctx).Println(v...) }
func Printf(ctx context.Context, format string, v ...any) { logger(ctx).Printf(format, v...) }
