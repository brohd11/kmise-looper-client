"""Dump USB descriptors of the XS04 LOOP&DRUM (read-only)."""
import libusb_package
import usb.core
import usb.util

dev = libusb_package.find(idVendor=0xBB00, idProduct=0x0001)
if dev is None:
    raise SystemExit("device not found")
print(dev)
for idx in (dev.iManufacturer, dev.iProduct, dev.iSerialNumber):
    if idx:
        try:
            print(f"string[{idx}] = {usb.util.get_string(dev, idx)!r}")
        except Exception as e:
            print(f"string[{idx}] error: {e}")
