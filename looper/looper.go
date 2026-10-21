// Package looper holds the high-level operations shared by the CLI and any future TUI/GUI:
// the same validation and safety checks apply whichever front end calls them.
package looper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/brohd11/kmise-looper-client/audio"
	"github.com/brohd11/kmise-looper-client/protocol"
)

// Progress reports bytes transferred so far out of total.
type Progress = func(done, total int)

// Pedal is the device surface the operations need. *device.Device implements it; tests and
// UI previews can supply a fake.
type Pedal interface {
	Version() (string, error)
	ListSlots() ([]protocol.Slot, error)
	ReadSlot(ctx context.Context, slot int, progress Progress) ([]byte, error)
	WriteSlot(slot int, pcm []byte, progress Progress) error
	Delete(slots []int) error
}

// FormatTime renders seconds as m:ss.ss.
func FormatTime(seconds float64) string {
	m := int(seconds / 60)
	return fmt.Sprintf("%d:%05.2f", m, seconds-float64(m*60))
}

// ParseSlots accepts "3", "3,7", "10-15" (any mix, several values). Slots are 0-based, as on
// the device. The result is sorted and deduplicated.
func ParseSlots(values []string) ([]int, error) {
	seen := map[int]bool{}
	var bad []int
	add := func(s int) {
		if s < 0 || s >= protocol.NumSlots {
			bad = append(bad, s)
		}
		seen[s] = true
	}
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			if part == "" {
				continue
			}
			if a, b, ok := strings.Cut(part, "-"); ok && a != "" {
				lo, err1 := strconv.Atoi(a)
				hi, err2 := strconv.Atoi(b)
				if err1 != nil || err2 != nil {
					return nil, fmt.Errorf("invalid slot range %q", part)
				}
				for s := lo; s <= hi; s++ {
					add(s)
				}
				continue
			}
			s, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid slot %q", part)
			}
			add(s)
		}
	}
	if len(bad) > 0 {
		return nil, fmt.Errorf("slots must be 0-%d: %v", protocol.NumSlots-1, bad)
	}
	out := make([]int, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Ints(out)
	return out, nil
}

// Info returns the firmware version and every slot.
func Info(d Pedal) (string, []protocol.Slot, error) {
	slots, err := d.ListSlots()
	if err != nil {
		return "", nil, err
	}
	version, err := d.Version()
	return version, slots, err
}

// SlotMap indexes a slot listing by slot number.
func SlotMap(slots []protocol.Slot) map[int]protocol.Slot {
	m := make(map[int]protocol.Slot, len(slots))
	for _, s := range slots {
		m[s.Index] = s
	}
	return m
}

// ExportName is the default file name for an exported slot.
func ExportName(prefix string, slot int) string {
	return fmt.Sprintf("%s_%02d.wav", prefix, slot)
}

// ExportSlot writes one slot to path as 24-bit WAV, bit-exact with what the pedal stores.
// The file appears only once the transfer completes; nothing is left behind on error or
// cancellation.
func ExportSlot(ctx context.Context, d Pedal, slot int, path string, progress Progress) (int, error) {
	pcm, err := d.ReadSlot(ctx, slot, progress)
	if err != nil {
		return 0, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".kmise-*.wav")
	if err != nil {
		return 0, err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(protocol.WavHeader(uint32(len(pcm)))); err == nil {
		_, err = tmp.Write(pcm)
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		return 0, err
	}
	return len(pcm), nil
}

// LoadForImport reads and converts an audio file, enforcing the pedal's 240 s limit. Call it
// before opening the device so bad input never touches the pedal.
func LoadForImport(path string) ([]byte, error) {
	pcm, err := audio.LoadPCM(path)
	if err != nil {
		return nil, err
	}
	if len(pcm) > protocol.MaxImportBytes {
		return nil, fmt.Errorf("audio is longer than the 240 s limit")
	}
	return pcm, nil
}

// SlotOccupiedError is returned by ImportPCM when the slot holds audio and force is false.
type SlotOccupiedError struct{ Slot protocol.Slot }

func (e *SlotOccupiedError) Error() string {
	return fmt.Sprintf("slot %d already holds %s", e.Slot.Index, FormatTime(e.Slot.Seconds()))
}

// ImportPCM writes pcm into slot and returns the slot as the device reports it afterwards.
// It refuses a non-empty slot unless force is set. It cannot be cancelled mid-transfer:
// that would leave the pedal in an unknown state.
func ImportPCM(d Pedal, slot int, pcm []byte, force bool, progress Progress) (protocol.Slot, error) {
	if slot < 0 || slot >= protocol.NumSlots {
		return protocol.Slot{}, fmt.Errorf("slots must be 0-%d: [%d]", protocol.NumSlots-1, slot)
	}
	slots, err := d.ListSlots()
	if err != nil {
		return protocol.Slot{}, err
	}
	if cur := SlotMap(slots)[slot]; !cur.Empty() && !force {
		return protocol.Slot{}, &SlotOccupiedError{cur}
	}
	if err := d.WriteSlot(slot, pcm, progress); err != nil {
		return protocol.Slot{}, err
	}
	if slots, err = d.ListSlots(); err != nil {
		return protocol.Slot{}, err
	}
	return SlotMap(slots)[slot], nil
}

// Delete permanently removes slots. Front ends must confirm with the user first.
func Delete(d Pedal, slots []int) error {
	return d.Delete(slots)
}
