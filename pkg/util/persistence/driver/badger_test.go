package driver

import (
	"os"
	"testing"

	"github.com/loggie-io/loggie/pkg/core/log"
	"github.com/loggie-io/loggie/pkg/util/persistence/reg"
)

func TestMain(m *testing.M) {
	log.InitDefaultLogger()
	os.Exit(m.Run())
}

func newTestEngine(t *testing.T) reg.DbEngine {
	t.Helper()
	dir, err := os.MkdirTemp("", "badger-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	e := Init(dir)
	t.Cleanup(func() {
		e.Close()
	})
	return e
}

func makeRegistry(jobUid, source, pipeline, filename string, offset int64, lineNumber int64) reg.Registry {
	return reg.Registry{
		JobUid:       jobUid,
		SourceName:   source,
		PipelineName: pipeline,
		Filename:     filename,
		Offset:       offset,
		LineNumber:   lineNumber,
		CollectTime:  "2024-01-01 12:00:00.000",
		Version:      "test",
	}
}

func TestInsertAndFindAll(t *testing.T) {
	e := newTestEngine(t)

	regs := []reg.Registry{
		makeRegistry("1-1", "access", "default", "/var/log/a.log", 100, 10),
		makeRegistry("2-2", "error", "default", "/var/log/b.log", 200, 20),
	}

	if err := e.Insert(regs); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	all, err := e.FindAll()
	if err != nil {
		t.Fatalf("FindAll() error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("FindAll() returned %d records, want 2", len(all))
	}
}

func TestInsertDuplicateKey(t *testing.T) {
	e := newTestEngine(t)

	regs := []reg.Registry{
		makeRegistry("1-1", "access", "default", "/var/log/a.log", 100, 10),
	}
	if err := e.Insert(regs); err != nil {
		t.Fatalf("first Insert() error = %v", err)
	}

	regs2 := []reg.Registry{
		makeRegistry("1-1", "access", "default", "/var/log/a.log", 200, 20),
	}
	if err := e.Insert(regs2); err != nil {
		t.Fatalf("second Insert() error = %v", err)
	}

	all, err := e.FindAll()
	if err != nil {
		t.Fatalf("FindAll() error = %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("FindAll() returned %d records, want 1 (duplicate key should overwrite)", len(all))
	}
	if all[0].Offset != 200 {
		t.Errorf("Offset = %d, want 200", all[0].Offset)
	}
}

func TestFindBy(t *testing.T) {
	e := newTestEngine(t)

	regs := []reg.Registry{
		makeRegistry("1-1", "access", "default", "/var/log/a.log", 100, 10),
		makeRegistry("2-2", "error", "prod", "/var/log/b.log", 200, 20),
	}
	if err := e.Insert(regs); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	r, err := e.FindBy("1-1", "access", "default")
	if err != nil {
		t.Fatalf("FindBy() error = %v", err)
	}
	if r.Offset != 100 {
		t.Errorf("FindBy() Offset = %d, want 100", r.Offset)
	}
	if r.Filename != "/var/log/a.log" {
		t.Errorf("FindBy() Filename = %q, want /var/log/a.log", r.Filename)
	}

	// not found
	r2, err := e.FindBy("9-9", "none", "none")
	if err != nil {
		t.Fatalf("FindBy() not found error = %v", err)
	}
	if r2.JobUid != "" {
		t.Errorf("FindBy() not found should return empty Registry, got %+v", r2)
	}
}

func TestUpdateFullIntegrity(t *testing.T) {
	e := newTestEngine(t)

	regs := []reg.Registry{
		makeRegistry("1-1", "access", "default", "/var/log/a.log", 100, 10),
	}
	if err := e.Insert(regs); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// Update with full integrity (all fields set) - should overwrite directly
	updated := makeRegistry("1-1", "access", "default", "/var/log/a-new.log", 500, 50)
	updated.CollectTime = "2024-06-01 00:00:00.000"
	updated.Version = "v2"
	if err := e.Update([]reg.Registry{updated}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	r, err := e.FindBy("1-1", "access", "default")
	if err != nil {
		t.Fatalf("FindBy() error = %v", err)
	}
	if r.Offset != 500 {
		t.Errorf("Update() Offset = %d, want 500", r.Offset)
	}
	if r.Filename != "/var/log/a-new.log" {
		t.Errorf("Update() Filename = %q, want /var/log/a-new.log", r.Filename)
	}
}

func TestUpdatePartialMerge(t *testing.T) {
	e := newTestEngine(t)

	regs := []reg.Registry{
		makeRegistry("1-1", "access", "default", "/var/log/a.log", 100, 10),
	}
	if err := e.Insert(regs); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// Update with partial fields (no integrity) - should merge
	partial := reg.Registry{
		JobUid:       "1-1",
		SourceName:   "access",
		PipelineName: "default",
		Offset:       300,
	}
	if err := e.Update([]reg.Registry{partial}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	r, err := e.FindBy("1-1", "access", "default")
	if err != nil {
		t.Fatalf("FindBy() error = %v", err)
	}
	if r.Offset != 300 {
		t.Errorf("Update() merge Offset = %d, want 300", r.Offset)
	}
	// Filename should be preserved from original
	if r.Filename != "/var/log/a.log" {
		t.Errorf("Update() merge Filename = %q, want /var/log/a.log (should be preserved)", r.Filename)
	}
}

func TestUpdateFileName(t *testing.T) {
	e := newTestEngine(t)

	regs := []reg.Registry{
		makeRegistry("1-1", "access", "default", "/var/log/old.log", 100, 10),
	}
	if err := e.Insert(regs); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	err := e.UpdateFileName([]reg.Registry{
		makeRegistry("1-1", "access", "default", "/var/log/new.log", 0, 0),
	})
	if err != nil {
		t.Fatalf("UpdateFileName() error = %v", err)
	}

	r, err := e.FindBy("1-1", "access", "default")
	if err != nil {
		t.Fatalf("FindBy() error = %v", err)
	}
	if r.Filename != "/var/log/new.log" {
		t.Errorf("UpdateFileName() Filename = %q, want /var/log/new.log", r.Filename)
	}
}

func TestDelete(t *testing.T) {
	e := newTestEngine(t)

	regs := []reg.Registry{
		makeRegistry("1-1", "access", "default", "/var/log/a.log", 100, 10),
	}
	if err := e.Insert(regs); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	r, _ := e.FindBy("1-1", "access", "default")
	if err := e.Delete(r); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	r2, _ := e.FindBy("1-1", "access", "default")
	if r2.JobUid != "" {
		t.Errorf("after Delete(), FindBy() should return empty, got %+v", r2)
	}
}

func TestDeleteBy(t *testing.T) {
	e := newTestEngine(t)

	regs := []reg.Registry{
		makeRegistry("1-1", "access", "default", "/var/log/a.log", 100, 10),
		makeRegistry("2-2", "error", "prod", "/var/log/b.log", 200, 20),
	}
	if err := e.Insert(regs); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	if err := e.DeleteBy("1-1", "access", "default"); err != nil {
		t.Fatalf("DeleteBy() error = %v", err)
	}

	all, _ := e.FindAll()
	if len(all) != 1 {
		t.Fatalf("after DeleteBy(), FindAll() returned %d records, want 1", len(all))
	}
	if all[0].JobUid != "2-2" {
		t.Errorf("remaining record JobUid = %q, want 2-2", all[0].JobUid)
	}
}

func TestBatchInsert(t *testing.T) {
	e := newTestEngine(t)

	// Insert more than batch size (100)
	var regs []reg.Registry
	for i := 0; i < 250; i++ {
		regs = append(regs, makeRegistry(
			"job-"+string(rune('0'+i%10)),
			"source",
			"pipeline",
			"/var/log/app.log",
			int64(i*100),
			int64(i),
		))
	}

	if err := e.Insert(regs); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	all, err := e.FindAll()
	if err != nil {
		t.Fatalf("FindAll() error = %v", err)
	}
	// 10 unique keys (job-0 through job-9)
	if len(all) != 10 {
		t.Errorf("FindAll() returned %d records, want 10", len(all))
	}
}

func TestEmptyOperations(t *testing.T) {
	e := newTestEngine(t)

	// FindAll on empty db
	all, err := e.FindAll()
	if err != nil {
		t.Fatalf("FindAll() error = %v", err)
	}
	if len(all) != 0 {
		t.Errorf("FindAll() on empty db returned %d records, want 0", len(all))
	}

	// FindBy on empty db
	r, err := e.FindBy("1-1", "access", "default")
	if err != nil {
		t.Fatalf("FindBy() error = %v", err)
	}
	if r.JobUid != "" {
		t.Errorf("FindBy() on empty db should return empty, got %+v", r)
	}

	// Delete on non-existent key
	if err := e.Delete(makeRegistry("1-1", "access", "default", "", 0, 0)); err != nil {
		t.Fatalf("Delete() on non-existent key error = %v", err)
	}
}

func TestGroup(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		total   int
		wantLen int
	}{
		{
			name:    "smaller than batch",
			size:    100,
			total:   50,
			wantLen: 1,
		},
		{
			name:    "exact batch",
			size:    100,
			total:   100,
			wantLen: 1,
		},
		{
			name:    "two batches",
			size:    100,
			total:   150,
			wantLen: 2,
		},
		{
			name:    "empty",
			size:    100,
			total:   0,
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var regs []reg.Registry
			for i := 0; i < tt.total; i++ {
				regs = append(regs, makeRegistry("job", "src", "pipe", "file", int64(i), int64(i)))
			}
			groups := Group(regs, tt.size)
			if len(groups) != tt.wantLen {
				t.Errorf("Group() returned %d groups, want %d", len(groups), tt.wantLen)
			}

			// verify total count preserved
			total := 0
			for _, g := range groups {
				total += len(g)
			}
			if total != tt.total {
				t.Errorf("Group() total records = %d, want %d", total, tt.total)
			}
		})
	}
}

func BenchmarkInsert(b *testing.B) {
	dir, _ := os.MkdirTemp("", "badger-bench-*")
	defer os.RemoveAll(dir)
	e := Init(dir)
	defer e.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Insert([]reg.Registry{
			makeRegistry("bench", "src", "pipe", "file", int64(i), int64(i)),
		})
	}
}

func BenchmarkUpdate(b *testing.B) {
	dir, _ := os.MkdirTemp("", "badger-bench-*")
	defer os.RemoveAll(dir)
	e := Init(dir)
	defer e.Close()

	// seed data
	e.Insert([]reg.Registry{
		makeRegistry("bench", "src", "pipe", "file", 0, 0),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Update([]reg.Registry{
			makeRegistry("bench", "src", "pipe", "file", int64(i), int64(i)),
		})
	}
}

func BenchmarkFindAll100(b *testing.B)  { benchFindAll(b, 100) }
func BenchmarkFindAll1000(b *testing.B) { benchFindAll(b, 1000) }
func BenchmarkFindAll10000(b *testing.B) { benchFindAll(b, 10000) }

func benchFindAll(b *testing.B, count int) {
	dir, _ := os.MkdirTemp("", "badger-bench-*")
	defer os.RemoveAll(dir)
	e := Init(dir)
	defer e.Close()

	var regs []reg.Registry
	for i := 0; i < count; i++ {
		regs = append(regs, makeRegistry(
			"inode-"+string(rune(i/256+'0'))+string(rune(i%256+'0')),
			"access-log",
			"default",
			"/var/log/pod.log",
			int64(i*100), int64(i),
		))
	}
	e.Insert(regs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.FindAll()
	}
}

// Benchmark10kFiles simulates a K8s node with 10000 tailed log files.
// Tests the full lifecycle: insert all → update all → FindAll → FindBy → delete.
func Benchmark10kFiles(b *testing.B) {
	const fileCount = 10000

	dir, _ := os.MkdirTemp("", "badger-10k-*")
	defer os.RemoveAll(dir)
	e := Init(dir)
	defer e.Close()

	// Phase 1: Insert 10000 registries (simulating job startup)
	b.Run("Insert10k", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			b.StopTimer()
			var regs []reg.Registry
			for i := 0; i < fileCount; i++ {
				regs = append(regs, makeRegistry(
					"inode-"+string(rune(i/256+'0'))+string(rune(i%256+'0')),
					"access-log",
					"default",
					"/var/log/pod-"+string(rune(i/1000+'a'))+"/"+string(rune(i%1000+'a'))+".log",
					0, 0,
				))
			}
			b.StartTimer()
			e.Insert(regs)
		}
	})

	// Re-seed with 10000 records for subsequent benchmarks
	{
		var regs []reg.Registry
		for i := 0; i < fileCount; i++ {
			regs = append(regs, makeRegistry(
				"inode-"+string(rune(i/256+'0'))+string(rune(i%256+'0')),
				"access-log",
				"default",
				"/var/log/pod-"+string(rune(i/1000+'a'))+"/"+string(rune(i%1000+'a'))+".log",
				int64(i*100), int64(i),
			))
		}
		e.Insert(regs)
	}

	// Phase 2: Update all 10000 (simulating a 2-second flush cycle)
	b.Run("Update10k", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var regs []reg.Registry
			for i := 0; i < fileCount; i++ {
				regs = append(regs, makeRegistry(
					"inode-"+string(rune(i/256+'0'))+string(rune(i%256+'0')),
					"access-log",
					"default",
					"/var/log/pod-"+string(rune(i/1000+'a'))+"/"+string(rune(i%1000+'a'))+".log",
					int64(n*100+i*100), int64(n*1000+i),
				))
			}
			e.Update(regs)
		}
	})

	// Phase 3: FindAll (simulating cleanData hourly scan)
	b.Run("FindAll10k", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			e.FindAll()
		}
	})

	// Phase 4: FindBy (simulating upsertOffsetByJobWatchId)
	b.Run("FindBy10k", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			i := n % fileCount
			e.FindBy(
				"inode-"+string(rune(i/256+'0'))+string(rune(i%256+'0')),
				"access-log",
				"default",
			)
		}
	})

	// Phase 5: Simulate a flush with compressStats + index lookup
	// This is the actual hot path in production
	b.Run("FlushWithIndex", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			// Simulate 10000 states (one per file, each with one update)
			regs := make([]reg.Registry, 0, fileCount)
			for i := 0; i < fileCount; i++ {
				regs = append(regs, makeRegistry(
					"inode-"+string(rune(i/256+'0'))+string(rune(i%256+'0')),
					"access-log",
					"default",
					"/var/log/pod.log",
					int64(n*100+i*100), int64(n*1000+i),
				))
			}
			// In production, write() would do:
			// 1. compressStats (reduces to 1 per WatchUid)
			// 2. index lookup (O(1) per entry)
			// 3. batch Update
			e.Update(regs)
		}
	})
}
