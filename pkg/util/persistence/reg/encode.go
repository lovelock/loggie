package reg

import (
	"encoding/binary"
	"errors"
	"io"
)

// Binary encoding format for Registry (version 1)
//
// Layout:
//   +--------+--------+--------+--------+-------------------+
//   | version|   id   | offset |lineNum | strings...        |
//   | 1 byte | 4 byte | 8 byte | 8 byte | (variable length) |
//   +--------+--------+--------+--------+-------------------+
//
// Strings section (each field):
//   +------------+-----------+
//   | len (uint16) | data []byte |
//   +------------+-----------+
//
// Field order in strings section:
//   1. filename
//   2. collectTime  (format: "2006-01-02 15:04:05.999", 23 bytes)
//   3. version      (e.g. "v1.2.3", ~6 bytes)
//   4. pipelineName (e.g. "default", ~7 bytes)
//   5. sourceName   (e.g. "access-log", ~10 bytes)
//   6. jobUid       (e.g. "12345-67890", ~11 bytes)
//
// Example sizes:
//   - 100 files × ~100 bytes/record = ~10 KB total
//   - Old JSON encoding: ~200 bytes/record → 20 KB for same 100 files
//
// Version history:
//   v1: initial binary format (2024-07)

const binaryVersion byte = 1

func (r *Registry) Encode() []byte {
	fLen := len(r.Filename)
	cLen := len(r.CollectTime)
	vLen := len(r.Version)
	pLen := len(r.PipelineName)
	sLen := len(r.SourceName)
	jLen := len(r.JobUid)

	size := 1 + 4 + 8 + 8 + // header: version(1) + id(4) + offset(8) + lineNumber(8)
		2+fLen + 2+cLen + 2+vLen + 2+pLen + 2+sLen + 2+jLen // strings

	buf := make([]byte, size)
	off := 0

	buf[0] = binaryVersion
	off++

	binary.LittleEndian.PutUint32(buf[off:], uint32(r.Id))
	off += 4

	binary.LittleEndian.PutUint64(buf[off:], uint64(r.Offset))
	off += 8

	binary.LittleEndian.PutUint64(buf[off:], uint64(r.LineNumber))
	off += 8

	off = putStr(buf, off, r.Filename)
	off = putStr(buf, off, r.CollectTime)
	off = putStr(buf, off, r.Version)
	off = putStr(buf, off, r.PipelineName)
	off = putStr(buf, off, r.SourceName)
	off = putStr(buf, off, r.JobUid)

	return buf
}

func DecodeRegistry(data []byte) (Registry, error) {
	if len(data) < 1 {
		return Registry{}, io.ErrUnexpectedEOF
	}

	ver := data[0]
	if ver != binaryVersion {
		return Registry{}, errors.New("unsupported binary version")
	}

	r := Registry{}
	off := 1

	if len(data) < off+4 {
		return Registry{}, io.ErrUnexpectedEOF
	}
	r.Id = int(binary.LittleEndian.Uint32(data[off:]))
	off += 4

	if len(data) < off+8 {
		return Registry{}, io.ErrUnexpectedEOF
	}
	r.Offset = int64(binary.LittleEndian.Uint64(data[off:]))
	off += 8

	if len(data) < off+8 {
		return Registry{}, io.ErrUnexpectedEOF
	}
	r.LineNumber = int64(binary.LittleEndian.Uint64(data[off:]))
	off += 8

	var err error
	r.Filename, off, err = getStr(data, off)
	if err != nil {
		return Registry{}, err
	}
	r.CollectTime, off, err = getStr(data, off)
	if err != nil {
		return Registry{}, err
	}
	r.Version, off, err = getStr(data, off)
	if err != nil {
		return Registry{}, err
	}
	r.PipelineName, off, err = getStr(data, off)
	if err != nil {
		return Registry{}, err
	}
	r.SourceName, off, err = getStr(data, off)
	if err != nil {
		return Registry{}, err
	}
	r.JobUid, off, err = getStr(data, off)
	if err != nil {
		return Registry{}, err
	}

	return r, nil
}

func putStr(buf []byte, off int, s string) int {
	binary.LittleEndian.PutUint16(buf[off:], uint16(len(s)))
	off += 2
	copy(buf[off:], s)
	return off + len(s)
}

func getStr(data []byte, off int) (string, int, error) {
	if len(data) < off+2 {
		return "", 0, io.ErrUnexpectedEOF
	}
	n := int(binary.LittleEndian.Uint16(data[off:]))
	off += 2
	if len(data) < off+n {
		return "", 0, io.ErrUnexpectedEOF
	}
	return string(data[off : off+n]), off + n, nil
}
