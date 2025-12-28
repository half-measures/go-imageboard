package main

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) {
	var err error
	DB, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	createTables()
	seedBoards()
}

func teardownTestDB() {
	if DB != nil {
		DB.Close()
	}
}

func TestGetBoards(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	boards, err := GetBoards()
	if err != nil {
		t.Fatalf("GetBoards failed: %v", err)
	}

	if len(boards) != 3 {
		t.Errorf("Expected 3 boards, got %d", len(boards))
	}
}

func TestCreateThreadAndOP(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	boardTag := "v"
	subject := "Test Thread"
	comment := "Test Comment"
	imageURL := "/uploads/test.jpg"

	images, err := CreateThreadAndOP(boardTag, subject, comment, imageURL)
	if err != nil {
		t.Fatalf("CreateThreadAndOP failed: %v", err)
	}

	if len(images) != 0 {
		t.Errorf("Expected 0 images to delete, got %d", len(images))
	}

	thread, err := GetThreads(boardTag)
	if err != nil {
		t.Fatalf("GetThreads failed: %v", err)
	}

	if len(thread) != 1 {
		t.Errorf("Expected 1 thread, got %d", len(thread))
	}

	if thread[0].Subject != subject {
		t.Errorf("Expected subject %s, got %s", subject, thread[0].Subject)
	}
}

func TestPruning(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	boardTag := "v"
	// limit is 3, so let's add 4 threads
	for i := 0; i < 4; i++ {
		_, err := CreateThreadAndOP(boardTag, "Subject", "Comment", "")
		if err != nil {
			t.Fatalf("CreateThreadAndOP failed at index %d: %v", i, err)
		}
	}

	threads, err := GetThreads(boardTag)
	if err != nil {
		t.Fatalf("GetThreads failed: %v", err)
	}

	if len(threads) != 3 {
		t.Errorf("Expected 3 threads after pruning, got %d", len(threads))
	}
}

func TestCreateReply(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	boardTag := "v"
	_, err := CreateThreadAndOP(boardTag, "Subject", "Comment", "")
	if err != nil {
		t.Fatalf("CreateThreadAndOP failed: %v", err)
	}

	threads, _ := GetThreads(boardTag)
	threadID := threads[0].ID

	images, pruned, err := CreateReply(threadID, "Reply Comment", "/uploads/reply.jpg")
	if err != nil {
		t.Fatalf("CreateReply failed: %v", err)
	}

	if pruned {
		t.Errorf("Thread should not be pruned yet")
	}

	if len(images) != 0 {
		t.Errorf("Expected 0 images to delete, got %d", len(images))
	}

	thread, err := GetThread(threadID)
	if err != nil {
		t.Fatalf("GetThread failed: %v", err)
	}

	if len(thread.Replies) != 1 {
		t.Errorf("Expected 1 reply, got %d", len(thread.Replies))
	}
}

func TestDeleteThread(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	boardTag := "v"
	CreateThreadAndOP(boardTag, "Subject", "Comment", "/uploads/op.jpg")
	threads, _ := GetThreads(boardTag)
	threadID := threads[0].ID

	images, err := DeleteThread(threadID)
	if err != nil {
		t.Fatalf("DeleteThread failed: %v", err)
	}

	if len(images) != 1 {
		t.Errorf("Expected 1 image (from thread OP), got %d", len(images))
	}

	if images[0] != "/uploads/op.jpg" {
		t.Errorf("Expected image /uploads/op.jpg, got %s", images[0])
	}

	// Verify thread is gone
	_, err = GetThread(threadID)
	if err == nil {
		t.Errorf("Expected error when getting deleted thread, got nil")
	}
}
