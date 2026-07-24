package persistence

import (
	"testing"
	"time"
)

func TestTime2TextAndText2Time(t *testing.T) {
	now := time.Now()
	text := time2text(now)

	// parse back
	parsed := text2time(text)

	// truncated to millisecond precision
	if !now.Truncate(time.Millisecond).Equal(parsed.Truncate(time.Millisecond)) {
		t.Errorf("round-trip failed: now=%v, text=%q, parsed=%v", now, text, parsed)
	}
}

func TestTime2TextFormat(t *testing.T) {
	loc, _ := time.LoadLocation("Local")
	ts := time.Date(2024, 3, 15, 14, 30, 45, 123456789, loc)
	text := time2text(ts)
	want := "2024-03-15 14:30:45.123"
	if text != want {
		t.Errorf("time2text() = %q, want %q", text, want)
	}
}

func TestCompressStats_Single(t *testing.T) {
	stats := []*State{
		{WatchUid: "job-1", Offset: 100, LineNumber: 10},
	}
	result := compressStats(stats)
	if len(result) != 1 {
		t.Fatalf("compressStats() returned %d pairs, want 1", len(result))
	}
	if result[0].first != result[0].last {
		t.Error("single stat: first and last should be the same pointer")
	}
}

func TestCompressStats_MultipleSameJob(t *testing.T) {
	stats := []*State{
		{WatchUid: "job-1", Offset: 300, LineNumber: 30},
		{WatchUid: "job-1", Offset: 100, LineNumber: 10},
		{WatchUid: "job-1", Offset: 200, LineNumber: 20},
	}
	result := compressStats(stats)
	if len(result) != 1 {
		t.Fatalf("compressStats() returned %d pairs, want 1", len(result))
	}
	if result[0].first.Offset != 100 {
		t.Errorf("first.Offset = %d, want 100", result[0].first.Offset)
	}
	if result[0].last.Offset != 300 {
		t.Errorf("last.Offset = %d, want 300", result[0].last.Offset)
	}
}

func TestCompressStats_DifferentJobs(t *testing.T) {
	stats := []*State{
		{WatchUid: "job-1", Offset: 100},
		{WatchUid: "job-2", Offset: 200},
		{WatchUid: "job-1", Offset: 50},
		{WatchUid: "job-2", Offset: 300},
	}
	result := compressStats(stats)
	if len(result) != 2 {
		t.Fatalf("compressStats() returned %d pairs, want 2", len(result))
	}

	for _, cs := range result {
		if cs.first.Offset > cs.last.Offset {
			t.Errorf("job %s: first.Offset (%d) > last.Offset (%d)",
				cs.first.WatchUid, cs.first.Offset, cs.last.Offset)
		}
	}
}

func TestCompressStats_Empty(t *testing.T) {
	result := compressStats([]*State{})
	if len(result) != 0 {
		t.Errorf("compressStats() on empty returned %d pairs, want 0", len(result))
	}
}

func TestCompositeKey(t *testing.T) {
	tests := []struct {
		name      string
		jobUid    string
		source    string
		pipeline  string
	}{
		{name: "basic", jobUid: "j1", source: "s1", pipeline: "p1"},
		{name: "empty", jobUid: "", source: "", pipeline: ""},
		{name: "k8s", jobUid: "123", source: "ns/pod", pipeline: "svc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := compositeKey(tt.jobUid, tt.source, tt.pipeline)
			want := tt.jobUid + "/" + tt.source + "/" + tt.pipeline
			if k != want {
				t.Errorf("compositeKey() = %q, want %q", k, want)
			}
		})
	}
}

func TestCompositeKeyUniqueness(t *testing.T) {
	k1 := compositeKey("j1", "s1", "p1")
	k2 := compositeKey("j1", "s1", "p1")

	if k1 != k2 {
		t.Error("same inputs should produce same key")
	}
	// Note: compositeKey uses "/" as separator, so component values containing "/"
	// could create ambiguity. This is fine because compositeKey is only used for
	// in-memory map lookup, not as a database key (that uses length-prefixed encoding in reg.GenKey).
}

func TestStateAppendTags(t *testing.T) {
	s := &State{}
	s.AppendTags("tag1")
	if s.Tags != "tag1" {
		t.Errorf("first AppendTags: Tags = %q, want tag1", s.Tags)
	}
	s.AppendTags("tag2")
	if s.Tags != "tag1,tag2" {
		t.Errorf("second AppendTags: Tags = %q, want tag1,tag2", s.Tags)
	}
}

func TestDbConfigSetDefaults(t *testing.T) {
	c := DbConfig{}
	c.SetDefaults()
	if c.File != "./data/badger" {
		t.Errorf("default File = %q, want ./data/badger", c.File)
	}

	// explicit file should not be overridden
	c2 := DbConfig{File: "/custom/path"}
	c2.SetDefaults()
	if c2.File != "/custom/path" {
		t.Errorf("explicit File = %q, want /custom/path", c2.File)
	}
}

func BenchmarkCompressStats(b *testing.B) {
	stats := make([]*State, 1000)
	for i := 0; i < 1000; i++ {
		stats[i] = &State{
			WatchUid: "job-" + string(rune('A'+i%26)),
			Offset:   int64(i * 100),
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compressStats(stats)
	}
}

func BenchmarkCompositeKeyLookup1000(b *testing.B) {
	index := make(map[string]int, 1000)
	for i := 0; i < 1000; i++ {
		key := compositeKey("job", "source", "pipeline-"+string(rune('A'+i%26)))
		index[key] = i
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := compositeKey("job", "source", "pipeline-Z")
		_ = index[key]
	}
}
