package main

import "time"

type Entry struct {
	Title       string    `json:"title"`
	Author      string    `json:"author"`
	URL         string    `json:"url"`
	Content     string    `json:"content"`
	PublishedAt time.Time `json:"published_at"`
	Feed        struct {
		Title string `json:"title"`
	} `json:"feed"`
}
