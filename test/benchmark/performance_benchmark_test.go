// Performance benchmark for enhanced storage implementation
package benchmark

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lgulliver/lodestone/internal/storage"
)

type BenchmarkResult struct {
	Operation    string
	TotalOps     int
	TotalTime    time.Duration
	AvgTime      time.Duration
	OpsPerSecond float64
	TotalBytes   int64
	Throughput   float64 // MB/s
}

func TestStoragePerformance(t *testing.T) {
	t.Log("=== Enhanced Storage Performance Benchmark ===")

	// Create temporary directory for benchmarks
	testDir, err := os.MkdirTemp("", "lodestone-benchmark")
	if err != nil {
		t.Fatal("Failed to create temp directory:", err)
	}
	defer os.RemoveAll(testDir)

	// Initialize enhanced storage
	storageDir := filepath.Join(testDir, "storage")
	storage, err := storage.NewLocalStorage(storageDir)
	if err != nil {
		t.Fatal("Failed to create storage:", err)
	}

	var results []BenchmarkResult

	// Test 1: Single-threaded write performance
	t.Log("1. Single-threaded write performance...")
	result1 := benchmarkSingleWrite(t, storage)
	results = append(results, result1)

	// Test 2: Concurrent write performance
	t.Log("2. Concurrent write performance...")
	result2 := benchmarkConcurrentWrite(t, storage)
	results = append(results, result2)

	// Test 3: Read performance
	t.Log("3. Read performance...")
	result3 := benchmarkRead(t, storage)
	results = append(results, result3)

	// Test 4: Mixed workload
	t.Log("4. Mixed workload performance...")
	result4 := benchmarkMixedWorkload(t, storage)
	results = append(results, result4)

	// Test 5: Large file performance
	t.Log("5. Large file performance...")
	result5 := benchmarkLargeFiles(t, storage)
	results = append(results, result5)

	// Print comprehensive results
	printResults(t, results)
	t.Log("✅ Performance benchmark completed!")
}

func benchmarkSingleWrite(t *testing.T, storage storage.BlobStorage) BenchmarkResult {
	ctx := context.Background()
	startTime := time.Now()
	var totalBytes int64
	count := 100

	for i := 0; i < count; i++ {
		content := fmt.Sprintf("Sequential write test data %d - %s", i, strings.Repeat("data", 10))
		path := fmt.Sprintf("bench_seq/file_%d.txt", i)

		err := storage.Store(ctx, path, strings.NewReader(content), "text/plain")
		if err != nil {
			t.Fatal("Failed to store file:", err)
		}
		totalBytes += int64(len(content))
	}

	totalTime := time.Since(startTime)
	return BenchmarkResult{
		Operation:    "Sequential Writes",
		TotalOps:     count,
		TotalTime:    totalTime,
		AvgTime:      totalTime / time.Duration(count),
		OpsPerSecond: float64(count) / totalTime.Seconds(),
		TotalBytes:   totalBytes,
		Throughput:   float64(totalBytes) / totalTime.Seconds() / 1024 / 1024,
	}
}

func benchmarkConcurrentWrite(t *testing.T, storage storage.BlobStorage) BenchmarkResult {
	ctx := context.Background()
	startTime := time.Now()
	var wg sync.WaitGroup
	var totalBytes int64
	var mu sync.Mutex

	goroutines := 10
	filesPerGoroutine := 50

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			var localBytes int64

			for i := 0; i < filesPerGoroutine; i++ {
				content := fmt.Sprintf("Concurrent write test data g%d_f%d - %s", gid, i, strings.Repeat("data", 15))
				path := fmt.Sprintf("bench_concurrent/g%d_file_%d.txt", gid, i)

				err := storage.Store(ctx, path, strings.NewReader(content), "text/plain")
				if err != nil {
					t.Logf("Failed to store file g%d_f%d: %v", gid, i, err)
					return
				}
				localBytes += int64(len(content))
			}

			mu.Lock()
			totalBytes += localBytes
			mu.Unlock()
		}(g)
	}

	wg.Wait()
	totalTime := time.Since(startTime)
	totalOps := goroutines * filesPerGoroutine

	return BenchmarkResult{
		Operation:    fmt.Sprintf("Concurrent Writes (%d goroutines)", goroutines),
		TotalOps:     totalOps,
		TotalTime:    totalTime,
		AvgTime:      totalTime / time.Duration(totalOps),
		OpsPerSecond: float64(totalOps) / totalTime.Seconds(),
		TotalBytes:   totalBytes,
		Throughput:   float64(totalBytes) / totalTime.Seconds() / 1024 / 1024,
	}
}

func benchmarkRead(t *testing.T, storage storage.BlobStorage) BenchmarkResult {
	ctx := context.Background()
	count := 100

	// First, create files to read
	for i := 0; i < count; i++ {
		content := fmt.Sprintf("Read test data %d - %s", i, strings.Repeat("test", 20))
		path := fmt.Sprintf("bench_read/file_%d.txt", i)
		err := storage.Store(ctx, path, strings.NewReader(content), "text/plain")
		if err != nil {
			t.Fatal("Failed to prepare read test file:", err)
		}
	}

	// Benchmark reads
	startTime := time.Now()
	var totalBytes int64

	for i := 0; i < count; i++ {
		path := fmt.Sprintf("bench_read/file_%d.txt", i)

		reader, err := storage.Retrieve(ctx, path)
		if err != nil {
			t.Fatal("Failed to retrieve file:", err)
		}

		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal("Failed to read file content:", err)
		}
		totalBytes += int64(len(data))
	}

	totalTime := time.Since(startTime)
	return BenchmarkResult{
		Operation:    "Sequential Reads",
		TotalOps:     count,
		TotalTime:    totalTime,
		AvgTime:      totalTime / time.Duration(count),
		OpsPerSecond: float64(count) / totalTime.Seconds(),
		TotalBytes:   totalBytes,
		Throughput:   float64(totalBytes) / totalTime.Seconds() / 1024 / 1024,
	}
}

func benchmarkMixedWorkload(t *testing.T, storage storage.BlobStorage) BenchmarkResult {
	ctx := context.Background()
	startTime := time.Now()
	var totalBytes int64
	totalOps := 200
	rand.Seed(time.Now().UnixNano())

	// Pre-populate some files for read/exists operations
	for i := 0; i < totalOps/4; i++ {
		content := fmt.Sprintf("Mixed workload pre-populated data %d", i)
		path := fmt.Sprintf("bench_mixed/existing_%d.txt", i)
		storage.Store(ctx, path, strings.NewReader(content), "text/plain")
	}

	for i := 0; i < totalOps; i++ {
		operation := rand.Intn(4) // 0=write, 1=read, 2=exists, 3=size

		switch operation {
		case 0: // Write
			content := fmt.Sprintf("Mixed workload write data %d - %s", i, strings.Repeat("data", 8))
			path := fmt.Sprintf("bench_mixed/new_%d.txt", i)
			err := storage.Store(ctx, path, strings.NewReader(content), "text/plain")
			if err == nil {
				totalBytes += int64(len(content))
			}

		case 1: // Read
			path := fmt.Sprintf("bench_mixed/existing_%d.txt", rand.Intn(totalOps/4))
			reader, err := storage.Retrieve(ctx, path)
			if err == nil {
				data, _ := io.ReadAll(reader)
				reader.Close()
				totalBytes += int64(len(data))
			}

		case 2: // Exists check
			path := fmt.Sprintf("bench_mixed/existing_%d.txt", rand.Intn(totalOps/4))
			storage.Exists(ctx, path)

		case 3: // Size check
			path := fmt.Sprintf("bench_mixed/existing_%d.txt", rand.Intn(totalOps/4))
			storage.GetSize(ctx, path)
		}
	}

	totalTime := time.Since(startTime)
	return BenchmarkResult{
		Operation:    "Mixed Workload (Write/Read/Exists/Size)",
		TotalOps:     totalOps,
		TotalTime:    totalTime,
		AvgTime:      totalTime / time.Duration(totalOps),
		OpsPerSecond: float64(totalOps) / totalTime.Seconds(),
		TotalBytes:   totalBytes,
		Throughput:   float64(totalBytes) / totalTime.Seconds() / 1024 / 1024,
	}
}

func benchmarkLargeFiles(t *testing.T, storage storage.BlobStorage) BenchmarkResult {
	ctx := context.Background()
	startTime := time.Now()
	var totalBytes int64
	count := 10

	// Create 1MB files
	largeContent := strings.Repeat("Large file test data with more content to simulate real package files. ", 12000) // ~1MB

	for i := 0; i < count; i++ {
		path := fmt.Sprintf("bench_large/file_%d.pkg", i)
		err := storage.Store(ctx, path, strings.NewReader(largeContent), "application/octet-stream")
		if err != nil {
			t.Fatal("Failed to store large file:", err)
		}
		totalBytes += int64(len(largeContent))
	}

	totalTime := time.Since(startTime)
	return BenchmarkResult{
		Operation:    "Large Files (~1MB each)",
		TotalOps:     count,
		TotalTime:    totalTime,
		AvgTime:      totalTime / time.Duration(count),
		OpsPerSecond: float64(count) / totalTime.Seconds(),
		TotalBytes:   totalBytes,
		Throughput:   float64(totalBytes) / totalTime.Seconds() / 1024 / 1024,
	}
}

func printResults(t *testing.T, results []BenchmarkResult) {
	t.Log("\n=== PERFORMANCE BENCHMARK RESULTS ===")
	t.Logf("%-40s %8s %12s %12s %15s %12s", "Operation", "Ops", "Total Time", "Avg Time", "Ops/Second", "Throughput")
	t.Logf("%-40s %8s %12s %12s %15s %12s", strings.Repeat("-", 40), strings.Repeat("-", 8), strings.Repeat("-", 12), strings.Repeat("-", 12), strings.Repeat("-", 15), strings.Repeat("-", 12))

	for _, result := range results {
		t.Logf("%-40s %8d %12s %12s %15.1f %8.2f MB/s",
			result.Operation,
			result.TotalOps,
			result.TotalTime.Round(time.Millisecond),
			result.AvgTime.Round(time.Microsecond),
			result.OpsPerSecond,
			result.Throughput,
		)
	}

	t.Log("\n=== SUMMARY ===")
	totalOps := 0
	totalTime := time.Duration(0)
	totalBytes := int64(0)

	for _, result := range results {
		totalOps += result.TotalOps
		totalTime += result.TotalTime
		totalBytes += result.TotalBytes
	}

	t.Logf("Total Operations: %d", totalOps)
	t.Logf("Total Time: %s", totalTime.Round(time.Millisecond))
	t.Logf("Total Data: %.2f MB", float64(totalBytes)/1024/1024)
	t.Logf("Overall Avg Ops/Second: %.1f", float64(totalOps)/totalTime.Seconds())
	t.Logf("Overall Avg Throughput: %.2f MB/s", float64(totalBytes)/totalTime.Seconds()/1024/1024)
}
