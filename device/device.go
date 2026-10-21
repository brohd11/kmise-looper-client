// Package device provides synchronous USB access to the XS04 LOOP&DRUM. It is the only
// package that touches USB. A Device is not safe for concurrent use.
package device

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/google/gousb"

	"github.com/brohd11/kmise-looper-client/protocol"
)

const defaultTimeout = 2 * time.Second

// Progress reports bytes transferred so far out of total.
type Progress = func(done, total int)

// Error is returned for device-level failures with a user-facing message.
type Error struct{ msg string }

func (e *Error) Error() string { return e.msg }

func errorf(format string, a ...any) error { return &Error{fmt.Sprintf(format, a...)} }

func accessHint() string {
	if runtime.GOOS == "linux" {
		return "install udev/60-kmise.rules (see README) or run as root, " +
			"and close any other program using the pedal"
	}
	return "close the KMISE app or any other program using the pedal"
}

// isAccess reports whether err is a libusb access error. gousb wraps some errors with %v,
// so fall back to matching the message.
func isAccess(err error) bool {
	return errors.Is(err, gousb.ErrorAccess) || strings.Contains(err.Error(), gousb.ErrorAccess.Error())
}

type Device struct {
	ctx     *gousb.Context
	dev     *gousb.Device
	release func()
	out     *gousb.OutEndpoint
	in      *gousb.InEndpoint
	buf     []byte
	Timeout time.Duration
}

// newContext wraps gousb.NewContext, which panics if libusb can't initialise (no USB
// subsystem, e.g. WSL or a container).
func newContext() (ctx *gousb.Context, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errorf("cannot initialise USB (%v); is a USB subsystem available?", r)
		}
	}()
	return gousb.NewContext(), nil
}

// Open finds the pedal, claims interface 0 and discards stale input.
func Open() (*Device, error) {
	ctx, err := newContext()
	if err != nil {
		return nil, err
	}
	d := &Device{ctx: ctx, buf: make([]byte, protocol.PacketSize), Timeout: defaultTimeout}
	ok := false
	defer func() {
		if !ok {
			d.Close()
		}
	}()

	dev, err := ctx.OpenDeviceWithVIDPID(protocol.VID, protocol.PID)
	if err != nil {
		if isAccess(err) {
			return nil, errorf("permission denied: %s", accessHint())
		}
		return nil, errorf("cannot open device (%v); %s", err, accessHint())
	}
	if dev == nil {
		return nil, errorf("XS04 LOOP&DRUM not found (is it plugged in and powered on?)")
	}
	d.dev = dev
	// Nothing should bind a vendor-class interface, but be safe. Unsupported on Windows.
	if runtime.GOOS != "windows" {
		_ = dev.SetAutoDetach(true)
	}
	// An unconfigured device (Linux, fresh plug) reports config 0; the pedal has one config.
	cfgNum, err := dev.ActiveConfigNum()
	if err != nil || cfgNum == 0 {
		cfgNum = 1
	}
	cfg, err := dev.Config(cfgNum)
	if err != nil {
		if isAccess(err) {
			return nil, errorf("permission denied: %s", accessHint())
		}
		return nil, errorf("cannot configure device (%v); %s", err, accessHint())
	}
	intf, err := cfg.Interface(0, 0)
	if err != nil {
		cfg.Close()
		return nil, errorf("cannot claim interface (%v); %s", err, accessHint())
	}
	d.release = func() {
		intf.Close()
		cfg.Close()
	}
	if d.out, err = intf.OutEndpoint(protocol.EPOut); err != nil {
		return nil, errorf("open OUT endpoint: %v", err)
	}
	if d.in, err = intf.InEndpoint(protocol.EPIn & 0x0F); err != nil {
		return nil, errorf("open IN endpoint: %v", err)
	}
	d.drain()
	ok = true
	return d, nil
}

// Close releases the interface and the USB context.
func (d *Device) Close() error {
	if d.release != nil {
		d.release()
		d.release = nil
	}
	if d.dev != nil {
		d.dev.Close()
		d.dev = nil
	}
	if d.ctx != nil {
		d.ctx.Close()
		d.ctx = nil
	}
	return nil
}

// -- low level -----------------------------------------------------------

func (d *Device) timeout(t time.Duration) time.Duration {
	if t <= 0 {
		return d.Timeout
	}
	return t
}

func (d *Device) write(data []byte, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout(timeout))
	defer cancel()
	if _, err := d.out.WriteContext(ctx, data); err != nil {
		if ctx.Err() != nil {
			return errorf("timed out writing to device")
		}
		return errorf("USB write failed: %v", err)
	}
	return nil
}

var errTimeout = errors.New("read timed out")

// read returns one IN transfer. The slice is reused by the next read.
func (d *Device) read(timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout(timeout))
	defer cancel()
	n, err := d.in.ReadContext(ctx, d.buf)
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, gousb.TransferTimedOut) {
			return nil, errTimeout
		}
		return nil, errorf("USB read failed: %v", err)
	}
	return d.buf[:n], nil
}

// drain discards any stale IN packets.
func (d *Device) drain() {
	for {
		if _, err := d.read(50 * time.Millisecond); err != nil {
			return
		}
	}
}

func (d *Device) send(cmd byte, payload []byte) error {
	pkt, err := protocol.BuildPacket(cmd, payload)
	if err != nil {
		return err
	}
	return d.write(pkt, 0)
}

// expect reads until a framed packet with cmd arrives and returns its payload.
func (d *Device) expect(cmd byte, timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(d.timeout(timeout))
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, errorf("timed out waiting for response to cmd %d", cmd)
		}
		data, err := d.read(remaining)
		if err == errTimeout {
			continue
		}
		if err != nil {
			return nil, err
		}
		if c, payload, ok := protocol.ParsePacket(data); ok && c == cmd {
			return append([]byte(nil), payload...), nil
		}
	}
}

// -- operations ----------------------------------------------------------

func (d *Device) Version() (string, error) {
	if err := d.send(protocol.CmdVersion, nil); err != nil {
		return "", err
	}
	p, err := d.expect(protocol.CmdVersion, 0)
	if err != nil {
		return "", err
	}
	return protocol.ParseVersion(p), nil
}

func (d *Device) ListSlots() ([]protocol.Slot, error) {
	if err := d.send(protocol.CmdFileInfo, nil); err != nil {
		return nil, err
	}
	p, err := d.expect(protocol.CmdFileInfo, 0)
	if err != nil {
		return nil, err
	}
	return protocol.ParseFileInfo(p)
}

// ReadSlot exports one slot and returns raw 24-bit stereo PCM. If ctx is cancelled the
// transfer is abandoned and stale input drained.
func (d *Device) ReadSlot(ctx context.Context, slot int, progress Progress) ([]byte, error) {
	if err := d.send(protocol.CmdExport, []byte{byte(slot)}); err != nil {
		return nil, err
	}
	p, err := d.expect(protocol.CmdExport, 0)
	if err != nil {
		return nil, err
	}
	total := int(binary.LittleEndian.Uint32(p))
	buf := make([]byte, 0, total)
	done := false
	for len(buf) < total {
		if err := ctx.Err(); err != nil {
			d.drain()
			return nil, err
		}
		data, err := d.read(5 * time.Second)
		if err == errTimeout {
			return nil, errorf("slot %d: timed out after %d of %d bytes", slot, len(buf), total)
		}
		if err != nil {
			return nil, err
		}
		if cmd, _, ok := protocol.ParsePacket(data); ok {
			if cmd == protocol.CmdExportDone {
				done = true
				break
			}
			continue
		}
		buf = append(buf, data[:min(len(data), total-len(buf))]...)
		if progress != nil {
			progress(len(buf), total)
		}
	}
	if !done {
		if _, err := d.expect(protocol.CmdExportDone, 5*time.Second); err != nil {
			return nil, err
		}
	}
	if len(buf) != total {
		return nil, errorf("slot %d: got %d of %d bytes", slot, len(buf), total)
	}
	return buf, nil
}

// WriteSlot imports raw 24-bit stereo PCM into a slot.
func (d *Device) WriteSlot(slot int, pcm []byte, progress Progress) error {
	if len(pcm) > protocol.MaxImportBytes {
		return errorf("audio longer than 240 s")
	}
	hdr := binary.LittleEndian.AppendUint32([]byte{byte(slot)}, uint32(len(pcm)))
	if err := d.send(protocol.CmdImport, hdr); err != nil {
		return err
	}
	status, err := d.expect(protocol.CmdImport, 0)
	if err != nil {
		return err
	}
	if status[0] != 0 {
		return errorf("device refused import (status %d)", status[0])
	}
	chunk := make([]byte, protocol.PacketSize)
	for off := 0; off < len(pcm); off += protocol.PacketSize {
		n := copy(chunk, pcm[off:])
		clear(chunk[n:])
		if err := d.write(chunk, 8*time.Second); err != nil {
			return err
		}
		if _, err := d.expect(protocol.CmdImportReady, 8*time.Second); err != nil {
			return err
		}
		if progress != nil {
			progress(off+n, len(pcm))
		}
	}
	return d.send(protocol.CmdImportDone, append([]byte{byte(slot)}, make([]byte, 0x20)...))
}

// Delete removes the given slots.
func (d *Device) Delete(slots []int) error {
	payload := []byte{byte(len(slots))}
	for _, s := range slots {
		payload = append(payload, byte(s))
	}
	if err := d.send(protocol.CmdDelete, payload); err != nil {
		return err
	}
	_, err := d.expect(protocol.CmdDelete, 10*time.Second)
	return err
}
