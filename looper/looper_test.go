package looper

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/brohd11/kmise-looper-client/protocol"
)

func TestParseSlots(t *testing.T) {
	got, err := ParseSlots([]string{"3", "10-12,7", "3"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{3, 7, 10, 11, 12}; !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for _, bad := range []string{"40", "-1", "x", "1-x"} {
		if _, err := ParseSlots([]string{bad}); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}

func TestFormatTime(t *testing.T) {
	for in, want := range map[float64]string{0: "0:00.00", 10: "0:10.00", 75.5: "1:15.50"} {
		if got := FormatTime(in); got != want {
			t.Errorf("FormatTime(%v) = %q, want %q", in, got, want)
		}
	}
}

type fakePedal struct {
	slots   []protocol.Slot
	pcm     map[int][]byte
	written map[int][]byte
}

func newFake() *fakePedal {
	f := &fakePedal{pcm: map[int][]byte{}, written: map[int][]byte{}}
	for i := 0; i < protocol.NumSlots; i++ {
		f.slots = append(f.slots, protocol.Slot{Index: i})
	}
	return f
}

func (f *fakePedal) Version() (string, error)            { return "fake", nil }
func (f *fakePedal) ListSlots() ([]protocol.Slot, error) { return slices.Clone(f.slots), nil }
func (f *fakePedal) Delete(slots []int) error            { return nil }

func (f *fakePedal) ReadSlot(ctx context.Context, slot int, progress Progress) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return f.pcm[slot], nil
}

func (f *fakePedal) WriteSlot(slot int, pcm []byte, progress Progress) error {
	f.written[slot] = pcm
	f.slots[slot].Frames = uint32(len(pcm) / protocol.FrameBytes)
	return nil
}

func TestExportSlotWritesWAV(t *testing.T) {
	f := newFake()
	f.pcm[2] = []byte{1, 2, 3, 4, 5, 6}
	path := filepath.Join(t.TempDir(), ExportName("slot", 2))
	n, err := ExportSlot(context.Background(), f, 2, path, nil)
	if err != nil || n != 6 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	got, _ := os.ReadFile(path)
	want := append(protocol.WavHeader(6), f.pcm[2]...)
	if !bytes.Equal(got, want) {
		t.Fatalf("file mismatch")
	}
}

func TestExportSlotCancelledLeavesNoFile(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dir := t.TempDir()
	if _, err := ExportSlot(ctx, newFake(), 0, filepath.Join(dir, "x.wav"), nil); err == nil {
		t.Fatal("expected error")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("left files behind: %v", entries)
	}
}

func TestImportRefusesOccupiedSlot(t *testing.T) {
	f := newFake()
	f.slots[5].Frames = 44100
	_, err := ImportPCM(f, 5, make([]byte, 6), false, nil)
	var occupied *SlotOccupiedError
	if !errors.As(err, &occupied) || len(f.written) != 0 {
		t.Fatalf("err=%v written=%d", err, len(f.written))
	}
	after, err := ImportPCM(f, 5, make([]byte, 12), true, nil)
	if err != nil || after.Frames != 2 {
		t.Fatalf("force: %+v %v", after, err)
	}
}
