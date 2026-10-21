package audio

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestSampleConversion(t *testing.T) {
	s16 := []byte{0xFE, 0xFF} // int16 -2
	if got, _ := to24bit(s16, 2); !bytes.Equal(got, []byte{0x00, 0xFE, 0xFF}) {
		t.Fatalf("16-bit: % x", got)
	}
	s32 := binary.LittleEndian.AppendUint32(nil, 0x12345678)
	if got, _ := to24bit(s32, 4); !bytes.Equal(got, []byte{0x56, 0x34, 0x12}) {
		t.Fatalf("32-bit: % x", got)
	}
	if got := monoToStereo([]byte{1, 2, 3}); !bytes.Equal(got, []byte{1, 2, 3, 1, 2, 3}) {
		t.Fatalf("mono: % x", got)
	}
}

// writeWAV writes a minimal PCM WAV; extra is appended to the fmt chunk.
func writeWAV(t *testing.T, format, channels, bits uint16, rate uint32, extra, data []byte) string {
	t.Helper()
	le := binary.LittleEndian
	fmtc := le.AppendUint16(nil, format)
	fmtc = le.AppendUint16(fmtc, channels)
	fmtc = le.AppendUint32(fmtc, rate)
	fmtc = le.AppendUint32(fmtc, rate*uint32(channels*bits/8))
	fmtc = le.AppendUint16(fmtc, channels*bits/8)
	fmtc = le.AppendUint16(fmtc, bits)
	fmtc = append(fmtc, extra...)

	body := []byte("WAVE")
	body = append(append(append(body, "fmt "...), le.AppendUint32(nil, uint32(len(fmtc)))...), fmtc...)
	body = append(append(append(body, "data"...), le.AppendUint32(nil, uint32(len(data)))...), data...)
	file := append(append([]byte("RIFF"), le.AppendUint32(nil, uint32(len(body)))...), body...)

	path := filepath.Join(t.TempDir(), "t.wav")
	if err := os.WriteFile(path, file, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadWAV16BitMono(t *testing.T) {
	path := writeWAV(t, 1, 1, 16, 44100, nil, []byte{1, 0, 0xFF, 0xFF}) // samples 1, -1
	pcm, err := LoadPCM(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 1, 0, 0, 1, 0, 0, 0xFF, 0xFF, 0, 0xFF, 0xFF}
	if !bytes.Equal(pcm, want) {
		t.Fatalf("got % x", pcm)
	}
}

func TestLoadWAVExtensible24Bit(t *testing.T) {
	extra := make([]byte, 24) // cbSize, validBits, channelMask, subformat GUID
	binary.LittleEndian.PutUint16(extra[0:], 22)
	binary.LittleEndian.PutUint16(extra[8:], 1) // KSDATAFORMAT_SUBTYPE_PCM
	data := []byte{1, 2, 3, 4, 5, 6}
	pcm, err := LoadPCM(writeWAV(t, 0xFFFE, 2, 24, 44100, extra, data))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pcm, data) {
		t.Fatalf("got % x", pcm)
	}
}

func TestOtherRateNeedsFFmpeg(t *testing.T) {
	path := writeWAV(t, 1, 2, 16, 48000, nil, make([]byte, 8))
	if _, ok, err := readWAV(path); ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}
