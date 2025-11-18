// models.go
package main

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

// We'll add Thread and Post structs here later
