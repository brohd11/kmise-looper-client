"""Convert audio files to the pedal's native PCM (24-bit LE, stereo, 44.1 kHz)."""
from __future__ import annotations

import shutil
import subprocess
import wave
from pathlib import Path

from . import protocol as p


class AudioError(RuntimeError):
    pass


def _to_24bit(frames: bytes, width: int) -> bytes:
    n = len(frames) // width
    out = bytearray(n * 3)
    if width == 3:
        return frames
    if width == 2:  # pad a zero LSB
        out[1::3] = frames[0::2]
        out[2::3] = frames[1::2]
    elif width == 4:  # drop the LSB, as the vendor app does
        out[0::3] = frames[1::4]
        out[1::3] = frames[2::4]
        out[2::3] = frames[3::4]
    else:
        raise AudioError(f"unsupported sample width {width * 8}-bit")
    return bytes(out)


def _mono_to_stereo(pcm24: bytes) -> bytes:
    out = bytearray(len(pcm24) * 2)
    for k in range(3):
        out[k::6] = pcm24[k::3]
        out[3 + k::6] = pcm24[k::3]
    return bytes(out)


def _read_wav(path: Path) -> bytes | None:
    """Fast path for 44.1 kHz integer WAV; None if ffmpeg is needed."""
    try:
        with wave.open(str(path), "rb") as w:
            if w.getframerate() != p.SAMPLE_RATE or w.getnchannels() not in (1, 2):
                return None
            if w.getsampwidth() not in (2, 3, 4):
                return None
            pcm = _to_24bit(w.readframes(w.getnframes()), w.getsampwidth())
            return _mono_to_stereo(pcm) if w.getnchannels() == 1 else pcm
    except (wave.Error, EOFError):
        return None


def _read_ffmpeg(path: Path) -> bytes:
    ffmpeg = shutil.which("ffmpeg")
    if not ffmpeg:
        raise AudioError(
            f"{path.name}: only 44.1 kHz 16/24/32-bit WAV is supported without ffmpeg on PATH"
        )
    cmd = [ffmpeg, "-v", "error", "-i", str(path), "-f", "s24le", "-acodec", "pcm_s24le",
           "-ac", str(p.CHANNELS), "-ar", str(p.SAMPLE_RATE), "-"]
    res = subprocess.run(cmd, capture_output=True)
    if res.returncode != 0:
        raise AudioError(f"ffmpeg failed: {res.stderr.decode(errors='replace').strip()}")
    return res.stdout


def load_pcm(path: Path) -> bytes:
    pcm = _read_wav(path)
    if pcm is None:
        pcm = _read_ffmpeg(path)
    pcm = pcm[: len(pcm) - len(pcm) % p.FRAME_BYTES]
    if not pcm:
        raise AudioError(f"{path.name}: no audio")
    return pcm
