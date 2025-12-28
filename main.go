// main.go
package main

import (
	"crypto/subtle"
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strings"
)

// 1. Define our list of boards
var boards = []Board{
	{Tag: "v", Name: "Video Games"},
	{Tag: "g", Name: "Technology"},
	{Tag: "tv", Name: "Television & Film"},
}

// formatComment processes the comment for greentext and reply links
func formatComment(comment string) template.HTML {
	// Escape the comment for safety
	escaped := html.EscapeString(comment)

	// Greentext: lines starting with > but NOT >>
	lines := strings.Split(escaped, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "&gt;") && !strings.HasPrefix(line, "&gt;&gt;") {
			lines[i] = fmt.Sprintf("<span class=\"greentext\">%s</span>", line)
		}
	}
	escaped = strings.Join(lines, "\n")

	// Reply links: >>ID
	re := regexp.MustCompile(`&gt;&gt;(\d+)`)
	escaped = re.ReplaceAllString(escaped, `<a class="reply-link" href="#p$1">&gt;&gt;$1</a>`)

	// Convert newlines to <br>
	escaped = strings.ReplaceAll(escaped, "\n", "<br>")

	return template.HTML(escaped)
}

var funcMap = template.FuncMap{
	"formatComment": formatComment,
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

	tmpl, err := template.New("index.html").Funcs(funcMap).ParseFiles("templates/index.html")
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
	http.HandleFunc("/admin/delete/", BasicAuth(deleteHandler, "admin", "secret123"))
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/{boardTag}/", boardHandler)

	log.Println("Starting server on :8081...")
	err := http.ListenAndServe(":8081", nil)
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

		tmpl, err := template.New("board.html").Funcs(funcMap).ParseFiles("templates/board.html")
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

		imagesToDelete, pruned, err := CreateReply(threadID, comment, imageURL)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if pruned {
			DeleteFiles(imagesToDelete)
			// Redirect back to the board index since the thread is gone
			http.Redirect(w, r, "/"+boardTag+"/", http.StatusSeeOther)
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

	tmpl, err := template.New("thread.html").Funcs(funcMap).ParseFiles("templates/thread.html")
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

	// CreateThreadAndOP now returns a list of images that were deleted
	imagesToDelete, err := CreateThreadAndOP(boardTag, subject, comment, imageURL)
	if err != nil {
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		log.Println("Error creating thread:", err)
		return
	}

	// If any images were returned (from a pruned thread), delete them.
	if len(imagesToDelete) > 0 {
		DeleteFiles(imagesToDelete)
	}

	http.Redirect(w, r, "/"+boardTag+"/", http.StatusSeeOther)
}

// BasicAuth wraps a handler and requires a username/password
func BasicAuth(handler http.HandlerFunc, username, password string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get credentials from the request header
		user, pass, ok := r.BasicAuth()

		// Verify credentials using ConstantTimeCompare to be secure
		if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// If pass, call the actual handler
		handler(w, r)
	}
}

// --- DELETE HANDLER ---

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	// Expected URL: /admin/delete/thread/123 or /admin/delete/post/456
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	if len(parts) < 4 {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	actionType := parts[2] // "thread" or "post"
	idStr := parts[3]
	var id int
	fmt.Sscanf(idStr, "%d", &id)

	var imagesToDelete []string
	var err error

	// Perform Database Deletion
	if actionType == "thread" {
		imagesToDelete, err = DeleteThread(id)
	} else if actionType == "post" {
		var img string
		img, err = DeletePost(id)
		if img != "" {
			imagesToDelete = append(imagesToDelete, img)
		}
	} else {
		http.Error(w, "Unknown type", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Perform File Deletion
	DeleteFiles(imagesToDelete)

	// Redirect back to home or the board (simple redirect to home for now)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
