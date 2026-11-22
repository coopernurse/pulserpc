package playground

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// resolveSessionFile resolves a session-relative filename and guarantees that
// the result stays inside sessionDir (S1/D14). Absolute paths, Windows volume
// paths, and any cleaned path containing a ".." segment are rejected. Backslashes
// are treated as separators on every OS so the same input cannot escape on
// Windows while looking harmless on Unix.
func resolveSessionFile(sessionDir, filename string) (string, error) {
	if strings.TrimSpace(filename) == "" {
		return "", fmt.Errorf("file path is required")
	}

	// Reject absolute paths in both native and slash-normalized forms. On
	// Windows, filepath.IsAbs does not recognize "/foo", so check the leading
	// separator and volume name explicitly.
	normalized := strings.ReplaceAll(filename, "\\", "/")
	if strings.HasPrefix(normalized, "/") || filepath.IsAbs(filename) || filepath.VolumeName(filename) != "" {
		return "", fmt.Errorf("absolute file paths are not allowed: %s", filename)
	}

	cleaned := filepath.Clean(filepath.FromSlash(normalized))
	fullPath := filepath.Join(sessionDir, cleaned)

	rel, err := filepath.Rel(sessionDir, fullPath)
	if err != nil {
		return "", fmt.Errorf("invalid file path %s: %w", filename, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("file path escapes session directory: %s", filename)
	}

	return fullPath, nil
}

// GetFile retrieves the contents of a specific file in a session
func (m *Manager) GetFile(sessionID string, filename string) ([]byte, error) {
	session, ok := m.GetSession(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	filePath, err := resolveSessionFile(session.Dir, filename)
	if err != nil {
		return nil, err
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data, nil
}

// CreateZip creates a ZIP archive of all files in a session
func (m *Manager) CreateZip(sessionID string) ([]byte, error) {
	session, ok := m.GetSession(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Create a buffer to write the zip to
	var buf []byte
	// Use a temporary file to create the zip
	tmpFile, err := os.CreateTemp("", "pulserpc-playground-*.zip")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	tmpFile.Close()

	// Create zip writer
	zipWriter, err := os.Create(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipWriter.Close()

	zipWriterWriter := zip.NewWriter(zipWriter)
	defer zipWriterWriter.Close()

	// Add files to zip
	for _, file := range session.Files {
		filePath, err := resolveSessionFile(session.Dir, file)
		if err != nil {
			return nil, fmt.Errorf("invalid session file %s: %w", file, err)
		}

		// Open file
		fileReader, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", file, err)
		}

		// ZIP entry names must use forward slashes. filepath.Rel produces
		// backslashes on Windows, which would create malformed entries.
		entryName := filepath.ToSlash(file)
		writer, err := zipWriterWriter.Create(entryName)
		if err != nil {
			fileReader.Close()
			return nil, fmt.Errorf("failed to create zip entry for %s: %w", file, err)
		}

		// Copy file content to zip
		_, err = io.Copy(writer, fileReader)
		fileReader.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to write %s to zip: %w", file, err)
		}
	}

	// Close zip writer and file
	zipWriterWriter.Close()
	zipWriter.Close()

	// Read the created zip file
	buf, err = os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read zip file: %w", err)
	}

	return buf, nil
}

// GetZipFilename returns the suggested filename for a session's ZIP archive
func (s *Session) GetZipFilename() string {
	// Format: pulserpc-<runtime>-<timestamp>.zip
	timestamp := s.Created.Format("20060102-150405")
	safeRuntime := strings.ReplaceAll(s.Runtime, "/", "-")
	return fmt.Sprintf("pulserpc-%s-%s.zip", safeRuntime, timestamp)
}

// Delete deletes a session and its files
func (m *Manager) Delete(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Remove session directory
	if err := os.RemoveAll(session.Dir); err != nil {
		return fmt.Errorf("failed to remove session directory: %w", err)
	}

	// Remove from session map
	delete(m.sessions, sessionID)

	return nil
}

// CleanupNow forces an immediate cleanup of expired sessions
func (m *Manager) CleanupNow() {
	m.cleanup()
}

// GetSessionCount returns the number of active sessions
func (m *Manager) GetSessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// IsExpired checks if a session is expired based on the current time and max age
func (m *Manager) IsExpired(sessionID string) bool {
	session, ok := m.GetSession(sessionID)
	if !ok {
		return true
	}

	return time.Since(session.Created) > m.maxAge
}

// SetMaxAge sets the maximum age for sessions before cleanup
// This is primarily used for testing
func (m *Manager) SetMaxAge(maxAge time.Duration) {
	m.maxAge = maxAge
}
