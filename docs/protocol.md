# XS04 LOOP&DRUM USB protocol

Recovered by static analysis of `KMISE LOOP&DRUM V1.0.1` (Qt5/MinGW, unpacked from its
Enigma Virtual Box bundle). Addresses refer to the unpacked main exe (image base 0x400000).

## Device

| Field | Value |
|---|---|
| VID / PID | `0xBB00` / `0x0001` |
| Manufacturer / product strings | `YUER ` / `XS04 LOOP&DRUM` |
| Speed | USB 2.0 High-Speed |
| Interface 0 | class `FF/FF/FF`, bound to WinUSB through MS OS descriptors (no custom driver) |
| EP `0x02` | Bulk OUT, 512 bytes |
| EP `0x81` | Bulk IN, 512 bytes |

The vendor app keeps one async IN transfer (512 bytes) permanently resubmitted, and
issues OUT transfers as needed.

## Platforms

The macOS build (`KMISE LOOP&DRUM V1.0.1 Setup_mac.dmg`, an Intel x86-64 Mach-O) is the same
code base. It has an identical `UsbDevice` moc method table, the same magic/trailer constants, the
same `cmd & 0x7F` dispatch over commands 0–7, the same WAV header template, and statically linked
libusb. The protocol is OS-independent.

## Framing (both directions)

Every command or response is exactly 512 bytes:

| Offset | Bytes | Meaning |
|---|---|---|
| 0x000 | `69 69 AA 55` | magic A (`0x55AA6969` LE) |
| 0x004 | `A5 A5 96 96` | magic B (`0x9696A5A5` LE) |
| 0x008 | 1 | command (the device may set bit 7 on responses; the host masks with `& 0x7F`) |
| 0x009 | … | payload |
| 0x1FC | `96 96 55 AA` | trailer (`0xAA559696` LE) |

A 512-byte IN packet that fails the magic/trailer check counts as **raw bulk data**
(used for export streaming).

Host sender: `send(cmd, buf, len)` @ `0x404960`. OUT timeout is 100 ms for commands
and 8000 ms for import data chunks.

## Commands

| cmd | Direction | Payload | Notes |
|---|---|---|---|
| 0 | H→D | – | Request device version |
| 0 | D→H | NUL-terminated ASCII string | Version (this unit returns `"Loop"`) |
| 1 | H→D | – | Request file info (slot list) |
| 1 | D→H | `[count:u8]` then `count` × `[slot:u8][frames:u32 LE]` | One entry per slot (count = 40, confirmed on hardware). `frames` = length in samples at 44.1 kHz (the app multiplies by 1/44100 to get seconds). 0 means empty |
| 2 | H→D | `[n:u8][slot:u8]×n` | Delete slots. The app sends `[1, idx]` or `[40, 0..39]` (delete all) |
| 2 | D→H | – | Delete done; the app re-requests file info |
| 3 | H→D | `[slot:u8]` | Begin export |
| 3 | D→H | `[length:u32 LE]` | Byte length of the PCM that follows as raw 512-byte IN packets |
| 4 | D→H | – | Export stream complete |
| 5 | H→D | `[slot:u8][length:u32 LE]` | Begin import of `length` PCM bytes |
| 5 | D→H | `[status:u8]` | 0 = OK, go ahead |
| (raw) | H→D | 512-byte chunks of PCM | Import data, no framing. The last chunk is zero-padded to 512 |
| 7 | D→H | – | Ready for next import chunk |
| 6 | H→D | `[slot:u8]` + zero padding (0x21 bytes total) | Import done |

There are 40 slots (0–39).

## Audio format

- Export data is raw **PCM, 2 ch, 44 100 Hz, 24-bit signed LE, interleaved** (6 bytes per frame).
  The app writes a fixed 44-byte WAV header (template @ `0x428020`) and patches the
  RIFF/data sizes.
- Import: the app decodes any format with FFmpeg and resamples to `S32` stereo 44.1 kHz
  (`swr_alloc_set_opts` @ `0x403939`), then keeps bytes 1–3 of each sample, which gives the
  same 24-bit LE stereo format. The buffer is capped at `0x3C8FE80` bytes (240 s).

## Export sequence

```
H→D  cmd 3 [slot]
D→H  cmd 3 [len:u32]
D→H  raw 512-byte packets until len bytes are received (the last packet may be padded)
D→H  cmd 4
```

## Import sequence

```
H→D  cmd 5 [slot][len:u32]
D→H  cmd 5 [0]
loop:
  H→D  raw 512-byte chunk
  D→H  cmd 7                (wait before sending the next chunk)
H→D  cmd 6 [slot]
H→D  cmd 1                  (refresh list)
```

## Key functions (unpacked exe)

| Address | Function |
|---|---|
| `0x404960` | `UsbDevice::send(cmd, buf, len)` (frames the 512-byte packet) |
| `0x4048b0` | Raw async bulk OUT (`EP 0x02`) |
| `0x4058f0` | IN transfer callback / response dispatcher (jump table @ `0x1958840`) |
| `0x406f20` | `UsbDevice::run()`, the worker loop (export file writing, import chunking) |
| `0x403630` | FFmpeg decode → 24-bit PCM buffer for import |
| `0x405d00`–`0x405f80` | `request*` slots (moc table @ `0x1976de0`) |
