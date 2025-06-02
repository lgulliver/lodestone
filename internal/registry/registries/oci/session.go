package oci

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/lgulliver/lodestone/internal/storage"
	"github.com/rs/zerolog/log"
)

// NewSessionManager creates a new session manager
func NewSessionManager(storage storage.BlobStorage) *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*UploadSession),
		storage:  storage,
	}

	// Start cleanup routine
	go sm.cleanupRoutine()

	return sm
}

// StartUpload creates a new upload session
func (sm *SessionManager) StartUpload(ctx context.Context, repository, userID string) (*UploadSession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sessionID := uuid.New().String()
	tempPath := fmt.Sprintf("temp/uploads/%s/%s", repository, sessionID)

	session := &UploadSession{
		ID:         sessionID,
		Repository: repository,
		UserID:     userID,
		StartedAt:  time.Now(),
		LastUpdate: time.Now(),
		Size:       0,
		TempPath:   tempPath,
	}

	sm.sessions[sessionID] = session

	log.Info().
		Str("session_id", sessionID).
		Str("repository", repository).
		Str("user_id", userID).
		Msg("Started blob upload session")

	return session, nil
}

// GetSession retrieves an upload session
func (sm *SessionManager) GetSession(sessionID string) (*UploadSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, false
	}

	// Update last access time
	session.mu.Lock()
	session.LastUpdate = time.Now()
	session.mu.Unlock()

	return session, true
}

// AppendChunk appends data to an upload session
func (sm *SessionManager) AppendChunk(ctx context.Context, sessionID string, data io.Reader, contentRange string) (*UploadSession, error) {
	session, exists := sm.GetSession(sessionID)
	if !exists {
		return nil, fmt.Errorf("upload session not found")
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// For simplicity, we'll read all data and store it
	// In a production system, you'd handle chunked uploads more efficiently
	content, err := io.ReadAll(data)
	if err != nil {
		return nil, fmt.Errorf("failed to read chunk data: %w", err)
	}

	// If this is the first chunk, create the temp file
	if session.Size == 0 {
		err = sm.storage.Store(ctx, session.TempPath, bytes.NewReader(content), "application/octet-stream")
		if err != nil {
			return nil, fmt.Errorf("failed to store initial chunk: %w", err)
		}
	} else {
		// For subsequent chunks, we'd need to append to the existing file
		// This is a simplified implementation - in production, you'd use a more sophisticated approach
		existing, err := sm.storage.Retrieve(ctx, session.TempPath)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve existing data: %w", err)
		}

		existingData, err := io.ReadAll(existing)
		existing.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read existing data: %w", err)
		}

		// Combine existing and new data
		combined := append(existingData, content...)
		err = sm.storage.Store(ctx, session.TempPath, bytes.NewReader(combined), "application/octet-stream")
		if err != nil {
			return nil, fmt.Errorf("failed to store combined data: %w", err)
		}
	}

	session.Size += int64(len(content))
	session.LastUpdate = time.Now()

	log.Debug().
		Str("session_id", sessionID).
		Int64("chunk_size", int64(len(content))).
		Int64("total_size", session.Size).
		Msg("Appended chunk to upload session")

	return session, nil
}

// CompleteUpload finalizes an upload session with digest verification
func (sm *SessionManager) CompleteUpload(ctx context.Context, sessionID, expectedDigest string) (*UploadSession, string, error) {
	session, exists := sm.GetSession(sessionID)
	if !exists {
		return nil, "", fmt.Errorf("upload session not found")
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// Retrieve the uploaded data for digest verification
	reader, err := sm.storage.Retrieve(ctx, session.TempPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to retrieve uploaded data: %w", err)
	}
	defer reader.Close()

	// Calculate SHA256 digest
	hasher := sha256.New()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read uploaded data: %w", err)
	}

	hasher.Write(data)
	actualDigest := "sha256:" + hex.EncodeToString(hasher.Sum(nil))

	// Verify digest if provided
	if expectedDigest != "" && expectedDigest != actualDigest {
		return nil, "", fmt.Errorf("digest mismatch: expected %s, got %s", expectedDigest, actualDigest)
	}

	session.Digest = actualDigest
	session.LastUpdate = time.Now()

	// Generate final storage path
	finalPath := fmt.Sprintf("oci/%s/blobs/%s", session.Repository, actualDigest)

	// Move from temp location to final location
	err = sm.storage.Store(ctx, finalPath, bytes.NewReader(data), "application/octet-stream")
	if err != nil {
		return nil, "", fmt.Errorf("failed to store blob at final location: %w", err)
	}

	// Clean up temp file
	sm.storage.Delete(ctx, session.TempPath)

	log.Info().
		Str("session_id", sessionID).
		Str("digest", actualDigest).
		Str("repository", session.Repository).
		Int64("size", session.Size).
		Msg("Completed blob upload")

	return session, finalPath, nil
}

// CancelUpload cancels an upload session
func (sm *SessionManager) CancelUpload(ctx context.Context, sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("upload session not found")
	}

	// Clean up temp file
	sm.storage.Delete(ctx, session.TempPath)

	// Remove session
	delete(sm.sessions, sessionID)

	log.Info().
		Str("session_id", sessionID).
		Str("repository", session.Repository).
		Msg("Cancelled blob upload session")

	return nil
}

// GetUploadStatus returns the current status of an upload session
func (sm *SessionManager) GetUploadStatus(sessionID string) (*UploadSession, error) {
	session, exists := sm.GetSession(sessionID)
	if !exists {
		return nil, fmt.Errorf("upload session not found")
	}

	return session, nil
}

// cleanupRoutine periodically cleans up expired sessions
func (sm *SessionManager) cleanupRoutine() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		sm.cleanupExpiredSessions()
	}
}

// cleanupExpiredSessions removes sessions that have been inactive for too long
func (sm *SessionManager) cleanupExpiredSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	expiry := time.Now().Add(-24 * time.Hour) // Sessions expire after 24 hours
	var expiredSessions []string

	for sessionID, session := range sm.sessions {
		session.mu.RLock()
		lastUpdate := session.LastUpdate
		tempPath := session.TempPath
		session.mu.RUnlock()

		if lastUpdate.Before(expiry) {
			expiredSessions = append(expiredSessions, sessionID)
			// Clean up temp file
			sm.storage.Delete(context.Background(), tempPath)
		}
	}

	for _, sessionID := range expiredSessions {
		delete(sm.sessions, sessionID)
		log.Info().
			Str("session_id", sessionID).
			Msg("Cleaned up expired upload session")
	}

	if len(expiredSessions) > 0 {
		log.Info().
			Int("count", len(expiredSessions)).
			Msg("Cleaned up expired upload sessions")
	}
}
