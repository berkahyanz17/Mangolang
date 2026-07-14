# mycli

A starter Go CLI tool built with **only the standard library** (no external
dependencies, so `go build` works even with restricted/offline networks).

## Structure

```
mycli/
├── main.go                  # entrypoint
├── cmd/
│   ├── root.go               # command registry + dispatcher (flag-based)
│   ├── greet.go               # example subcommand
│   └── config.go              # example subcommand: reads config
├── internal/
│   └── config/
│       └── config.go          # JSON + env var config loader
├── go.mod
└── mycli.json                 # example config file (optional)
```

## Build & run

```bash
go build -o mycli .
./mycli               # shows help
./mycli --version
./mycli greet --name Budi --loud
./mycli config --file mycli.json
```

## Adding a new subcommand

Create a new file in `cmd/`, e.g. `cmd/hello.go`:

```go
package cmd

func init() {
    Register(&Command{
        Name:        "hello",
        Description: "Say hello",
        Run: func(args []string) error {
            fs := NewFlagSet("hello")
            fs.Parse(args)
            fmt.Println("hello!")
            return nil
        },
    })
}
```

It'll automatically show up in `mycli --help` and be dispatchable via
`mycli hello`.

## Upgrading to Cobra later

This template mirrors Cobra's mental model (root dispatcher + registered
subcommands) on purpose. If you later have proper network access and want
`github.com/spf13/cobra` + `spf13/viper` for richer flag parsing, nested
subcommands, shell completion, etc., you can swap `cmd/root.go` for a real
`cobra.Command` tree with minimal restructuring — the subcommand files stay
conceptually the same.
