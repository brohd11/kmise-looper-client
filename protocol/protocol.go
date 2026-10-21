// Package protocol implements packet framing and payload parsing for the XS04 LOOP&DRUM
// (see docs/protocol.md). It does no I/O.
package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	VID        = 0xBB00
	PID        = 0x0001
	EPOut      = 0x02
	EPIn       = 0x81
	PacketSize = 512
	NumSlots   = 40

	MagicA        = 0x55AA6969
	MagicB        = 0x9696A5A5
	Trailer       = 0xAA559696
	TrailerOffset = 0x1FC
	PayloadOffset = 9
	MaxPayload    = TrailerOffset - PayloadOffset
)

const (
	CmdVersion     = 0
	CmdFileInfo    = 1
	CmdDelete      = 2
	CmdExport      = 3
	CmdExportDone  = 4
	CmdImport      = 5
	CmdImportDone  = 6
	CmdImportReady = 7
)

const (
	SampleRate     = 44100
	Channels       = 2
	SampleBytes    = 3 // 24-bit
	FrameBytes     = Channels * SampleBytes
	MaxImportBytes = 0x3C8FE80 // 240 s, the vendor app's limit
)

var le = binary.LittleEndian

// BuildPacket frames cmd and payload into a 512-byte command packet.
func BuildPacket(cmd byte, payload []byte) ([]byte, error) {
	if len(payload) > MaxPayload {
		return nil, fmt.Errorf("payload too long (%d > %d)", len(payload), MaxPayload)
	}
	pkt := make([]byte, PacketSize)
	le.PutUint32(pkt[0:], MagicA)
	le.PutUint32(pkt[4:], MagicB)
	pkt[8] = cmd
	copy(pkt[PayloadOffset:], payload)
	le.PutUint32(pkt[TrailerOffset:], Trailer)
	return pkt, nil
}

// ParsePacket returns the command and payload of a framed packet. ok is false for raw data.
func ParsePacket(data []byte) (cmd byte, payload []byte, ok bool) {
	if len(data) != PacketSize {
		return 0, nil, false
	}
	if le.Uint32(data[0:]) != MagicA || le.Uint32(data[4:]) != MagicB ||
		le.Uint32(data[TrailerOffset:]) != Trailer {
		return 0, nil, false
	}
	return data[8] & 0x7F, data[PayloadOffset:TrailerOffset], true
}

// Slot is one of the pedal's 40 recording slots.
type Slot struct {
	Index  int
	Frames uint32
}

func (s Slot) Empty() bool      { return s.Frames == 0 }
func (s Slot) Seconds() float64 { return float64(s.Frames) / SampleRate }
func (s Slot) PCMBytes() int64  { return int64(s.Frames) * FrameBytes }

// ParseFileInfo decodes `[count:u8]` then count x `[slot:u8][frames:u32 LE]`.
func ParseFileInfo(payload []byte) ([]Slot, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("empty file-info payload")
	}
	count := int(payload[0])
	if len(payload) < 1+5*count {
		return nil, fmt.Errorf("file-info payload too short for %d slots", count)
	}
	slots := make([]Slot, count)
	for i := range slots {
		off := 1 + 5*i
		slots[i] = Slot{Index: int(payload[off]), Frames: le.Uint32(payload[off+1:])}
	}
	return slots, nil
}

// ParseVersion returns the NUL-terminated version string.
func ParseVersion(payload []byte) string {
	if i := bytes.IndexByte(payload, 0); i >= 0 {
		payload = payload[:i]
	}
	return string(bytes.ToValidUTF8(payload, []byte("�")))
}

// WavHeader returns the 44-byte header matching the vendor app's template
// (PCM, 2ch, 44.1 kHz, 24-bit).
func WavHeader(pcmLen uint32) []byte {
	h := make([]byte, 44)
	copy(h[0:], "RIFF")
	le.PutUint32(h[4:], pcmLen+36)
	copy(h[8:], "WAVEfmt ")
	le.PutUint32(h[16:], 16)
	le.PutUint16(h[20:], 1)
	le.PutUint16(h[22:], Channels)
	le.PutUint32(h[24:], SampleRate)
	le.PutUint32(h[28:], SampleRate*FrameBytes)
	le.PutUint16(h[32:], FrameBytes)
	le.PutUint16(h[34:], SampleBytes*8)
	copy(h[36:], "data")
	le.PutUint32(h[40:], pcmLen)
	return h
}
