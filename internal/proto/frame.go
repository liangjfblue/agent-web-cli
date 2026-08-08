// Package proto defines the wire protocol between the awc CLI and the local
// native host.
//
// Frame layout (CLI <-> host over Unix socket / named pipe):
//
//	+--------+--------+--------+--------+--------+-----------+--------+
//	| magic  | ver    | payload length (uint32 BE) | msgpack   | crc16  |
//	| 0x41 0x57 | u16  |        4 bytes            | N bytes   | 2 bytes|
//	+--------+--------+--------+--------+--------+-----------+--------+
//
// magic = "AW" (0x41, 0x57) — frame sync marker.
// crc16 covers the msgpack payload only, big-endian.
//
// This is a binary, length-prefixed, checksummed protocol. It deliberately
// differs from line-delimited JSON schemes to avoid being mistaken for any
// existing project's wire format.
package proto

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Magic identifies an awc frame on the wire.
var Magic = [2]byte{0x41, 0x57} // "AW"

// Version is the current protocol version.
const Version uint16 = 1

const (
	// headerLen = magic(2) + version(2) + length(4) = 8 bytes
	headerLen = 8
	crcLen    = 2
)

// ErrBadMagic is returned when a frame does not start with the awc magic.
var ErrBadMagic = errors.New("awc: bad frame magic")

// ErrShortRead is returned when the stream ended inside a frame.
var ErrShortRead = errors.New("awc: short read")

// ErrCRC is returned when the trailing CRC16 does not match the payload.
var ErrCRC = errors.New("awc: crc mismatch")

// ReadFrame reads a single frame from r and returns its msgpack payload.
func ReadFrame(r io.Reader) ([]byte, error) {
	hd := make([]byte, headerLen)
	if _, err := io.ReadFull(r, hd); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("awc: read header: %w", err)
	}
	if hd[0] != Magic[0] || hd[1] != Magic[1] {
		return nil, ErrBadMagic
	}
	ver := binary.BigEndian.Uint16(hd[2:4])
	if ver != Version {
		return nil, fmt.Errorf("awc: unsupported frame version %d", ver)
	}
	n := binary.BigEndian.Uint32(hd[4:8])
	if n == 0 {
		// Still must consume the CRC.
		tail := make([]byte, crcLen)
		if _, err := io.ReadFull(r, tail); err != nil {
			return nil, ErrShortRead
		}
		return nil, nil
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, ErrShortRead
	}
	tail := make([]byte, crcLen)
	if _, err := io.ReadFull(r, tail); err != nil {
		return nil, ErrShortRead
	}
	if want := crc16(payload); binary.BigEndian.Uint16(tail) != want {
		return nil, ErrCRC
	}
	return payload, nil
}

// WriteFrame encodes and writes a single frame to w.
func WriteFrame(w io.Writer, payload []byte) error {
	var buf bytes.Buffer
	buf.Write(Magic[:])
	var hd [6]byte // version(2) + length(4)
	binary.BigEndian.PutUint16(hd[0:2], Version)
	binary.BigEndian.PutUint32(hd[2:6], uint32(len(payload)))
	buf.Write(hd[:])
	buf.Write(payload)
	var cb [2]byte
	binary.BigEndian.PutUint16(cb[:], crc16(payload))
	buf.Write(cb[:])
	_, err := w.Write(buf.Bytes())
	return err
}
