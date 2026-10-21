# Firmware recon: XS04 LOOP&DRUM

Status: **read-only recon, nothing written to the pedal.**

## Known from software and USB

- OEM: **YUER** (USB manufacturer string `"YUER "`). The product string is `XS04 LOOP&DRUM`, and VID `0xBB00` is not a registered vendor ID.
- **USB 2.0 High-Speed** (512-byte bulk endpoints). That rules out the many cheap audio SoCs whose USB is full-speed only, and is a useful filter once the chip markings are known.
- WinUSB binding through MS OS descriptors, so the firmware implements the MS OS 1.0/2.0 descriptor handshake.
- Audio is stored as **uncompressed 24-bit stereo PCM at 44.1 kHz** (264.6 KB/s). There are 40 slots, and an import can be up to 240 s (~63.5 MB). That much storage implies **external storage** (SPI-NAND, eMMC or an SD-type part), not just the SoC's code flash.
- The version command returns only `"Loop"`, with no build number.
- The vendor PC app has **no firmware-update code path** (no update/firmware/DFU strings and no unused command IDs in its dispatcher; commands 0–7 are all accounted for). Any official updater would be a separate tool.

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
