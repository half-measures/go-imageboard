// main.go
package main

import (
	"fmt"
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

	// Serve static CSS files
	fsStatic := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fsStatic))

	// 💡 NEW: Serve uploaded images from the /uploads/ path
	fsUploads := http.FileServer(http.Dir(UploadDir))
	http.Handle("/uploads/", http.StripPrefix("/uploads/", fsUploads))

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/{boardTag}/", boardHandler)

	log.Println("Starting server on :8080...")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func boardHandler(w http.ResponseWriter, r *http.Request) {
	// Extract boardTag from path
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	boardTag := parts[0]

	// 1. Validate board existence (reuse previous logic)
	boardName := ""
	boards, err := GetBoards()
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

	// 2. Handle POST request for new thread creation
	if r.Method == "POST" && len(parts) == 2 && parts[1] == "new" {
		handleNewThread(w, r, boardTag)
		return // Important: stop execution after handling POST
	}

	// 3. Handle GET request to view the board (viewing logic from previous step)
	// 3. Handle GET request to view the board
	if r.Method == "GET" && len(parts) == 1 {

		// FETCH THREADS HERE
		threads, err := GetThreads(boardTag)
		if err != nil {
			http.Error(w, "Database error fetching threads.", http.StatusInternalServerError)
			log.Println("Error fetching threads:", err)
			return
		}

		data := BoardPageData{
			BoardTag:  boardTag,
			BoardName: boardName,
			Threads:   threads, // Pass the actual threads
		}

		tmpl, err := template.ParseFiles("templates/board.html")
		if err != nil {
			http.Error(w, "Failed to load template", http.StatusInternalServerError)
			log.Println("Template error:", err)
			return
		}
		tmpl.Execute(w, data)
		return
	}
	if len(parts) >= 3 && parts[1] == "res" {
		threadIDStr := parts[2]
		// We'll parse the ID inside the handler
		handleThreadRoute(w, r, boardTag, threadIDStr)
		return
	}

	// 2. New Thread: POST /g/new
	if r.Method == "POST" && len(parts) == 2 && parts[1] == "new" {
		handleNewThread(w, r, boardTag)
		return
	}

	// 3. View Board: GET /g/
	if r.Method == "GET" && len(parts) == 1 {
		// ... (Existing code to fetch and show threads) ...
		threads, _ := GetThreads(boardTag)
		data := BoardPageData{BoardTag: boardTag, Threads: threads}
		tmpl, _ := template.ParseFiles("templates/board.html")
		tmpl.Execute(w, data)
		return
	}

	http.NotFound(w, r)
}

func processUpload(r *http.Request) (string, error) {
	file, header, err := r.FormFile("image")
	if err != nil {
		if err == http.ErrMissingFile {
			return "", nil // No image uploaded, which is fine
		}
		return "", err
	}
	defer file.Close()
	return SaveFile(file, header)
}

func handleThreadRoute(w http.ResponseWriter, r *http.Request, boardTag, threadIDStr string) {
	// Convert string ID to int
	var threadID int
	fmt.Sscanf(threadIDStr, "%d", &threadID)

	if r.Method == "POST" {
		// Handle Reply Submission
		r.ParseMultipartForm(10 << 20)
		comment := r.FormValue("comment")

		// Use our new helper
		imageURL, err := processUpload(r)
		if err != nil {
			http.Error(w, "Upload error", http.StatusInternalServerError)
			return
		}

		err = CreateReply(threadID, comment, imageURL)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		// Redirect back to the same thread
		http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
		return
	}

	// Handle GET: Show the thread
	thread, err := GetThread(threadID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Reuse BoardPageData struct, but we only populate one Thread
	data := BoardPageData{
		BoardTag: boardTag,
		// We wrap the single thread in a slice because the struct expects a slice,
		// or we could make a new struct. Reusing is "quick and easy".
		Threads: []Thread{thread},
	}

	tmpl, err := template.ParseFiles("templates/thread.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, data)
}

// Update handleNewThread to use the processUpload helper too
func handleNewThread(w http.ResponseWriter, r *http.Request, boardTag string) {
	r.ParseMultipartForm(10 << 20)
	subject := r.FormValue("subject")
	comment := r.FormValue("comment")

	imageURL, err := processUpload(r)
	if err != nil {
		http.Error(w, "Upload error", http.StatusInternalServerError)
		return
	}

	err = CreateThreadAndOP(boardTag, subject, comment, imageURL)
	if err != nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/"+boardTag+"/", http.StatusSeeOther)
}
