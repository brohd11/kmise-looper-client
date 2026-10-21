# kmise

Open-source replacement for the "KMISE LOOP&DRUM" PC software for the YUER **XS04 LOOP&DRUM**
looper (also sold as lotmusic, Flamma, Donner…). It adds bulk export, which the vendor app lacks.

```
pip install -e .
kmise info
kmise list [--all]
kmise export --all -o backup/        # or: kmise export 3 7 10-15
kmise import 5 song.wav [--force]    # 44.1 kHz WAV natively; anything else via ffmpeg on PATH
kmise delete 5                       # asks for confirmation; --all deletes everything
```

Close the vendor app first, since only one program can claim the USB interface.

## Platform setup

The pedal is a vendor-class USB device that speaks the same protocol to every OS. The Windows and
Mac vendor apps are the same code base. `libusb-package` bundles libusb, so nothing else is needed
except as noted below.

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

If no bundled libusb wheel exists for your platform, install the system libusb
(`apt install libusb-1.0-0`, `brew install libusb`). It's picked up automatically.

Exports are 24-bit stereo 44.1 kHz WAV, bit-exact with what the pedal stores.

- Protocol: [docs/protocol.md](docs/protocol.md)
- Firmware notes: [docs/firmware_recon.md](docs/firmware_recon.md)
- RE helpers: `tools/` (capstone/pefile scripts used on the unpacked vendor exe in `re/`)
