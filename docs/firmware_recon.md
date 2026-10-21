# Firmware recon: XS04 LOOP&DRUM

Status: **read-only recon, nothing written to the pedal.**

## Known from software and USB

- OEM: **YUER** (USB manufacturer string `"YUER "`). The product string is `XS04 LOOP&DRUM`, and VID `0xBB00` is not a registered vendor ID.
- **USB 2.0 High-Speed** (512-byte bulk endpoints). That rules out the many cheap audio SoCs whose USB is full-speed only, and is a useful filter once the chip markings are known.
- WinUSB binding through MS OS descriptors, so the firmware implements the MS OS 1.0/2.0 descriptor handshake.
- Audio is stored as **uncompressed 24-bit stereo PCM at 44.1 kHz** (264.6 KB/s). There are 40 slots, and an import can be up to 240 s (~63.5 MB). That much storage implies **external storage** (SPI-NAND, eMMC or an SD-type part), not just the SoC's code flash.
- The version command returns only `"Loop"`, with no build number.
- The vendor PC app has **no firmware-update code path** (no update/firmware/DFU strings and no unused command IDs in its dispatcher; commands 0–7 are all accounted for). Any official updater would be a separate tool.

## Upgrade-mode probe (2026-10-20)

Holding both footswitches at power-on (the Flamma FS21's documented "upgrade mode" combo)
produced **no new USB device**. Only the normal `BB00:0001` enumerated, and the screen didn't change.

## Sibling products

### Flamma FS21: same YUER platform (strong match)
`FS21 DRUM&LOOP V1.0.1 Setup_win.exe` (Flamma's official Drive) is a Smart Install Maker
installer. Its file table lists:
- `FLAMMA FS21.exe` (the editor), `FLAMMA FS21 Update.exe` (the flasher), `libusb-1.0.dll`
- **`Firmware\FLAMMA FS21_V1.0.1.mr`**, a firmware image
- `Driver\CIRCLE_LOOPER.inf` + libusbK/WinUSB co-installers. The driver is named after the **Donner
  Circle Looper**, so Flamma, Donner and KMISE share this platform.

The payload is compressed with an obfuscated 7z-style header (first header byte `0xDF`), and
neither libarchive nor WinRAR opens it. Extracting it means installing the package or reversing
the stub's unpacker.

### Flamma FF20: different platform (Mooer), reference only
`FLAMMA+FF20_V1.0.3_WIN.exe` is an NSIS installer → Electron app (`app.asar`, readable JS)
+ `resources/firmware/FF20_App.mr` (194 MB). It's built by **Mooer**: USB VID `0x34DB`, HID,
`AA 55 len16 cmd … CRC16` framing, and a `MOOER_FILE_HEAD` header in the `.mr`. It's useful as a
reference for how these updaters work:
- App mode: `cmd 14` reboots into a bootloader with a different PID (`0xF000`).
- Bootloader: erase (block list from the `.mr` header), page writes and `0xFF` reboot.
  **There's no read-back command**, so a USB dump isn't possible on that design.
- The `.mr` container is a 128-byte file head followed by N × (512-byte block tag + data). Block tags
  carry the ROM id, start/stop address, size and start page.

Extraction helpers: `tools/nsis_blobs.py`, `tools/asar_extract.py`, `tools/dmg_raw.py`.

## Next step: hardware photos needed

Open the pedal and photograph, in focus and with readable markings:

1. The main SoC (the largest QFP/QFN). Get every line of text, including the date code.
2. Any 8-pin SOIC/WSON (SPI NOR/NAND flash, e.g. `25Qxx`, `W25N`, `GD5F`) and any BGA/TSOP storage part.
3. The audio codec (DAC/ADC) near the jacks.
4. Test pads, unpopulated headers, or pads labelled `TX/RX`, `SWD`, `BOOT`, `UPDATE`, `DP/DM`.
5. Both sides of the board.

With the SoC identified, I'll assess:
- whether it has a mask-ROM USB/UART boot mode, and which existing tools can **dump** flash through it
- whether an SPI flash can be dumped off-board, or in-circuit with a CH341A plus SOIC clip (a reversible read)
- the expected firmware image format and how to load it into Ghidra
