package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	addr := ":8024"
	log.Printf("Starting HTTP server at %s", addr)
	mux := http.NewServeMux()
	mux.Handle("/webhook", http.HandlerFunc(ServeHTTP))
	mux.Handle("/epub/", http.HandlerFunc(Epub))
	mux.Handle("/", http.HandlerFunc(Index))
	srv := http.Server{
		Addr:    addr,
		Handler: mux,
	}
	go func() {
		err := srv.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			log.Println("Error:", err)
			cancel()
		}
	}()

	<-ctx.Done()
	log.Print("Shuting down HTTP server")
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Println(err)
	}
	log.Println("Goodbye")
}
