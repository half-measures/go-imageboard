// main.go
package main

import (
	"html/template"
	"log"
	"net/http"
	"strings"
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
	InitDB()
	defer DB.Close()

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Register handlers:
	http.HandleFunc("/", homeHandler)

	// 💡 New Handler Registration: Use the handler to catch all board requests.
	// The path MUST end in a slash for this catch-all to work correctly!
	http.HandleFunc("/{boardTag}/", boardHandler)

	log.Println("Starting server on :8080...")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func boardHandler(w http.ResponseWriter, r *http.Request) {
	// r.URL.Path will be something like "/g/" or "/g/123"

	// Split the path: ["", "g", ""] or ["", "g", "123"]
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	// The board tag is the first element
	boardTag := parts[0]

	// 1. Basic Validation: Ensure the tag exists in our database
	boardName := ""
	boards, err := GetBoards() // Use the function from database.go
	if err != nil {
		http.Error(w, "Database error fetching boards.", http.StatusInternalServerError)
		return
	}

	for _, board := range boards {
		if board.Tag == boardTag {
			boardName = board.Name
			break
		}
	}

	if boardName == "" {
		http.NotFound(w, r)
		return
	}

	// 2. Decide what kind of request this is (e.g., viewing a board vs. creating a thread)
	if r.Method == "GET" && len(parts) == 1 {
		// A request to view the main board page (e.g., GET /g/)
		// We'll fetch threads in the next step. For now, empty list.
		data := BoardPageData{
			BoardTag:  boardTag,
			BoardName: boardName,
			Threads:   []Thread{}, // Currently empty
		}

		tmpl, err := template.ParseFiles("templates/board.html")
		if err != nil {
			http.Error(w, "Failed to load template", http.StatusInternalServerError)
			log.Println("Template error:", err)
			return
		}
		tmpl.Execute(w, data)

	} else if r.Method == "POST" && len(parts) == 2 && parts[1] == "new" {
		// A request to create a new thread (e.g., POST /g/new)
		// We will implement this handler in the next major step!
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte("Thread creation logic coming soon!"))

	} else {
		// Catch-all for other odd URLs or methods
		http.NotFound(w, r)
	}
}
