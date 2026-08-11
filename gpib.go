//go:build windows

// Package gpib is a minimal wrapper around National Instruments' NI-488.2
// driver (ni4882.dll) for talking to GPIB instruments.
//
// It calls into the DLL directly via syscall.LazyDLL, so it does not
// require cgo or a C toolchain — building and cross-compiling work like
// any other pure-Go package. It only builds on Windows, since ni4882.dll
// and the NI-488.2 driver are Windows-only.
//
// A Device is not safe for concurrent use. Serialize calls to a given
// Device, or add your own locking around it.
package github.com/JaimeMaertz/gpib

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

var (
	dll = syscall.NewLazyDLL("ni4882.dll")

	procIbdev = dll.NewProc("ibdev") // open a device
	procIbwrt = dll.NewProc("ibwrt") // write to a device
	procIbrd  = dll.NewProc("ibrd")  // read from a device
	procIbonl = dll.NewProc("ibonl") // take a device on/offline
)

// Status bits from the ibsta word the driver returns from ibwrt, ibrd, and
// ibonl. See the NI-488.2 Programmer's Reference Manual for the full bit
// layout; these are the ones this package acts on.
const (
	StatusErr  = 0x8000 // ERR: the call failed; see the returned ibsta for detail
	StatusTimo = 0x4000 // TIMO: the call timed out
	StatusEnd  = 0x2000 // END: EOI or the configured EOS character was detected
	StatusCmpl = 0x0100 // CMPL: I/O completed
)

// Fixed ibdev parameters used by Open for every device. board selects the
// GPIB interface card (0 is the first/only board on most setups); sad, eot,
// and eos are the secondary address, EOI-on-write, and end-of-string
// settings. timeout is the NI-488.2 T10s enum value (10 second timeout).
const (
	defaultBoard   = 0
	defaultSad     = 0
	defaultTimeout = 13 // T10s
	defaultEOT     = 1
	defaultEOS     = 0
)

// readBufferSize is the maximum response size Read will return in one call.
const readBufferSize = 256

// Device represents one instrument opened over GPIB via the NI-488.2
// driver. The zero value is not usable; call Open to obtain a ready Device.
type Device struct {
	address uint16
	name    string
	handle  uintptr
	open    bool
}

// StatusError reports a failed NI-488.2 call. Ibsta is the raw status word
// the driver returned; compare it against the Status* constants, or the
// NI-488.2 reference manual, for the specific condition.
type StatusError struct {
	Op    string
	Ibsta uint32
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("gpib: %s failed: ibsta=0x%04x", e.Op, e.Ibsta)
}

// statusError turns the ibsta word returned by ibwrt/ibrd/ibonl into an
// error, or nil if the ERR bit is not set.
func statusError(op string, ibsta uintptr) error {
	sta := uint32(ibsta)
	if sta&StatusErr != 0 {
		return &StatusError{Op: op, Ibsta: sta}
	}
	return nil
}

// Open opens the instrument at the given GPIB primary address and readies
// it for Write and Read. Board, secondary address, timeout, EOT, and EOS
// are fixed at this package's defaults — see the default* constants.
//
// Unlike ibwrt/ibrd/ibonl, ibdev does not return ibsta; the driver signals
// failure by returning a negative unit descriptor instead.
func (d *Device) Open(name string, address uint16) error {
	handle, _, _ := procIbdev.Call(
		uintptr(defaultBoard),
		uintptr(address),
		uintptr(defaultSad),
		uintptr(defaultTimeout),
		uintptr(defaultEOT),
		uintptr(defaultEOS),
	)

	if int32(handle) < 0 {
		return fmt.Errorf("gpib: ibdev failed to open device %q at address %d", name, address)
	}

	d.name = name
	d.handle = handle
	d.address = address
	d.open = true
	return nil
}

// Close takes the device offline. Calling Close on a Device that was never
// successfully opened is a no-op.
func (d *Device) Close() error {
	if !d.open {
		return nil
	}
	ibsta, _, _ := procIbonl.Call(d.handle, 0)
	d.open = false
	return statusError("ibonl", ibsta)
}

// Write sends command to the device, appending the trailing newline the
// instrument expects.
func (d *Device) Write(command string) error {
	if !d.open {
		return fmt.Errorf("gpib: device not open")
	}
	cmd := []byte(command + "\n")

	ibsta, _, _ := procIbwrt.Call(
		d.handle,
		uintptr(unsafe.Pointer(&cmd[0])),
		uintptr(len(cmd)),
	)

	return statusError("ibwrt", ibsta)
}

// Read reads a single response from the device, up to readBufferSize
// bytes, and returns it with surrounding whitespace trimmed. A response
// longer than readBufferSize is truncated.
func (d *Device) Read() (string, error) {
	if !d.open {
		return "", fmt.Errorf("gpib: device not open")
	}
	buf := make([]byte, readBufferSize)

	ibsta, _, _ := procIbrd.Call(
		d.handle,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)-1),
	)

	if err := statusError("ibrd", ibsta); err != nil {
		return "", err
	}

	n := 0
	for n < len(buf) && buf[n] != 0 {
		n++
	}

	return strings.TrimSpace(string(buf[:n])), nil
}
