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
	if r.Method == "GET" && len(parts) == 1 {
		// We'll update this GET logic in the next step to fetch actual threads
		data := BoardPageData{
			BoardTag:  boardTag,
			BoardName: boardName,
			Threads:   []Thread{}, // Still empty for now
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

	// Fallthrough for thread view, etc. (Not implemented yet)
	http.NotFound(w, r)
}
func handleNewThread(w http.ResponseWriter, r *http.Request, boardTag string) {
	// 1. Parse the multipart form data (necessary for file uploads)
	// Max 10MB file upload size
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "File too large or form parsing error.", http.StatusBadRequest)
		log.Println("Form parse error:", err)
		return
	}

	// Extract form fields
	subject := r.FormValue("subject")
	comment := r.FormValue("comment")

	if comment == "" {
		http.Error(w, "Comment is required.", http.StatusBadRequest)
		return
	}

	// 2. Handle Image Upload
	file, header, err := r.FormFile("image")
	imageURL := "" // Default to empty string if no image is uploaded

	if err == nil { // Only process if an image was provided
		defer file.Close()

		// Use the SaveFile function from upload.go
		imageURL, err = SaveFile(file, header)
		if err != nil {
			http.Error(w, "Failed to save image.", http.StatusInternalServerError)
			log.Println("Image save error:", err)
			return
		}
	} else if err != http.ErrMissingFile {
		// Log other errors besides the expected "missing file" error
		log.Println("Error retrieving file:", err)
		http.Error(w, "Error processing file upload.", http.StatusInternalServerError)
		return
	}

	// 3. Save the thread and post to the database
	err = CreateThreadAndOP(boardTag, subject, comment, imageURL)
	if err != nil {
		http.Error(w, "Failed to create thread in database.", http.StatusInternalServerError)
		log.Println("DB thread creation error:", err)
		return
	}

	// 4. Success: Redirect the user back to the board page
	// We use the HTTP 302 status code for redirection after a successful POST
	http.Redirect(w, r, "/"+boardTag+"/", http.StatusSeeOther)
}
