# kmise

Open-source replacement for the "KMISE LOOP&DRUM" PC software for the YUER **XS04 LOOP&DRUM**
looper (also sold as lotmusic, Flamma, Donner…). It adds bulk export, which the vendor app lacks.

## Install

A single binary per platform, with libusb built in. Nothing else to install.

```sh
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/brohd11/kmise-looper-client/main/install.sh | sh
```
```powershell
# Windows
irm https://raw.githubusercontent.com/brohd11/kmise-looper-client/main/install.ps1 | iex
```

## Use

```
kmise info
kmise list [--all]
kmise export --all -o backup/        # or: kmise export 3 7 10-15
kmise import 5 song.wav [--force]    # 44.1 kHz WAV natively; anything else via ffmpeg on PATH
kmise delete 5                       # asks for confirmation; --all deletes everything
```

Close the vendor app first, since only one program can claim the USB interface.
Exports are 24-bit stereo 44.1 kHz WAV, bit-exact with what the pedal stores.

## Platform setup

The pedal is a vendor-class USB device that speaks the same protocol to every OS.

| OS | Setup |
|---|---|
| Windows | Nothing. The pedal binds to the built-in WinUSB driver automatically. |
| macOS | Nothing. |
| Linux | For non-root access, install the udev rule (below). |

```sh
sudo cp udev/60-kmise.rules /etc/udev/rules.d/
sudo udevadm control --reload && sudo udevadm trigger
# then replug the pedal
```

## Development

Go, with [gousb](https://github.com/google/gousb) (cgo + libusb). Packages:

| Package | Role |
|---|---|
| `protocol` | packet framing, payload parsing, WAV header (no I/O) |
| `audio` | audio file → pedal PCM (WAV natively, ffmpeg fallback) |
| `device` | USB access; the only package that touches libusb |
| `looper` | operations with their safety checks, shared by the CLI and any future TUI/GUI |
| `cmd` | the `kmise` CLI |

Building needs a C compiler and a static libusb, which `scripts/build-libusb.sh` builds:

```sh
# macOS:   xcode-select --install && brew install pkgconf
# Linux:   apt install build-essential pkg-config bzip2   (or your distro's equivalent)
# Windows: in an MSYS2 UCRT64 shell with
#   pacman -S make bzip2 diffutils mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-pkgconf
eval "$(bash scripts/build-libusb.sh .libusb)"
make build      # -> build/<os>-<arch>/kmise
make test
```

`protocol`, `audio` and `looper` don't need cgo: `go test ./protocol ./audio ./looper`.

Shared install scripts, the makefile body and `cliff.toml` come from the `sh-templates`
submodule: `./render-go.sh` (or `--check`). `test.yml` and `release.yml` are custom, because
the shared workflows cross-compile with `CGO_ENABLED=0`. Releases build natively on each OS;
push a `v*` tag.

The original Python implementation (`kmise/`, `pip install -e .`) is kept as a reference until
the Go port is verified on hardware.

- Protocol: [docs/protocol.md](docs/protocol.md)
- Firmware notes: [docs/firmware_recon.md](docs/firmware_recon.md)
- RE helpers: `tools/` (capstone/pefile scripts used on the unpacked vendor exe in `re/`)
