// models.go (Updated)
package main

import "time"

// Board represents a single message board
type Board struct {
	Tag  string // e.g., "g"
	Name string // e.g., "Technology"
}

// PageData holds the data we'll pass to the homepage template
type PageData struct {
	Title  string
	Boards []Board
}

// --- New Structures for Threads and Posts ---

// Post represents a single user post (OP or reply)
type Post struct {
	ID        int
	ThreadID  int
	Comment   string
	ImageURL  string
	CreatedAt time.Time
}

// Thread represents the Original Post (OP) and the subsequent replies
type Thread struct {
	ID        int
	BoardTag  string
	Subject   string
	OP        Post   // The Original Post
	Replies   []Post // List of replies
	CreatedAt time.Time
}

// BoardPageData is the data structure for rendering the /g/ page, etc.
type BoardPageData struct {
	BoardTag  string
	BoardName string
	Threads   []Thread
}
