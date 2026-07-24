package reg

import (
	"testing"
)

func TestGenKey(t *testing.T) {
	tests := []struct {
		name     string
		jobUid   string
		source   string
		pipeline string
	}{
		{
			name:     "basic",
			jobUid:   "123-456",
			source:   "access",
			pipeline: "default",
		},
		{
			name:     "empty strings",
			jobUid:   "",
			source:   "",
			pipeline: "",
		},
		{
			name:     "special characters",
			jobUid:   "123-456",
			source:   "my-source",
			pipeline: "my-pipeline",
		},
		{
			name:     "k8s names with slashes",
			jobUid:   "123-456",
			source:   "namespace/pod",
			pipeline: "app/deployment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenKey(tt.jobUid, tt.source, tt.pipeline)
			if got == nil {
				t.Error("GenKey returned nil")
			}
		})
	}
}

func TestGenKeyUniqueness(t *testing.T) {
	k1 := GenKey("a", "b", "c")
	k2 := GenKey("a", "b", "c")
	k3 := GenKey("a/b", "c", "d")
	k4 := GenKey("a", "b/c", "d")
	k5 := GenKey("a", "b", "c/d")

	if string(k1) != string(k2) {
		t.Error("same inputs should produce same key")
	}
	if string(k1) == string(k3) {
		t.Error("different inputs should produce different keys")
	}
	if string(k1) == string(k4) {
		t.Error("different inputs should produce different keys")
	}
	if string(k1) == string(k5) {
		t.Error("different inputs should produce different keys")
	}
}

func TestRegistryKey(t *testing.T) {
	r := &Registry{
		JobUid:       "inode-dev",
		SourceName:   "app",
		PipelineName: "prod",
	}
	got := r.Key()
	want := GenKey("inode-dev", "app", "prod")
	if string(got) != string(want) {
		t.Errorf("Key() = %v, want %v", got, want)
	}
}

func TestCheckIntegrity(t *testing.T) {
	tests := []struct {
		name string
		reg  Registry
		want bool
	}{
		{
			name: "valid",
			reg: Registry{
				Filename:    "/var/log/app.log",
				Offset:      1024,
				CollectTime: "2024-01-01 12:00:00",
				Version:     "1.0",
				LineNumber:  100,
			},
			want: true,
		},
		{
			name: "empty filename",
			reg: Registry{
				Filename:    "",
				Offset:      1024,
				CollectTime: "2024-01-01 12:00:00",
				Version:     "1.0",
				LineNumber:  100,
			},
			want: false,
		},
		{
			name: "zero offset",
			reg: Registry{
				Filename:    "/var/log/app.log",
				Offset:      0,
				CollectTime: "2024-01-01 12:00:00",
				Version:     "1.0",
				LineNumber:  100,
			},
			want: false,
		},
		{
			name: "empty collect time",
			reg: Registry{
				Filename:    "/var/log/app.log",
				Offset:      1024,
				CollectTime: "",
				Version:     "1.0",
				LineNumber:  100,
			},
			want: false,
		},
		{
			name: "empty version",
			reg: Registry{
				Filename:    "/var/log/app.log",
				Offset:      1024,
				CollectTime: "2024-01-01 12:00:00",
				Version:     "",
				LineNumber:  100,
			},
			want: false,
		},
		{
			name: "zero line number",
			reg: Registry{
				Filename:    "/var/log/app.log",
				Offset:      1024,
				CollectTime: "2024-01-01 12:00:00",
				Version:     "1.0",
				LineNumber:  0,
			},
			want: false,
		},
		{
			name: "all empty",
			reg:  Registry{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.reg.CheckIntegrity(); got != tt.want {
				t.Errorf("CheckIntegrity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name     string
		base     Registry
		other    Registry
		wantBase Registry
	}{
		{
			name: "merge all fields",
			base: Registry{
				Filename:    "old.log",
				Offset:      100,
				CollectTime: "2024-01-01 00:00:00",
				Version:     "1.0",
				LineNumber:  10,
			},
			other: Registry{
				Filename:    "new.log",
				Offset:      200,
				CollectTime: "2024-01-01 12:00:00",
				Version:     "2.0",
				LineNumber:  20,
			},
			wantBase: Registry{
				Filename:    "new.log",
				Offset:      200,
				CollectTime: "2024-01-01 12:00:00",
				Version:     "2.0",
				LineNumber:  20,
			},
		},
		{
			name: "merge partial - only offset",
			base: Registry{
				Filename:    "old.log",
				Offset:      100,
				CollectTime: "2024-01-01 00:00:00",
				Version:     "1.0",
				LineNumber:  10,
			},
			other: Registry{
				Offset:     200,
				LineNumber: 20,
			},
			wantBase: Registry{
				Filename:    "old.log",
				Offset:      200,
				CollectTime: "2024-01-01 00:00:00",
				Version:     "1.0",
				LineNumber:  20,
			},
		},
		{
			name: "merge empty other does not overwrite",
			base: Registry{
				Filename:    "old.log",
				Offset:      100,
				CollectTime: "2024-01-01 00:00:00",
				Version:     "1.0",
				LineNumber:  10,
			},
			other: Registry{},
			wantBase: Registry{
				Filename:    "old.log",
				Offset:      100,
				CollectTime: "2024-01-01 00:00:00",
				Version:     "1.0",
				LineNumber:  10,
			},
		},
		{
			name: "merge zero offset does not overwrite",
			base: Registry{
				Offset:     100,
				LineNumber: 10,
			},
			other: Registry{
				Offset:     0,
				LineNumber: 0,
			},
			wantBase: Registry{
				Offset:     100,
				LineNumber: 10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(tt.other)
			if tt.base != tt.wantBase {
				t.Errorf("Merge() = %+v, want %+v", tt.base, tt.wantBase)
			}
		})
	}
}

func TestBinaryEncodeDecode(t *testing.T) {
	r := Registry{
		Id:           1,
		PipelineName: "default",
		SourceName:   "access",
		Filename:     "/var/log/app.log",
		JobUid:       "123-456",
		Offset:       1024,
		CollectTime:  "2024-01-01 12:00:00.000",
		Version:      "1.0",
		LineNumber:   100,
	}

	data := r.Encode()
	if len(data) == 0 {
		t.Fatal("Encode() returned empty bytes")
	}

	decoded, err := DecodeRegistry(data)
	if err != nil {
		t.Fatalf("DecodeRegistry() error = %v", err)
	}

	if decoded.PipelineName != r.PipelineName {
		t.Errorf("PipelineName = %q, want %q", decoded.PipelineName, r.PipelineName)
	}
	if decoded.SourceName != r.SourceName {
		t.Errorf("SourceName = %q, want %q", decoded.SourceName, r.SourceName)
	}
	if decoded.Filename != r.Filename {
		t.Errorf("Filename = %q, want %q", decoded.Filename, r.Filename)
	}
	if decoded.JobUid != r.JobUid {
		t.Errorf("JobUid = %q, want %q", decoded.JobUid, r.JobUid)
	}
	if decoded.Offset != r.Offset {
		t.Errorf("Offset = %d, want %d", decoded.Offset, r.Offset)
	}
	if decoded.LineNumber != r.LineNumber {
		t.Errorf("LineNumber = %d, want %d", decoded.LineNumber, r.LineNumber)
	}
	if decoded.CollectTime != r.CollectTime {
		t.Errorf("CollectTime = %q, want %q", decoded.CollectTime, r.CollectTime)
	}
	if decoded.Version != r.Version {
		t.Errorf("Version = %q, want %q", decoded.Version, r.Version)
	}
}

func TestBinarySize(t *testing.T) {
	r := Registry{
		Id:           1,
		PipelineName: "default",
		SourceName:   "access-log",
		Filename:     "/var/log/app.log",
		JobUid:       "12345-67890",
		Offset:       1024000,
		CollectTime:  "2024-01-01 12:00:00.000",
		Version:      "v1.0",
		LineNumber:   5000,
	}
	binData := r.Encode()
	t.Logf("binary size: %d bytes", len(binData))

	// Old JSON format was ~200 bytes for this record
	// Binary should be ~104 bytes (21 byte header + strings)
	if len(binData) > 150 {
		t.Errorf("binary size %d should be well under 150 bytes", len(binData))
	}
}

func TestBinaryEmptyStrings(t *testing.T) {
	r := Registry{
		Offset:     100,
		LineNumber: 10,
	}
	data := r.Encode()
	decoded, err := DecodeRegistry(data)
	if err != nil {
		t.Fatalf("DecodeRegistry() error = %v", err)
	}
	if decoded.Filename != "" {
		t.Errorf("Filename = %q, want empty", decoded.Filename)
	}
}

func TestBinaryDecodeError(t *testing.T) {
	_, err := DecodeRegistry(nil)
	if err == nil {
		t.Error("DecodeRegistry(nil) should return error")
	}
	_, err = DecodeRegistry([]byte{})
	if err == nil {
		t.Error("DecodeRegistry(empty) should return error")
	}
	_, err = DecodeRegistry([]byte{0xFF})
	if err == nil {
		t.Error("DecodeRegistry(bad version) should return error")
	}
}

func BenchmarkGenKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenKey("inode-12345", "access-log", "default-pipeline")
	}
}

func BenchmarkValue(b *testing.B) {
	r := Registry{
		PipelineName: "default",
		SourceName:   "access",
		Filename:     "/var/log/app.log",
		JobUid:       "123-456",
		Offset:       1024,
		CollectTime:  "2024-01-01 12:00:00.000",
		Version:      "1.0",
		LineNumber:   100,
	}
	for i := 0; i < b.N; i++ {
		r.Value()
	}
}

func BenchmarkDecodeRegistry(b *testing.B) {
	r := Registry{
		PipelineName: "default",
		SourceName:   "access",
		Filename:     "/var/log/app.log",
		JobUid:       "123-456",
		Offset:       1024,
		CollectTime:  "2024-01-01 12:00:00.000",
		Version:      "1.0",
		LineNumber:   100,
	}
	data := r.Encode()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DecodeRegistry(data)
	}
}
