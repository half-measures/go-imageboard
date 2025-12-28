// upload.go
package main

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The directory where all user images will be saved.
const UploadDir = "./uploads"

func init() {
	// Ensure the upload directory exists when the program starts
	if err := os.MkdirAll(UploadDir, 0755); err != nil {
		fmt.Printf("Error creating upload directory %s: %v\n", UploadDir, err)
		os.Exit(1)
	}
}

// SaveFile saves the uploaded file and returns its resulting public URL path.
func SaveFile(file multipart.File, header *multipart.FileHeader) (string, error) {
	// 1. Generate a unique filename to prevent collisions.
	// We use timestamp + a random-ish part of the original filename.
	extension := filepath.Ext(header.Filename)
	uniqueFilename := fmt.Sprintf("%d_%s%s",
		time.Now().UnixNano(),
		filepath.Base(header.Filename)[:5], // Use first 5 chars for a hint
		extension)

	// 2. Define the full file path on the server
	filePath := filepath.Join(UploadDir, uniqueFilename)

	// 3. Create the new file on the server's disk
	out, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// 4. Copy the uploaded file content to the new file
	_, err = io.Copy(out, file)
	if err != nil {
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}

	// 5. Return the URL path, which will be served via the static handler
	return "/uploads/" + uniqueFilename, nil
}

// IsAllowedExtension checks if the file extension is supported.
func IsAllowedExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".mp4", ".webm":
		return true
	}
	return false
}

// DeleteFiles removes the specified files from the disk.
// Paths are expected to be public URL paths like "/uploads/filename.jpg".
func DeleteFiles(filePaths []string) {
	for _, imgPath := range filePaths {
		// Strip the leading slash to make it relative to our project root
		// e.g. "/uploads/abc.jpg" -> "uploads/abc.jpg"
		relativePath := strings.TrimPrefix(imgPath, "/")
		err := os.Remove(relativePath)
		if err != nil {
			log.Printf("Failed to delete file %s: %v\n", relativePath, err)
		} else {
			log.Printf("Deleted file: %s\n", relativePath)
		}
	}
}
