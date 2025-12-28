package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"os"
	"strings"
	"testing"
)

func TestSaveFile(t *testing.T) {
	// Create a temporary upload directory for testing
	tempUploadDir := "./test_uploads"
	os.MkdirAll(tempUploadDir, 0755)
	defer os.RemoveAll(tempUploadDir)

	// Mocking UploadDir (a bit hacky since it's a const, but let's see)
	// Actually, UploadDir is a const, so we can't change it.
	// We'll have to use the real UploadDir but clean up.

	content := "test image content"
	filename := "test.jpg"

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("image", filename)
	io.WriteString(part, content)
	writer.Close()

	// Parse the multipart form to get a multipart.File and FileHeader
	reader := multipart.NewReader(body, writer.Boundary())
	form, err := reader.ReadForm(10 << 20)
	if err != nil {
		t.Fatalf("ReadForm failed: %v", err)
	}

	fileHeader := form.File["image"][0]
	file, _ := fileHeader.Open()
	defer file.Close()

	url, err := SaveFile(file, fileHeader)
	if err != nil {
		t.Fatalf("SaveFile failed: %v", err)
	}

	if !strings.HasPrefix(url, "/uploads/") {
		t.Errorf("Expected URL to start with /uploads/, got %s", url)
	}

	// Verify file exists on disk
	relPath := strings.TrimPrefix(url, "/")
	if _, err := os.Stat(relPath); os.IsNotExist(err) {
		t.Errorf("File was not saved to disk: %s", relPath)
	}

	// Clean up
	os.Remove(relPath)
}

func TestDeleteFiles(t *testing.T) {
	// Create a dummy file
	testFile := "uploads/test_delete.jpg"
	os.MkdirAll("uploads", 0755)
	err := os.WriteFile(testFile, []byte("data"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	DeleteFiles([]string{"/uploads/test_delete.jpg"})

	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Errorf("File was not deleted from disk")
	}
}
