"""Synchronous USB access to the XS04 LOOP&DRUM. The only module that touches USB."""
from __future__ import annotations

import struct
import time
from typing import Callable

import libusb_package
import usb.core
import usb.util

from . import protocol as p

Progress = Callable[[int, int], None]


class DeviceError(RuntimeError):
    pass


class Device:
    def __init__(self, timeout_ms: int = 2000):
        dev = libusb_package.find(idVendor=p.VID, idProduct=p.PID)
        if dev is None:
            raise DeviceError("XS04 LOOP&DRUM not found (is it plugged in and powered on?)")
        self.dev = dev
        self.timeout_ms = timeout_ms
        try:
            dev.set_configuration()
        except usb.core.USBError:
            pass  # already configured (WinUSB)
        try:
            usb.util.claim_interface(dev, 0)
        except usb.core.USBError as e:
            raise DeviceError(f"cannot claim interface ({e}); close the KMISE app first") from e
        self.drain()

    def close(self) -> None:
        usb.util.release_interface(self.dev, 0)
        usb.util.dispose_resources(self.dev)

    def __enter__(self) -> "Device":
        return self

    def __exit__(self, *exc) -> None:
        self.close()

    # -- low level -------------------------------------------------------

    def _write(self, data: bytes, timeout_ms: int | None = None) -> None:
        self.dev.write(p.EP_OUT, data, timeout=timeout_ms or self.timeout_ms)

    def _read(self, timeout_ms: int | None = None) -> bytes:
        return bytes(self.dev.read(p.EP_IN, p.PACKET_SIZE, timeout=timeout_ms or self.timeout_ms))

    def drain(self) -> None:
        """Discard any stale IN packets."""
        while True:
            try:
                self._read(timeout_ms=50)
            except usb.core.USBTimeoutError:
                return

    def send(self, cmd: int, payload: bytes = b"") -> None:
        self._write(p.build_packet(cmd, payload))

    def expect(self, cmd: int, timeout_ms: int | None = None) -> bytes:
        """Read until a framed packet with `cmd` arrives; return its payload."""
        deadline = time.monotonic() + (timeout_ms or self.timeout_ms) / 1000
        while True:
            remaining = int((deadline - time.monotonic()) * 1000)
            if remaining <= 0:
                raise DeviceError(f"timed out waiting for response to cmd {cmd}")
            try:
                data = self._read(timeout_ms=remaining)
            except usb.core.USBTimeoutError:
                continue
            parsed = p.parse_packet(data)
            if parsed and parsed[0] == cmd:
                return parsed[1]

    # -- operations ------------------------------------------------------

    def version(self) -> str:
        self.send(p.CMD_VERSION)
        return p.parse_version(self.expect(p.CMD_VERSION))

    def list_slots(self) -> list[p.Slot]:
        self.send(p.CMD_FILE_INFO)
        return p.parse_file_info(self.expect(p.CMD_FILE_INFO))

    def read_slot(self, slot: int, progress: Progress | None = None) -> bytes:
        """Export one slot; returns raw 24-bit stereo PCM."""
        self.send(p.CMD_EXPORT, bytes([slot]))
        (total,) = struct.unpack_from("<I", self.expect(p.CMD_EXPORT))
        buf = bytearray()
        while len(buf) < total:
            data = self._read(timeout_ms=5000)
            parsed = p.parse_packet(data)
            if parsed is not None:
                if parsed[0] == p.CMD_EXPORT_DONE:
                    break
                continue
            buf += data[: total - len(buf)]
            if progress:
                progress(len(buf), total)
        else:
            self.expect(p.CMD_EXPORT_DONE, timeout_ms=5000)
        if len(buf) != total:
            raise DeviceError(f"slot {slot}: got {len(buf)} of {total} bytes")
        return bytes(buf)

    def write_slot(self, slot: int, pcm: bytes, progress: Progress | None = None) -> None:
        """Import raw 24-bit stereo PCM into a slot."""
        if len(pcm) > p.MAX_IMPORT_BYTES:
            raise DeviceError("audio longer than 240 s")
        self.send(p.CMD_IMPORT, struct.pack("<BI", slot, len(pcm)))
        status = self.expect(p.CMD_IMPORT)[0]
        if status != 0:
            raise DeviceError(f"device refused import (status {status})")
        for off in range(0, len(pcm), p.PACKET_SIZE):
            chunk = pcm[off:off + p.PACKET_SIZE].ljust(p.PACKET_SIZE, b"\0")
            self._write(chunk, timeout_ms=8000)
            self.expect(p.CMD_IMPORT_READY, timeout_ms=8000)
            if progress:
                progress(min(off + p.PACKET_SIZE, len(pcm)), len(pcm))
        self.send(p.CMD_IMPORT_DONE, bytes([slot]) + bytes(0x20))

    def delete(self, slots: list[int]) -> None:
        self.send(p.CMD_DELETE, bytes([len(slots), *slots]))
        self.expect(p.CMD_DELETE, timeout_ms=10000)
