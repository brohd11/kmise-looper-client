// Package audio converts audio files to the pedal's native PCM (24-bit LE, stereo, 44.1 kHz).
package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/brohd11/kmise-looper-client/protocol"
)

// to24bit converts little-endian integer samples of the given byte width to 24-bit.
func to24bit(frames []byte, width int) ([]byte, error) {
	if width == 3 {
		return frames, nil
	}
	n := len(frames) / width
	out := make([]byte, n*3)
	switch width {
	case 2: // pad a zero LSB
		for i := 0; i < n; i++ {
			out[3*i+1] = frames[2*i]
			out[3*i+2] = frames[2*i+1]
		}
	case 4: // drop the LSB, as the vendor app does
		for i := 0; i < n; i++ {
			copy(out[3*i:3*i+3], frames[4*i+1:4*i+4])
		}
	default:
		return nil, fmt.Errorf("unsupported sample width %d-bit", width*8)
	}
	return out, nil
}

func monoToStereo(pcm24 []byte) []byte {
	out := make([]byte, len(pcm24)*2)
	for i := 0; i+3 <= len(pcm24); i += 3 {
		copy(out[2*i:], pcm24[i:i+3])
		copy(out[2*i+3:], pcm24[i:i+3])
	}
	return out
}

type wavFormat struct {
	format, channels uint16
	rate             uint32
	bits             uint16
}

// readWAV is the fast path for 44.1 kHz integer WAV. ok is false if ffmpeg is needed.
func readWAV(path string) (pcm []byte, ok bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, false, nil
	}
	var fmtc *wavFormat
	var frames []byte
	r := bytes.NewReader(data[12:])
	for {
		var hdr struct {
			ID   [4]byte
			Size uint32
		}
		if binary.Read(r, binary.LittleEndian, &hdr) != nil {
			break
		}
		body := make([]byte, hdr.Size)
		if _, err := io.ReadFull(r, body); err != nil {
			body = body[:len(body)-r.Len()] // tolerate a truncated final chunk
		}
		if hdr.Size%2 == 1 {
			r.ReadByte() // chunks are word aligned
		}
		switch string(hdr.ID[:]) {
		case "fmt ":
			if len(body) < 16 {
				return nil, false, nil
			}
			fmtc = &wavFormat{
				format:   binary.LittleEndian.Uint16(body[0:]),
				channels: binary.LittleEndian.Uint16(body[2:]),
				rate:     binary.LittleEndian.Uint32(body[4:]),
				bits:     binary.LittleEndian.Uint16(body[14:]),
			}
			// WAVE_FORMAT_EXTENSIBLE: the subformat GUID starts with the real format tag.
			if fmtc.format == 0xFFFE && len(body) >= 26 {
				fmtc.format = binary.LittleEndian.Uint16(body[24:])
			}
		case "data":
			frames = body
		}
	}
	// Integer PCM only; float and anything else go through ffmpeg.
	if fmtc == nil || frames == nil || fmtc.format != 1 || fmtc.rate != protocol.SampleRate ||
		(fmtc.channels != 1 && fmtc.channels != 2) {
		return nil, false, nil
	}
	width := int(fmtc.bits) / 8
	if width < 2 || width > 4 || int(fmtc.bits)%8 != 0 {
		return nil, false, nil
	}
	frames = frames[:len(frames)-len(frames)%(width*int(fmtc.channels))]
	pcm, err = to24bit(frames, width)
	if err != nil {
		return nil, false, err
	}
	if fmtc.channels == 1 {
		pcm = monoToStereo(pcm)
	}
	return pcm, true, nil
}

func readFFmpeg(path string) ([]byte, error) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("%s: only 44.1 kHz 16/24/32-bit WAV is supported without ffmpeg on PATH",
			filepath.Base(path))
	}
	cmd := exec.Command(ffmpeg, "-v", "error", "-i", path, "-f", "s24le", "-acodec", "pcm_s24le",
		"-ac", fmt.Sprint(protocol.Channels), "-ar", fmt.Sprint(protocol.SampleRate), "-")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg failed: %s", bytes.TrimSpace(stderr.Bytes()))
	}
	return out, nil
}

// LoadPCM reads an audio file and returns 24-bit stereo 44.1 kHz PCM.
func LoadPCM(path string) ([]byte, error) {
	pcm, ok, err := readWAV(path)
	if err != nil {
		return nil, err
	}
	if !ok {
		if pcm, err = readFFmpeg(path); err != nil {
			return nil, err
		}
	}
	pcm = pcm[:len(pcm)-len(pcm)%protocol.FrameBytes]
	if len(pcm) == 0 {
		return nil, errors.New(filepath.Base(path) + ": no audio")
	}
	return pcm, nil
}
