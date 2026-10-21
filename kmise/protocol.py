"""Packet framing and payload parsing for the XS04 LOOP&DRUM (see docs/protocol.md)."""
from __future__ import annotations

import struct
from dataclasses import dataclass

VID = 0xBB00
PID = 0x0001
EP_OUT = 0x02
EP_IN = 0x81
PACKET_SIZE = 512
NUM_SLOTS = 40

MAGIC_A = 0x55AA6969
MAGIC_B = 0x9696A5A5
TRAILER = 0xAA559696
TRAILER_OFFSET = 0x1FC
PAYLOAD_OFFSET = 9
MAX_PAYLOAD = TRAILER_OFFSET - PAYLOAD_OFFSET

CMD_VERSION = 0
CMD_FILE_INFO = 1
CMD_DELETE = 2
CMD_EXPORT = 3
CMD_EXPORT_DONE = 4
CMD_IMPORT = 5
CMD_IMPORT_DONE = 6
CMD_IMPORT_READY = 7

SAMPLE_RATE = 44100
CHANNELS = 2
SAMPLE_BYTES = 3  # 24-bit
FRAME_BYTES = CHANNELS * SAMPLE_BYTES
MAX_IMPORT_BYTES = 0x3C8FE80  # 240 s, the vendor app's limit


def build_packet(cmd: int, payload: bytes = b"") -> bytes:
    if len(payload) > MAX_PAYLOAD:
        raise ValueError(f"payload too long ({len(payload)} > {MAX_PAYLOAD})")
    pkt = bytearray(PACKET_SIZE)
    struct.pack_into("<IIB", pkt, 0, MAGIC_A, MAGIC_B, cmd)
    pkt[PAYLOAD_OFFSET:PAYLOAD_OFFSET + len(payload)] = payload
    struct.pack_into("<I", pkt, TRAILER_OFFSET, TRAILER)
    return bytes(pkt)


def parse_packet(data: bytes) -> tuple[int, bytes] | None:
    """Return (cmd, payload) for a framed packet, or None if this is raw data."""
    if len(data) != PACKET_SIZE:
        return None
    a, b, cmd = struct.unpack_from("<IIB", data, 0)
    (trailer,) = struct.unpack_from("<I", data, TRAILER_OFFSET)
    if a != MAGIC_A or b != MAGIC_B or trailer != TRAILER:
        return None
    return cmd & 0x7F, bytes(data[PAYLOAD_OFFSET:TRAILER_OFFSET])


@dataclass(frozen=True)
class Slot:
    index: int
    frames: int

    @property
    def empty(self) -> bool:
        return self.frames == 0

    @property
    def seconds(self) -> float:
        return self.frames / SAMPLE_RATE

    @property
    def pcm_bytes(self) -> int:
        return self.frames * FRAME_BYTES


def parse_file_info(payload: bytes) -> list[Slot]:
    """`[count:u8]` then `count` x `[slot:u8][frames:u32 LE]`."""
    count = payload[0]
    return [Slot(*struct.unpack_from("<BI", payload, 1 + 5 * i)) for i in range(count)]


def parse_version(payload: bytes) -> str:
    return payload.split(b"\0", 1)[0].decode("ascii", "replace")


def wav_header(pcm_len: int) -> bytes:
    """44-byte header matching the vendor app's template (PCM, 2ch, 44.1 kHz, 24-bit)."""
    byte_rate = SAMPLE_RATE * FRAME_BYTES
    return struct.pack(
        "<4sI4s4sIHHIIHH4sI",
        b"RIFF", pcm_len + 36, b"WAVE", b"fmt ", 16, 1, CHANNELS,
        SAMPLE_RATE, byte_rate, FRAME_BYTES, SAMPLE_BYTES * 8, b"data", pcm_len,
    )
