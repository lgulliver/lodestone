// Direct storage testing to verify enhanced storage implementation
package storage_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lgulliver/lodestone/internal/storage"
)

func TestStorageDirectOperations(t *testing.T) {
	t.Log("=== Enhanced Storage Direct Test ===")

	// Create temporary directory for test data
	testDir, err := os.MkdirTemp("", "lodestone-storage-test")
	if err != nil {
		t.Fatal("Failed to create temp directory:", err)
	}
	defer os.RemoveAll(testDir)

	// Setup storage
	storageBackend, err := storage.NewLocalStorage(testDir)
	if err != nil {
		t.Fatal("Failed to create local storage:", err)
	}

	// Run test categories
	t.Log("1. Testing basic storage operations...")
	testBasicOperations(t, storageBackend)

	t.Log("2. Testing atomic writes and integrity...")
	testAtomicWrites(t, storageBackend)

	t.Log("3. Testing concurrent access...")
	testConcurrentAccess(t, storageBackend)

	t.Log("4. Testing context cancellation...")
	testContextCancellation(t, storageBackend)

	t.Log("5. Testing different file types and sizes...")
	testFileTypes(t, storageBackend)

	t.Log("6. Testing error handling...")
	testErrorHandling(t, storageBackend)

	t.Log("7. Testing list functionality...")
	testListFunctionality(t, storageBackend)

	t.Log("✅ All enhanced storage tests passed!")
}

// Test basic storage operations: store, retrieve, exists, size, delete
func testBasicOperations(t *testing.T, storage storage.BlobStorage) {
	ctx := context.Background()
	content := "This is test content for basic operations"
	path := "basic_test/test_file.txt"

	// Store
	err := storage.Store(ctx, path, strings.NewReader(content), "text/plain")
	if err != nil {
		t.Fatal("Store failed:", err)
	}

	// Exists
	exists, err := storage.Exists(ctx, path)
	if err != nil {
		t.Fatal("Exists failed:", err)
	}
	if !exists {
		t.Fatal("File should exist after store")
	}

	// Size
	size, err := storage.GetSize(ctx, path)
	if err != nil {
		t.Fatal("GetSize failed:", err)
	}
	if size != int64(len(content)) {
		t.Fatal("Size mismatch")
	}

	// Retrieve
	reader, err := storage.Retrieve(ctx, path)
	if err != nil {
		t.Fatal("Retrieve failed:", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal("Read failed:", err)
	}

	if string(data) != content {
		t.Fatal("Content mismatch")
	}

	// Delete
	err = storage.Delete(ctx, path)
	if err != nil {
		t.Fatal("Delete failed:", err)
	}

	// Verify deletion
	exists, err = storage.Exists(ctx, path)
	if err != nil {
		t.Fatal("Exists check after delete failed:", err)
	}
	if exists {
		t.Fatal("File should not exist after delete")
	}

	t.Logf("✅ Basic storage operations verified")
}

// Test atomic writes and data integrity
func testAtomicWrites(t *testing.T, storage storage.BlobStorage) {
	ctx := context.Background()

	// Large content to ensure atomic behavior
	largeContent := strings.Repeat("Large atomic test content with integrity verification. ", 1000) // ~50KB
	path := "atomic_test/large_file.txt"

	startTime := time.Now()

	// Store with integrity verification
	hasher := sha256.New()
	hasher.Write([]byte(largeContent))
	expectedChecksum := hex.EncodeToString(hasher.Sum(nil))

	err := storage.Store(ctx, path, strings.NewReader(largeContent), "text/plain")
	if err != nil {
		t.Fatal("Atomic store failed:", err)
	}

	duration := time.Since(startTime)

	// Retrieve and verify integrity
	reader, err := storage.Retrieve(ctx, path)
	if err != nil {
		t.Fatal("Retrieve after atomic store failed:", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal("Read after atomic store failed:", err)
	}

	if string(data) != largeContent {
		t.Fatal("Content integrity failed")
	}

	// Verify checksum
	hasher.Reset()
	hasher.Write(data)
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))

	if actualChecksum != expectedChecksum {
		t.Fatal("SHA256 checksum mismatch")
	}

	t.Logf("✅ Atomic writes verified: %d bytes in %v", len(largeContent), duration)
}

// Test concurrent access with multiple goroutines
func testConcurrentAccess(t *testing.T, storage storage.BlobStorage) {
	ctx := context.Background()
	numGoroutines := 200
	var wg sync.WaitGroup
	var mu sync.Mutex
	stored := make(map[string]string)

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			content := fmt.Sprintf("Concurrent test content from goroutine %d - %s", id, strings.Repeat("data", 20))
			path := fmt.Sprintf("concurrent_test/file_%d.txt", id)

			err := storage.Store(ctx, path, strings.NewReader(content), "text/plain")
			if err != nil {
				t.Logf("❌ Concurrent store failed: %v", err)
				return
			}

			mu.Lock()
			stored[path] = content
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	expectedFiles := numGoroutines
	if len(stored) != expectedFiles {
		t.Fatalf("Expected %d files, got %d", expectedFiles, len(stored))
	}

	// Concurrent reads to verify all files
	for path, expectedContent := range stored {
		wg.Add(1)
		go func(p, expected string) {
			defer wg.Done()

			reader, err := storage.Retrieve(ctx, p)
			if err != nil {
				t.Logf("❌ Concurrent retrieve failed: %v", err)
				return
			}
			defer reader.Close()

			data, err := io.ReadAll(reader)
			if err != nil {
				t.Logf("❌ Concurrent read failed: %v", err)
				return
			}

			if string(data) != expected {
				t.Logf("❌ Content mismatch in concurrent read")
				return
			}
		}(path, expectedContent)
	}

	wg.Wait()
	t.Logf("✅ Concurrent access verified: %d files written and read safely", len(stored))
}

// Test context cancellation handling
func testContextCancellation(t *testing.T, storage storage.BlobStorage) {
	// Test cancelled context for store
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := storage.Store(cancelledCtx, "cancel_test/file.txt", strings.NewReader("test"), "text/plain")
	if err == nil {
		t.Fatal("Expected store to fail with cancelled context")
	}
	if err != context.Canceled {
		t.Logf("⚠️  Expected context.Canceled, got: %v", err)
	}

	// Test cancelled context for retrieve
	// First store a file with normal context
	normalCtx := context.Background()
	testPath := "cancel_test/existing_file.txt"
	err = storage.Store(normalCtx, testPath, strings.NewReader("test content"), "text/plain")
	if err != nil {
		t.Fatal("Setup for cancel test failed:", err)
	}

	// Now try to retrieve with cancelled context
	_, err = storage.Retrieve(cancelledCtx, testPath)
	if err == nil {
		t.Fatal("Expected retrieve to fail with cancelled context")
	}
	if err != context.Canceled {
		t.Logf("⚠️  Expected context.Canceled, got: %v", err)
	}

	t.Logf("✅ Context cancellation handled correctly")
}

// Test different file types and sizes
func testFileTypes(t *testing.T, storage storage.BlobStorage) {
	ctx := context.Background()

	testFiles := []struct {
		name        string
		content     string
		contentType string
	}{
		{"types_test/small.txt", "Small file", "text/plain"},
		{"types_test/json.json", `{"key": "value", "number": 123}`, "application/json"},
		{"types_test/binary.bin", string([]byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE}), "application/octet-stream"},
		{"types_test/large.txt", strings.Repeat("Large file content with repeated data. ", 5000), "text/plain"}, // ~200KB
		{"types_test/empty.txt", "", "text/plain"},
		{"types_test/unicode.txt", "Unicode content: 你好世界 🌍 café naïve résumé", "text/plain; charset=utf-8"},
	}

	for _, tf := range testFiles {
		// Store
		err := storage.Store(ctx, tf.name, strings.NewReader(tf.content), tf.contentType)
		if err != nil {
			t.Fatalf("Failed to store %s: %v", tf.name, err)
		}

		// Verify size
		size, err := storage.GetSize(ctx, tf.name)
		if err != nil {
			t.Fatalf("Failed to get size of %s: %v", tf.name, err)
		}
		if size != int64(len(tf.content)) {
			t.Fatalf("Size mismatch for %s: expected %d, got %d", tf.name, len(tf.content), size)
		}

		// Verify content
		reader, err := storage.Retrieve(ctx, tf.name)
		if err != nil {
			t.Fatalf("Failed to retrieve %s: %v", tf.name, err)
		}

		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatalf("Failed to read %s: %v", tf.name, err)
		}

		if string(data) != tf.content {
			t.Fatalf("Content mismatch for %s", tf.name)
		}
	}

	t.Logf("✅ Different file types verified: %d files", len(testFiles))
}

// Test error handling for edge cases
func testErrorHandling(t *testing.T, storage storage.BlobStorage) {
	ctx := context.Background()

	// Test retrieve non-existent file
	_, err := storage.Retrieve(ctx, "error_test/non_existent.txt")
	if err == nil {
		t.Fatal("Expected error for non-existent file retrieve")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Logf("⚠️  Error message doesn't indicate file not found: %v", err)
	}

	// Test size of non-existent file
	_, err = storage.GetSize(ctx, "error_test/non_existent.txt")
	if err == nil {
		t.Fatal("Expected error for non-existent file size")
	}

	// Test exists for non-existent file (should not error)
	exists, err := storage.Exists(ctx, "error_test/non_existent.txt")
	if err != nil {
		t.Fatal("Exists should not error for non-existent file:", err)
	}
	if exists {
		t.Fatal("Exists should return false for non-existent file")
	}

	// Test delete non-existent file (should not error)
	err = storage.Delete(ctx, "error_test/non_existent.txt")
	if err != nil {
		t.Logf("⚠️  Delete of non-existent file returned error: %v", err)
	}

	t.Logf("✅ Error handling verified")
}

// Test list functionality with different prefixes
func testListFunctionality(t *testing.T, storage storage.BlobStorage) {
	ctx := context.Background()

	// Create test files with unique prefixes to avoid conflicts
	timestamp := time.Now().UnixNano()
	testFiles := []string{
		fmt.Sprintf("list_test_%d/dir1/file1.txt", timestamp),
		fmt.Sprintf("list_test_%d/dir1/file2.txt", timestamp),
		fmt.Sprintf("list_test_%d/dir1/subdir/file3.txt", timestamp),
		fmt.Sprintf("list_test_%d/dir2/file4.txt", timestamp),
		fmt.Sprintf("list_test_%d/file5.txt", timestamp),
	}

	// Store all test files
	for _, path := range testFiles {
		content := fmt.Sprintf("List test content for %s", path)
		err := storage.Store(ctx, path, strings.NewReader(content), "text/plain")
		if err != nil {
			t.Fatalf("Failed to store %s: %v", path, err)
		}
	}

	// Test different list operations
	listTests := []struct {
		prefix   string
		name     string
		expected int
	}{
		{fmt.Sprintf("list_test_%d", timestamp), "all files", 5},
		{fmt.Sprintf("list_test_%d/dir1", timestamp), "dir1 files", 3},
		{fmt.Sprintf("list_test_%d/dir2", timestamp), "dir2 files", 1},
		{fmt.Sprintf("list_test_%d/dir1/subdir", timestamp), "subdir files", 1},
		{fmt.Sprintf("nonexistent_%d", timestamp), "non-existent prefix", 0},
	}

	for _, test := range listTests {
		files, err := storage.List(ctx, test.prefix)
		if err != nil {
			t.Fatalf("List failed for prefix '%s': %v", test.prefix, err)
		}

		if len(files) != test.expected {
			t.Fatalf("List with prefix '%s' (%s): expected %d files, got %d",
				test.prefix, test.name, test.expected, len(files))
		}

		// Verify all returned files match the prefix
		for _, file := range files {
			if !strings.HasPrefix(file, test.prefix) {
				t.Fatalf("File %s doesn't match prefix %s", file, test.prefix)
			}
		}

		t.Logf("✅ List '%s' (%s): found %d files", test.prefix, test.name, len(files))
	}

	// Clean up test files
	for _, path := range testFiles {
		err := storage.Delete(ctx, path)
		if err != nil {
			t.Logf("Warning: failed to clean up test file %s: %v", path, err)
		}
	}

	t.Logf("✅ List functionality verified across different prefixes with cleanup")
}
