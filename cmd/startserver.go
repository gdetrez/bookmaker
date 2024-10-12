/*
Copyright © 2024 Grégoire Détrez <gregoire@d13.info>

*/
package cmd

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
        "github.com/gdetrez/bookmaker/internal"
)

// startserverCmd represents the startserver command
var startserverCmd = &cobra.Command{
	Use:   "startserver",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	addr := ":8024"
	log.Printf("Starting HTTP server at %s", addr)
	mux := http.NewServeMux()
	mux.Handle("/webhook", http.HandlerFunc(internal.ServeHTTP))
	mux.Handle("/epub/", http.HandlerFunc(internal.Epub))
	mux.Handle("/", http.HandlerFunc(internal.Index))
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
	},
}

func init() {
	rootCmd.AddCommand(startserverCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// startserverCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// startserverCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
