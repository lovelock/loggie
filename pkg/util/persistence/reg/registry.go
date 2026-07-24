package reg

import (
	"encoding/binary"
)

type Registry struct {
	Id           int    `json:"id"`
	PipelineName string `json:"pipelineName"`
	SourceName   string `json:"sourceName"`
	Filename     string `json:"filename"`
	JobUid       string `json:"jobUid"`
	Offset       int64  `json:"offset"`
	CollectTime  string `json:"collectTime"`
	Version      string `json:"version"`
	LineNumber   int64  `json:"lineNumber"`
}

type RegistryList []Registry

func (r *Registry) Key() []byte {
	return GenKey(r.JobUid, r.SourceName, r.PipelineName)
}

// Key format: length-prefixed encoding, collision-free
//
//   +------+--------+------+--------+------+----------+
//   | len  | jobUid | len  | source | len  | pipeline |
//   |uint16| []byte |uint16| []byte |uint16| []byte   |
//   +------+--------+------+--------+------+----------+
//
// Length prefix is uint16 little-endian (max 65535 bytes per component).
// This avoids key ambiguity when components contain '/' or other delimiters.
//
// Old format used '/' separator: "jobUid/source/pipeline"
// Problem: K8s names often contain '/', causing collisions:
//   key("ns/pod", "svc", "pipe") == key("ns", "pod/svc", "pipe")
// New format: length prefix eliminates this.
func GenKey(jobUid, source, pipeline string) []byte {
	jb := []byte(jobUid)
	sb := []byte(source)
	pb := []byte(pipeline)
	buf := make([]byte, 2+len(jb)+2+len(sb)+2+len(pb))
	off := 0
	binary.LittleEndian.PutUint16(buf[off:], uint16(len(jb)))
	off += 2
	copy(buf[off:], jb)
	off += len(jb)
	binary.LittleEndian.PutUint16(buf[off:], uint16(len(sb)))
	off += 2
	copy(buf[off:], sb)
	off += len(sb)
	binary.LittleEndian.PutUint16(buf[off:], uint16(len(pb)))
	off += 2
	copy(buf[off:], pb)
	return buf
}

func (r *Registry) Value() []byte {
	return r.Encode()
}

func (r *Registry) CheckIntegrity() bool {
	return len(r.Filename) > 0 &&
		r.Offset > 0 &&
		len(r.CollectTime) > 0 &&
		len(r.Version) > 0 &&
		r.LineNumber > 0
}

func (r *Registry) Merge(registry Registry) {
	if len(registry.Filename) > 0 {
		r.Filename = registry.Filename
	}

	if registry.Offset > 0 {
		r.Offset = registry.Offset
	}

	if len(registry.CollectTime) > 0 {
		r.CollectTime = registry.CollectTime
	}

	if len(registry.Version) > 0 {
		r.Version = registry.Version
	}

	if registry.LineNumber > 0 {
		r.LineNumber = registry.LineNumber
	}
}
