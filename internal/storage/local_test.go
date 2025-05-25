package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLocalStorage(t *testing.T) {
	tests := []struct {
		name        string
		basePath    string
		shouldError bool
	}{
		{
			name:        "valid path",
			basePath:    t.TempDir(),
			shouldError: false,
		},
		{
			name:        "non-existent path",
			basePath:    filepath.Join(t.TempDir(), "nested", "path"),
			shouldError: false,
		},
		{
			name:        "invalid path (file instead of directory)",
			basePath:    createTempFile(t),
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := NewLocalStorage(tt.basePath)

			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, storage)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, storage)
				assert.Equal(t, tt.basePath, storage.basePath)

				// Verify directory was created
				info, err := os.Stat(tt.basePath)
				assert.NoError(t, err)
				assert.True(t, info.IsDir())
			}
		})
	}
}

func TestLocalStorage_Store(t *testing.T) {
	storage := setupTestStorage(t)
	ctx := context.Background()

	tests := []struct {
		name        string
		path        string
		content     string
		contentType string
		shouldError bool
	}{
		{
			name:        "simple file",
			path:        "test.txt",
			content:     "hello world",
			contentType: "text/plain",
			shouldError: false,
		},
		{
			name:        "nested path",
			path:        "nested/dir/test.txt",
			content:     "nested content",
			contentType: "text/plain",
			shouldError: false,
		},
		{
			name:        "binary content",
			path:        "binary.bin",
			content:     string([]byte{0x00, 0x01, 0x02, 0xFF}),
			contentType: "application/octet-stream",
			shouldError: false,
		},
		{
			name:        "empty content",
			path:        "empty.txt",
			content:     "",
			contentType: "text/plain",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.content)
			err := storage.Store(ctx, tt.path, reader, tt.contentType)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify file exists
				exists, err := storage.Exists(ctx, tt.path)
				assert.NoError(t, err)
				assert.True(t, exists)

				// Verify content
				retrieved, err := storage.Retrieve(ctx, tt.path)
				assert.NoError(t, err)
				defer retrieved.Close()

				content, err := io.ReadAll(retrieved)
				assert.NoError(t, err)
				assert.Equal(t, tt.content, string(content))
			}
		})
	}
}

func TestLocalStorage_StoreAtomic(t *testing.T) {
	storage := setupTestStorage(t)
	ctx := context.Background()

	// Test that failed writes don't leave partial files
	t.Run("failed write cleanup", func(t *testing.T) {
		// Create a reader that will fail after some data
		failingReader := &failingReader{
			data:      []byte("some data"),
			failAfter: 5,
		}

		err := storage.Store(ctx, "failing.txt", failingReader, "text/plain")
		assert.Error(t, err)

		// Verify no file was left behind
		exists, err := storage.Exists(ctx, "failing.txt")
		assert.NoError(t, err)
		assert.False(t, exists)

		// Verify no temp files are left
		files, err := os.ReadDir(storage.basePath)
		assert.NoError(t, err)
		for _, file := range files {
			assert.False(t, strings.Contains(file.Name(), ".tmp."),
				"temp file should not exist: %s", file.Name())
		}
	})
}

func TestLocalStorage_Retrieve(t *testing.T) {
	storage := setupTestStorage(t)
	ctx := context.Background()

	// Store test content
	testContent := "test content for retrieval"
	err := storage.Store(ctx, "retrieve_test.txt", strings.NewReader(testContent), "text/plain")
	require.NoError(t, err)

	tests := []struct {
		name        string
		path        string
		shouldError bool
		expectedErr string
	}{
		{
			name:        "existing file",
			path:        "retrieve_test.txt",
			shouldError: false,
		},
		{
			name:        "non-existent file",
			path:        "non_existent.txt",
			shouldError: true,
			expectedErr: "file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := storage.Retrieve(ctx, tt.path)

			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				assert.Nil(t, reader)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, reader)
				defer reader.Close()

				content, err := io.ReadAll(reader)
				assert.NoError(t, err)
				assert.Equal(t, testContent, string(content))
			}
		})
	}
}

func TestLocalStorage_Delete(t *testing.T) {
	storage := setupTestStorage(t)
	ctx := context.Background()

	// Store test content
	testPath := "delete_test.txt"
	err := storage.Store(ctx, testPath, strings.NewReader("test content"), "text/plain")
	require.NoError(t, err)

	tests := []struct {
		name        string
		path        string
		shouldError bool
	}{
		{
			name:        "existing file",
			path:        testPath,
			shouldError: false,
		},
		{
			name:        "non-existent file",
			path:        "non_existent.txt",
			shouldError: false, // Should not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := storage.Delete(ctx, tt.path)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify file doesn't exist
				exists, err := storage.Exists(ctx, tt.path)
				assert.NoError(t, err)
				assert.False(t, exists)
			}
		})
	}
}

func TestLocalStorage_Exists(t *testing.T) {
	storage := setupTestStorage(t)
	ctx := context.Background()

	// Store test content
	testPath := "exists_test.txt"
	err := storage.Store(ctx, testPath, strings.NewReader("test content"), "text/plain")
	require.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "existing file",
			path:     testPath,
			expected: true,
		},
		{
			name:     "non-existent file",
			path:     "non_existent.txt",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := storage.Exists(ctx, tt.path)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, exists)
		})
	}
}

func TestLocalStorage_GetSize(t *testing.T) {
	storage := setupTestStorage(t)
	ctx := context.Background()

	// Store test content
	testContent := "test content with known size"
	testPath := "size_test.txt"
	err := storage.Store(ctx, testPath, strings.NewReader(testContent), "text/plain")
	require.NoError(t, err)

	tests := []struct {
		name         string
		path         string
		expectedSize int64
		shouldError  bool
		expectedErr  string
	}{
		{
			name:         "existing file",
			path:         testPath,
			expectedSize: int64(len(testContent)),
			shouldError:  false,
		},
		{
			name:        "non-existent file",
			path:        "non_existent.txt",
			shouldError: true,
			expectedErr: "file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, err := storage.GetSize(ctx, tt.path)

			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSize, size)
			}
		})
	}
}

func TestLocalStorage_List(t *testing.T) {
	storage := setupTestStorage(t)
	ctx := context.Background()

	// Store test files
	testFiles := []string{
		"file1.txt",
		"file2.txt",
		"nested/file3.txt",
		"nested/deeper/file4.txt",
	}

	for _, file := range testFiles {
		err := storage.Store(ctx, file, strings.NewReader("content"), "text/plain")
		require.NoError(t, err)
	}

	tests := []struct {
		name          string
		prefix        string
		expectedFiles []string
	}{
		{
			name:          "root level",
			prefix:        "",
			expectedFiles: testFiles,
		},
		{
			name:   "nested directory",
			prefix: "nested",
			expectedFiles: []string{
				"nested/file3.txt",
				"nested/deeper/file4.txt",
			},
		},
		{
			name:          "non-existent prefix",
			prefix:        "nonexistent",
			expectedFiles: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := storage.List(ctx, tt.prefix)
			assert.NoError(t, err)

			// Sort both slices for comparison
			assert.ElementsMatch(t, tt.expectedFiles, files)
		})
	}
}

func TestLocalStorage_ConcurrentAccess(t *testing.T) {
	storage := setupTestStorage(t)
	ctx := context.Background()

	// Test concurrent writes
	t.Run("concurrent writes", func(t *testing.T) {
		const numGoroutines = 10
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(index int) {
				defer wg.Done()

				path := fmt.Sprintf("concurrent_%d.txt", index)
				content := fmt.Sprintf("content from goroutine %d", index)

				err := storage.Store(ctx, path, strings.NewReader(content), "text/plain")
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()

		// Verify all files were created
		for i := 0; i < numGoroutines; i++ {
			path := fmt.Sprintf("concurrent_%d.txt", i)
			exists, err := storage.Exists(ctx, path)
			assert.NoError(t, err)
			assert.True(t, exists)
		}
	})

	// Test concurrent reads
	t.Run("concurrent reads", func(t *testing.T) {
		// Store a test file
		testPath := "concurrent_read.txt"
		testContent := "shared content for concurrent reads"
		err := storage.Store(ctx, testPath, strings.NewReader(testContent), "text/plain")
		require.NoError(t, err)

		const numGoroutines = 10
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()

				reader, err := storage.Retrieve(ctx, testPath)
				assert.NoError(t, err)
				defer reader.Close()

				content, err := io.ReadAll(reader)
				assert.NoError(t, err)
				assert.Equal(t, testContent, string(content))
			}()
		}

		wg.Wait()
	})
}

func TestLocalStorage_ContextCancellation(t *testing.T) {
	storage := setupTestStorage(t)

	t.Run("store with cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := storage.Store(ctx, "cancelled.txt", strings.NewReader("content"), "text/plain")
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("retrieve with cancelled context", func(t *testing.T) {
		// First store a file
		err := storage.Store(context.Background(), "retrieve_cancel.txt", strings.NewReader("content"), "text/plain")
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		reader, err := storage.Retrieve(ctx, "retrieve_cancel.txt")
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
		assert.Nil(t, reader)
	})
}

func TestLocalStorage_IntegrityVerification(t *testing.T) {
	storage := setupTestStorage(t)
	ctx := context.Background()

	testContent := "content for integrity verification"
	testPath := "integrity_test.txt"

	// Store content
	err := storage.Store(ctx, testPath, strings.NewReader(testContent), "text/plain")
	require.NoError(t, err)

	// Retrieve and verify content matches
	reader, err := storage.Retrieve(ctx, testPath)
	require.NoError(t, err)
	defer reader.Close()

	retrievedContent, err := io.ReadAll(reader)
	require.NoError(t, err)

	assert.Equal(t, testContent, string(retrievedContent))

	// Verify checksum would match (we can't easily test the logged checksum,
	// but we can verify the content integrity)
	expectedHash := sha256.Sum256([]byte(testContent))
	actualHash := sha256.Sum256(retrievedContent)
	assert.Equal(t, expectedHash, actualHash)
}

// Helper functions

func setupTestStorage(t *testing.T) *LocalStorage {
	tempDir := t.TempDir()
	storage, err := NewLocalStorage(tempDir)
	require.NoError(t, err)
	return storage
}

func createTempFile(t *testing.T) string {
	tempFile, err := os.CreateTemp("", "test")
	require.NoError(t, err)
	tempFile.Close()
	return tempFile.Name()
}

// failingReader is a test helper that fails after reading a certain number of bytes
type failingReader struct {
	data      []byte
	pos       int
	failAfter int
}

func (fr *failingReader) Read(p []byte) (n int, err error) {
	if fr.pos >= fr.failAfter {
		return 0, io.ErrUnexpectedEOF
	}

	if fr.pos >= len(fr.data) {
		return 0, io.EOF
	}

	n = copy(p, fr.data[fr.pos:])
	fr.pos += n

	if fr.pos >= fr.failAfter {
		return n, io.ErrUnexpectedEOF
	}

	return n, nil
}
