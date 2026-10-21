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
No driver install is needed: the pedal binds to Windows' built-in WinUSB.

Exports are 24-bit stereo 44.1 kHz WAV, bit-exact with what the pedal stores.

- Protocol: [docs/protocol.md](docs/protocol.md)
- Firmware notes: [docs/firmware_recon.md](docs/firmware_recon.md)
- RE helpers: `tools/` (capstone/pefile scripts used on the unpacked vendor exe in `re/`)
