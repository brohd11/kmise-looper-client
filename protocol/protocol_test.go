package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// Real file-info payload captured from the device (all 40 slots empty), first 0xD0 bytes.
func emptyFileInfo() []byte {
	b := []byte{0x28}
	for i := 0; i < 40; i++ {
		b = append(b, byte(i), 0, 0, 0, 0)
	}
	return b
}

func TestPacketRoundtrip(t *testing.T) {
	pkt, err := BuildPacket(CmdImport, []byte{5, 0x10, 0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(pkt) != 512 {
		t.Fatalf("len %d", len(pkt))
	}
	if want, _ := hex.DecodeString("6969aa55a5a5969605"); !bytes.Equal(pkt[:9], want) {
		t.Fatalf("header % x", pkt[:9])
	}
	if want, _ := hex.DecodeString("969655aa"); !bytes.Equal(pkt[0x1FC:], want) {
		t.Fatalf("trailer % x", pkt[0x1FC:])
	}
	cmd, payload, ok := ParsePacket(pkt)
	if !ok || cmd != CmdImport || !bytes.Equal(payload[:5], []byte{5, 0x10, 0, 0, 0}) {
		t.Fatalf("parse: %v %d % x", ok, cmd, payload[:5])
	}
}

func TestResponseBit7Masked(t *testing.T) {
	pkt, _ := BuildPacket(CmdFileInfo, nil)
	pkt[8] |= 0x80
	if cmd, _, ok := ParsePacket(pkt); !ok || cmd != CmdFileInfo {
		t.Fatalf("got %d %v", cmd, ok)
	}
}

func TestRawDataIsNotAPacket(t *testing.T) {
	if _, _, ok := ParsePacket(make([]byte, 512)); ok {
		t.Fatal("zeros parsed as packet")
	}
	if _, _, ok := ParsePacket(make([]byte, 100)); ok {
		t.Fatal("short buffer parsed as packet")
	}
}

func TestPayloadTooLong(t *testing.T) {
	if _, err := BuildPacket(CmdVersion, make([]byte, MaxPayload+1)); err == nil {
		t.Fatal("expected error")
	}
}

func TestFileInfoEmptyDevice(t *testing.T) {
	slots, err := ParseFileInfo(emptyFileInfo())
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 40 {
		t.Fatalf("got %d slots", len(slots))
	}
	for i, s := range slots {
		if s.Index != i || !s.Empty() {
			t.Fatalf("slot %d: %+v", i, s)
		}
	}
}

func TestFileInfoLengths(t *testing.T) {
	p := emptyFileInfo()
	off := 1 + 5*3
	p[off] = 3
	le.PutUint32(p[off+1:], 441000)
	slots, _ := ParseFileInfo(p)
	s := slots[3]
	if s.Index != 3 || s.Seconds() != 10.0 || s.PCMBytes() != 441000*6 {
		t.Fatalf("%+v", s)
	}
}

func TestWavHeaderMatchesVendorTemplate(t *testing.T) {
	// Template bytes taken verbatim from the vendor exe @ 0x428020
	vendor, _ := hex.DecodeString("52494646cc2c0a0057415645666d74201000000001000200" +
		"44ac0000980904000600180064617461a82c0a00")
	if got := WavHeader(0x0A2CA8); !bytes.Equal(got, vendor) {
		t.Fatalf("got  % x\nwant % x", got, vendor)
	}
}

func TestParseVersion(t *testing.T) {
	if v := ParseVersion([]byte("V1.0\x00junk")); v != "V1.0" {
		t.Fatalf("got %q", v)
	}
}
