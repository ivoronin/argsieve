# argsieve

Go argument parsing with known/unknown flag separation for CLI wrappers.

[![CI](https://github.com/ivoronin/argsieve/actions/workflows/test.yml/badge.svg)](https://github.com/ivoronin/argsieve/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ivoronin/argsieve.svg)](https://pkg.go.dev/github.com/ivoronin/argsieve)

## Installation

```bash
go get github.com/ivoronin/argsieve
```

## Usage

```go
type Options struct {
    Verbose bool   `short:"v" long:"verbose"`
    Config  string `short:"c" long:"config"`
}

var opts Options

// Sift: extract known flags, pass through unknown
remaining, positional, err := argsieve.Sift(&opts, os.Args[1:], []string{"-o"}, nil)

// Parse: strict mode, error on unknown flags
positional, err := argsieve.Parse(&opts, os.Args[1:], nil)
```

## Configuration

Both `Sift` and `Parse` accept an optional `*Config` parameter (pass `nil` for defaults).

### RequirePositionalDelimiter

Require all positional arguments to appear after `--`:

```go
cfg := &argsieve.Config{RequirePositionalDelimiter: true}
positional, err := argsieve.Parse(&opts, os.Args[1:], cfg)

// "-v filename"     → error: positional before "--"
// "-v -- filename"  → OK: positional after delimiter
```

See [pkg.go.dev](https://pkg.go.dev/github.com/ivoronin/argsieve) for full API documentation.

## License

[GPL-3.0](LICENSE)
