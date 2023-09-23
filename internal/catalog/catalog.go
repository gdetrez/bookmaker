package catalog

import "fmt"

type Book struct {
	Title string
}

type Entry struct {
	Book
	Id   string
	File string
	Log  string
	Err  error
}

var db []Entry

func Rec(title string, file string, log string, err error) {
	id := fmt.Sprintf("%d", len(db))
	db = append(db, Entry{Book: Book{Title: title}, Id: id, File: file, Log: log, Err: err})
}

func List() []Entry {
	return db
}
