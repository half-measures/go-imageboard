// main.go
package main

import (
	"html/template"
	"log"
	"net/http"
)

// 1. Define our list of boards
var boards = []Board{
	{Tag: "v", Name: "Video Games"},
	{Tag: "g", Name: "Technology"},
	{Tag: "tv", Name: "Television & Film"},
}

// 2. Define our handler for the homepage
func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Get boards from the database instead of the hardcoded list
	boards, err := GetBoards()
	if err != nil {
		http.Error(w, "Failed to fetch boards", http.StatusInternalServerError)
		log.Println("Error fetching boards:", err)
		return
	}

	data := PageData{
		Title:  "Go Imageboard",
		Boards: boards,
	}

	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	// 1. Initialize the database
	InitDB()
	// Defer closing the connection until the application exits
	defer DB.Close()

	// 2. Set up a file server for our static assets
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// 3. Register our homepage handler
	http.HandleFunc("/", homeHandler)

	// 4. Start the web server
	log.Println("Starting server on :8080...")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
