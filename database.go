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

// CreateThreadAndOP creates a new thread, ensuring the board does not exceed its thread limit.
// It returns a list of image file paths that were deleted if a thread was pruned.
func CreateThreadAndOP(boardTag, subject, comment, imageURL string) ([]string, error) {
	const threadLimit = 20
	var imagesToDelete []string

	// Start a transaction for atomicity
	tx, err := DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() // Rollback if transaction fails, otherwise committed below

	// 1. Count current threads on the board
	var threadCount int
	err = tx.QueryRow("SELECT COUNT(*) FROM threads WHERE board_tag = ?", boardTag).Scan(&threadCount)
	if err != nil {
		return nil, err
	}

	// 2. If limit is reached, delete the oldest thread
	if threadCount >= threadLimit {
		// Find the oldest thread
		var oldestThreadID int
		err = tx.QueryRow("SELECT id FROM threads WHERE board_tag = ? ORDER BY created_at ASC LIMIT 1", boardTag).Scan(&oldestThreadID)
		if err != nil {
			return nil, err
		}

		// Collect image URLs from the thread and its posts before deleting
		rows, err := tx.Query(`
			SELECT image_url FROM threads WHERE id = ?
			UNION ALL
			SELECT image_url FROM posts WHERE thread_id = ?
		`, oldestThreadID, oldestThreadID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var img sql.NullString
			if err := rows.Scan(&img); err != nil {
				continue // Or handle error
			}
			if img.Valid && img.String != "" {
				imagesToDelete = append(imagesToDelete, img.String)
			}
		}

		// Delete replies
		_, err = tx.Exec("DELETE FROM posts WHERE thread_id = ?", oldestThreadID)
		if err != nil {
			return nil, err
		}

		// Delete the thread itself
		_, err = tx.Exec("DELETE FROM threads WHERE id = ?", oldestThreadID)
		if err != nil {
			return nil, err
		}
	}

	// 3. Insert the new thread
	threadStmt, err := tx.Prepare(`
		INSERT INTO threads (board_tag, subject, comment, image_url) 
		VALUES (?, ?, ?, ?);
	`)
	if err != nil {
		return nil, err
	}
	defer threadStmt.Close()

	result, err := threadStmt.Exec(boardTag, subject, comment, imageURL)
	if err != nil {
		return nil, err
	}

	threadID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	// 4. Insert the original post (OP)
	postStmt, err := tx.Prepare(`
		INSERT INTO posts (thread_id, comment, image_url) 
		VALUES (?, ?, ?);
	`)
	if err != nil {
		return nil, err
	}
	defer postStmt.Close()

	_, err = postStmt.Exec(threadID, comment, imageURL)
	if err != nil {
		return nil, err
	}

	// 5. Commit the transaction
	return imagesToDelete, tx.Commit()
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

// CreateReply inserts a new post into an existing thread.
// If the post count reaches 500, it deletes the thread and returns its images.
func CreateReply(threadID int, comment, imageURL string) ([]string, bool, error) {
	const postLimit = 500

	// Start a transaction
	tx, err := DB.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()

	// 1. Get current post count
	var count int
	err = tx.QueryRow("SELECT COUNT(*) FROM posts WHERE thread_id = ?", threadID).Scan(&count)
	if err != nil {
		return nil, false, err
	}

	// 2. If the new post would reach the limit, prune the thread
	if count+1 >= postLimit {
		// Collect images before deleting
		var images []string
		rows, err := tx.Query(`
			SELECT image_url FROM threads WHERE id = ?
			UNION ALL
			SELECT image_url FROM posts WHERE thread_id = ?
		`, threadID, threadID)
		if err != nil {
			return nil, false, err
		}
		defer rows.Close()

		for rows.Next() {
			var img sql.NullString
			if err := rows.Scan(&img); err != nil {
				continue
			}
			if img.Valid && img.String != "" {
				images = append(images, img.String)
			}
		}

		// Delete posts
		_, err = tx.Exec("DELETE FROM posts WHERE thread_id = ?", threadID)
		if err != nil {
			return nil, false, err
		}

		// Delete thread
		_, err = tx.Exec("DELETE FROM threads WHERE id = ?", threadID)
		if err != nil {
			return nil, false, err
		}

		err = tx.Commit()
		return images, true, err
	}

	// 3. Otherwise, just insert the new post
	statement, err := tx.Prepare("INSERT INTO posts (thread_id, comment, image_url) VALUES (?, ?, ?)")
	if err != nil {
		return nil, false, err
	}
	defer statement.Close()

	_, err = statement.Exec(threadID, comment, imageURL)
	if err != nil {
		return nil, false, err
	}

	err = tx.Commit()
	return nil, false, err
}

func GetThreads(boardTag string) ([]Thread, error) {
	// Query threads specifically for this board, ordered by newest first
	// We select the OP data stored directly on the thread record
	rows, err := DB.Query(`
		SELECT id, subject, comment, image_url, created_at
		FROM threads
		WHERE board_tag = ?
		ORDER BY id DESC`, boardTag)
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

// database.go

// DeleteThread deletes a thread, its replies, and returns all associated image paths
func DeleteThread(threadID int) ([]string, error) {
	// 1. Collect all image URLs associated with this thread (OP + Replies)
	// We need these to delete the actual files from the disk later
	rows, err := DB.Query(`
		SELECT image_url FROM threads WHERE id = ?
		UNION
		SELECT image_url FROM posts WHERE thread_id = ?
	`, threadID, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []string
	for rows.Next() {
		var img sql.NullString
		if err := rows.Scan(&img); err != nil {
			continue
		}
		if img.Valid && img.String != "" {
			images = append(images, img.String)
		}
	}

	// 2. Delete the records from the database
	tx, err := DB.Begin()
	if err != nil {
		return nil, err
	}

	// Delete replies first (foreign key constraint usually handles this, but let's be explicit)
	_, err = tx.Exec("DELETE FROM posts WHERE thread_id = ?", threadID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Delete the thread/OP
	_, err = tx.Exec("DELETE FROM threads WHERE id = ?", threadID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = tx.Commit()
	return images, err
}

// DeletePost deletes a single reply and returns its image path (if any)
func DeletePost(postID int) (string, error) {
	// 1. Get the image URL
	var img sql.NullString
	err := DB.QueryRow("SELECT image_url FROM posts WHERE id = ?", postID).Scan(&img)
	if err != nil {
		return "", err
	}

	// 2. Delete the record
	_, err = DB.Exec("DELETE FROM posts WHERE id = ?", postID)
	if err != nil {
		return "", err
	}

	if img.Valid {
		return img.String, nil
	}
	return "", nil
}
