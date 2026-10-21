import struct
import wave

from kmise import protocol as p
from kmise.audio import _mono_to_stereo, _to_24bit, load_pcm

# Real file-info payload captured from the device (all 40 slots empty), first 0xD0 bytes.
EMPTY_FILE_INFO = bytes([0x28]) + b"".join(bytes([i, 0, 0, 0, 0]) for i in range(40))


def test_packet_roundtrip():
    pkt = p.build_packet(p.CMD_IMPORT, b"\x05\x10\x00\x00\x00")
    assert len(pkt) == 512
    assert pkt[:9] == bytes.fromhex("6969aa55a5a59696") + b"\x05"
    assert pkt[0x1FC:] == bytes.fromhex("969655aa")
    cmd, payload = p.parse_packet(pkt)
    assert cmd == p.CMD_IMPORT and payload[:5] == b"\x05\x10\x00\x00\x00"


def test_response_bit7_masked():
    pkt = bytearray(p.build_packet(p.CMD_FILE_INFO))
    pkt[8] |= 0x80
    assert p.parse_packet(bytes(pkt))[0] == p.CMD_FILE_INFO


def test_raw_data_is_not_a_packet():
    assert p.parse_packet(bytes(512)) is None
    assert p.parse_packet(b"\x00" * 100) is None


def test_file_info_empty_device():
    slots = p.parse_file_info(EMPTY_FILE_INFO)
    assert [s.index for s in slots] == list(range(40))
    assert all(s.empty for s in slots)


def test_file_info_lengths():
    payload = bytearray(EMPTY_FILE_INFO)
    struct.pack_into("<BI", payload, 1 + 5 * 3, 3, 441000)
    s = p.parse_file_info(bytes(payload))[3]
    assert s.index == 3 and s.seconds == 10.0 and s.pcm_bytes == 441000 * 6


def test_wav_header_matches_vendor_template():
    # Template bytes taken verbatim from the vendor exe @ 0x428020
    vendor = bytes.fromhex(
        "52494646cc2c0a0057415645666d74201000000001000200"
        "44ac00009809040006001800646174 61a82c0a00".replace(" ", "")
    )
    assert p.wav_header(0x0A2CA8) == vendor


def test_sample_conversion():
    s16 = struct.pack("<h", -2)  # 0xFFFE
    assert _to_24bit(s16, 2) == b"\x00\xfe\xff"
    s32 = struct.pack("<i", 0x12345678)
    assert _to_24bit(s32, 4) == b"\x56\x34\x12"
    assert _mono_to_stereo(b"\x01\x02\x03") == b"\x01\x02\x03\x01\x02\x03"


def test_load_wav_16bit_mono(tmp_path):
    path = tmp_path / "t.wav"
    with wave.open(str(path), "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(44100)
        w.writeframes(struct.pack("<hh", 1, -1))
    assert load_pcm(path) == b"\x00\x01\x00" * 2 + b"\x00\xff\xff" * 2
