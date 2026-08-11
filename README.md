# gpib

A minimal Go wrapper around National Instruments' **NI-488.2 driver** (`ni4882.dll`) for talking to GPIB instruments — **no cgo required**. It calls into the DLL directly via `syscall.LazyDLL`, so building and cross-compiling work exactly like any other pure-Go package (no C toolchain, no `CGO_ENABLED=1`).

Windows only, since `ni4882.dll` and the NI-488.2 driver themselves are Windows-only. The package won't build on other platforms (it's gated behind a `//go:build windows` tag).

## Requirements

- Windows
- [NI-488.2 driver](https://www.ni.com/en/support/downloads/drivers/download.ni-488-2.html) installed (provides `ni4882.dll`)
- A GPIB interface board configured as board index 0

## Install

```
go get github.com/JaimeMaertz/gpib
```

## Usage

```go
var dev gpib.Device

if err := dev.Open("scope", 22); err != nil {
    log.Fatal(err)
}
defer dev.Close()

if err := dev.Write("*IDN?"); err != nil {
    log.Fatal(err)
}

resp, err := dev.Read()
if err != nil {
    log.Fatal(err)
}

fmt.Println(resp)
```

See [`examples/basic`](examples/basic) for a complete program, including how to pull the raw `ibsta` status word out of a failed call.

## API

```go
type Device struct { /* ... */ }

func (d *Device) Open(name string, address uint16) error
func (d *Device) Close() error
func (d *Device) Write(command string) error
func (d *Device) Read() (string, error)
```

- **`Open`** opens the instrument at the given primary GPIB address. Board index, secondary address, timeout, EOI-on-write, and EOS are fixed at sane defaults (board 0, no secondary address, 10s timeout, EOI asserted, no EOS character) — see the `default*` constants in `gpib.go` if you need to change them for your setup.
- **`Write`** appends a trailing newline and sends the command.
- **`Read`** reads a single response, up to 256 bytes, with surrounding whitespace trimmed. A longer response is truncated.
- **`Close`** takes the device offline. Safe to call even if `Open` never succeeded.

All four methods return `nil` on success. On failure from `Write`, `Read`, or `Close`, the error is a `*gpib.StatusError` carrying the raw `ibsta` status word the driver returned — check it against the `Status*` constants (`StatusErr`, `StatusTimo`, `StatusEnd`, `StatusCmpl`) or the NI-488.2 Programmer's Reference Manual for the specific condition:

```go
var statusErr *gpib.StatusError
if errors.As(err, &statusErr) {
    fmt.Printf("ibsta = 0x%04x\n", statusErr.Ibsta)
}
```

## Known limitations

- **Single device open per `Device`, one goroutine at a time.** `Device` is not safe for concurrent use.
- **Fixed board/timeout/EOS settings.** `Open` doesn't expose board index, secondary address, timeout, EOT, or EOS as parameters — they're constants in `gpib.go`. Fine for a single-board, single-timeout setup; edit the constants (or send a PR to make them configurable) if you need otherwise.
- **256-byte read buffer.** Responses longer than that are silently truncated rather than read in multiple passes.
- **Hardware-verify before you trust it.** The error handling here was rewritten to check the driver's `ibsta` status word (the documented NI-488.2 failure signal) instead of the Windows syscall error the original version checked — but it hasn't been verified against real hardware. Test it against your instrument before relying on it.

## License

MIT — see [LICENSE](LICENSE).
