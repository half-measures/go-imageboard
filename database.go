// database.go
package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3" // The blank import for the driver
)

// DB is our global database connection
var DB *sql.DB

// InitDB initializes the database connection and creates tables
func InitDB() {
	var err error
	// Open a connection. This also creates the file if it doesn't exist.
	DB, err = sql.Open("sqlite3", "./imageboard.db")
	if err != nil {
		log.Fatal("Error opening database: ", err)
	}

	// Ping the database to verify the connection
	if err = DB.Ping(); err != nil {
		log.Fatal("Error connecting to database: ", err)
	}

	// Create the tables if they don't exist
	createTables()

	// Seed the database with our initial boards
	seedBoards()
}

func createTables() {
	// We use "IF NOT EXISTS" to make this function safe to run multiple times
	boardTableSQL := `
	CREATE TABLE IF NOT EXISTS boards (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tag TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL
	);`

	threadTableSQL := `
	CREATE TABLE IF NOT EXISTS threads (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		board_tag TEXT NOT NULL,
		subject TEXT,
		comment TEXT NOT NULL,
		image_url TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (board_tag) REFERENCES boards (tag)
	);`

	postTableSQL := `
	CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		thread_id INTEGER NOT NULL,
		comment TEXT NOT NULL,
		image_url TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (thread_id) REFERENCES threads (id)
	);`

	// Execute the SQL statements
	for _, sql := range []string{boardTableSQL, threadTableSQL, postTableSQL} {
		statement, err := DB.Prepare(sql)
		if err != nil {
			log.Fatal("Error preparing table SQL: ", err)
		}
		statement.Exec()
	}

	log.Println("Tables created successfully (if they didn't exist).")
}

func seedBoards() {
	// We use "INSERT OR IGNORE" to avoid errors if the board already exists
	// This makes the function idempotent (a good DevOps practice)
	boardsToSeed := []Board{
		{Tag: "v", Name: "Video Games"},
		{Tag: "g", Name: "Technology"},
		{Tag: "tv", Name: "Television & Film"},
	}

	statement, err := DB.Prepare("INSERT OR IGNORE INTO boards (tag, name) VALUES (?, ?)")
	if err != nil {
		log.Fatal("Error preparing seed statement: ", err)
	}
	defer statement.Close()

	for _, board := range boardsToSeed {
		_, err := statement.Exec(board.Tag, board.Name)
		if err != nil {
			log.Println("Error seeding board:", board.Tag, err)
		}
	}
	log.Println("Boards seeded successfully (if they didn't exist).")
}

// GetBoards fetches all boards from the database
func GetBoards() ([]Board, error) {
	var boards []Board

	rows, err := DB.Query("SELECT tag, name FROM boards ORDER BY tag")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var board Board
		if err := rows.Scan(&board.Tag, &board.Name); err != nil {
			return nil, err
		}
		boards = append(boards, board)
	}

	return boards, nil
}

func CreateThreadAndOP(boardTag, subject, comment, imageURL string) error {
	// Start a transaction for atomicity
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // Rollback if transaction fails, otherwise committed below

	// 1. Insert the new thread
	threadStmt, err := tx.Prepare(`
		INSERT INTO threads (board_tag, subject, comment, image_url) 
		VALUES (?, ?, ?, ?);
	`)
	if err != nil {
		return err
	}
	defer threadStmt.Close()

	// The 'comment' and 'image_url' fields in the threads table are just a duplicate
	// of the OP's post data for easy display on the board index.
	result, err := threadStmt.Exec(boardTag, subject, comment, imageURL)
	if err != nil {
		return err
	}

	threadID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	// 2. Insert the original post (OP)
	postStmt, err := tx.Prepare(`
		INSERT INTO posts (thread_id, comment, image_url) 
		VALUES (?, ?, ?);
	`)
	if err != nil {
		return err
	}
	defer postStmt.Close()

	// The OP's thread_id is the ID we just received
	_, err = postStmt.Exec(threadID, comment, imageURL)
	if err != nil {
		return err
	}

	// 3. Commit the transaction
	return tx.Commit()
}
func GetThread(threadID int) (Thread, error) {
	var t Thread

	// 1. Fetch the Thread details (Subject, Board)
	err := DB.QueryRow("SELECT id, board_tag, subject FROM threads WHERE id = ?", threadID).Scan(&t.ID, &t.BoardTag, &t.Subject)
	if err != nil {
		return t, err
	}

	// 2. Fetch all posts associated with this thread (OP + Replies)
	rows, err := DB.Query("SELECT id, comment, image_url, created_at FROM posts WHERE thread_id = ? ORDER BY id ASC", threadID)
	if err != nil {
		return t, err
	}
	defer rows.Close()

	var allPosts []Post
	for rows.Next() {
		var p Post
		var imageURL sql.NullString

		if err := rows.Scan(&p.ID, &p.Comment, &imageURL, &p.CreatedAt); err != nil {
			return t, err
		}
		if imageURL.Valid {
			p.ImageURL = imageURL.String
		}
		allPosts = append(allPosts, p)
	}

	// 3. Assign OP and Replies
	// Since we inserted the OP into the 'posts' table first, it is the first element.
	if len(allPosts) > 0 {
		t.OP = allPosts[0]
		t.Replies = allPosts[1:] // All subsequent posts are replies
	}

	return t, nil
}

// CreateReply inserts a new post into an existing thread
func CreateReply(threadID int, comment, imageURL string) error {
	statement, err := DB.Prepare("INSERT INTO posts (thread_id, comment, image_url) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer statement.Close()

	_, err = statement.Exec(threadID, comment, imageURL)
	return err
}

func GetThreads(boardTag string) ([]Thread, error) {
	// Query threads specifically for this board, ordered by newest first
	// We select the OP data stored directly on the thread record
	rows, err := DB.Query(`
		SELECT id, subject, comment, image_url, created_at 
		FROM threads 
		WHERE board_tag = ? 
		ORDER BY created_at DESC`, boardTag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []Thread
	for rows.Next() {
		var t Thread

		// We scan the data into the Thread struct.
		// Note: We map the comment/image directly to the OP struct inside the Thread.
		var subject sql.NullString  // Handle potential NULL subject
		var imageURL sql.NullString // Handle potential NULL image

		err := rows.Scan(&t.ID, &subject, &t.OP.Comment, &imageURL, &t.CreatedAt)
		if err != nil {
			return nil, err
		}

		// Convert sql.NullString to string
		if subject.Valid {
			t.Subject = subject.String
		}
		if imageURL.Valid {
			t.OP.ImageURL = imageURL.String
		}

		t.BoardTag = boardTag
		threads = append(threads, t)
	}

	return threads, nil
}
