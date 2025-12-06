package importer

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ExtractZIP extracts a ZIP file to a temporary directory and validates structure
func ExtractZIP(zipPath string) (extractedDir string, cleanup func(), err error) {
	// Create unique temp directory
	extractedDir, err = os.MkdirTemp("", "telegram-import-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	cleanup = func() {
		os.RemoveAll(extractedDir)
	}

	// Open ZIP file
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to open ZIP file: %w", err)
	}
	defer r.Close()

	// Extract all files
	for _, f := range r.File {
		if err := extractFile(f, extractedDir); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("failed to extract file %s: %w", f.Name, err)
		}
	}

	// Validate required files exist
	resultJSONPath := filepath.Join(extractedDir, "result.json")
	if _, err := os.Stat(resultJSONPath); os.IsNotExist(err) {
		cleanup()
		return "", nil, fmt.Errorf("invalid Telegram export: result.json not found")
	}

	return extractedDir, cleanup, nil
}

// extractFile extracts a single file from ZIP archive
func extractFile(f *zip.File, destDir string) error {
	// Build destination path
	destPath := filepath.Join(destDir, f.Name)

	// Create directory structure if needed
	if f.FileInfo().IsDir() {
		return os.MkdirAll(destPath, f.Mode())
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Open source file from ZIP
	srcFile, err := f.Open()
	if err != nil {
		return fmt.Errorf("failed to open file in ZIP: %w", err)
	}
	defer srcFile.Close()

	// Create destination file
	destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Copy content
	if _, err := io.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}
