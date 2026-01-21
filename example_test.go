package argsieve_test

import (
	"errors"
	"fmt"

	"github.com/ivoronin/argsieve"
)

func Example() {
	type Options struct {
		Config  string `short:"c" long:"config"`
		Verbose bool   `short:"v" long:"verbose"`
	}

	var opts Options
	args := []string{"-v", "--config", "app.yaml", "file1.txt", "file2.txt"}

	remaining, positional, err := argsieve.Sift(&opts, args, nil, nil)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Config: %s\n", opts.Config)
	fmt.Printf("Verbose: %t\n", opts.Verbose)
	fmt.Printf("Remaining: %v\n", remaining)
	fmt.Printf("Positional: %v\n", positional)
	// Output:
	// Config: app.yaml
	// Verbose: true
	// Remaining: []
	// Positional: [file1.txt file2.txt]
}

func ExampleSift() {
	type Options struct {
		Config string `short:"c" long:"config"`
	}

	var opts Options
	args := []string{"-c", "app.yaml", "-x", "extra-value", "target"}

	// -x takes a value, so list it in passthroughWithArg
	remaining, positional, _ := argsieve.Sift(&opts, args, []string{"-x"}, nil)

	fmt.Printf("Config: %s\n", opts.Config)
	fmt.Printf("Passthrough: %v\n", remaining)
	fmt.Printf("Positional: %v\n", positional)
	// Output:
	// Config: app.yaml
	// Passthrough: [-x extra-value]
	// Positional: [target]
}

func ExampleSift_passthrough() {
	type Options struct {
		Debug bool `short:"d" long:"debug"`
	}

	var opts Options
	args := []string{"-d", "-L", "8080:localhost:80", "--unknown", "value", "target"}

	// List flags that consume values so they're captured correctly
	remaining, positional, _ := argsieve.Sift(&opts, args, []string{"-L", "--unknown"}, nil)

	fmt.Printf("Debug: %t\n", opts.Debug)
	fmt.Printf("Passthrough: %v\n", remaining)
	fmt.Printf("Positional: %v\n", positional)
	// Output:
	// Debug: true
	// Passthrough: [-L 8080:localhost:80 --unknown value]
	// Positional: [target]
}

func ExampleParse() {
	type Options struct {
		Output string `short:"o" long:"output"`
		Force  bool   `short:"f" long:"force"`
	}

	var opts Options
	args := []string{"-f", "--output", "result.txt", "input.txt"}

	positional, err := argsieve.Parse(&opts, args, nil)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Output: %s\n", opts.Output)
	fmt.Printf("Force: %t\n", opts.Force)
	fmt.Printf("Files: %v\n", positional)
	// Output:
	// Output: result.txt
	// Force: true
	// Files: [input.txt]
}

func ExampleParse_errorHandling() {
	type Options struct {
		Output string `short:"o" long:"output"`
	}

	var opts Options
	args := []string{"--unknown-flag"}

	_, err := argsieve.Parse(&opts, args, nil)
	if errors.Is(err, argsieve.ErrParse) {
		fmt.Println("Parse error:", err)
	}
	// Output:
	// Parse error: argument parsing error: unknown option --unknown-flag
}

// LogLevel demonstrates encoding.TextUnmarshaler for custom flag types.
type LogLevel string

func (l *LogLevel) UnmarshalText(text []byte) error {
	switch string(text) {
	case "info", "debug", "error":
		*l = LogLevel(text)
	default:
		return fmt.Errorf("unknown log level: %q", text)
	}
	return nil
}

func ExampleParse_textUnmarshaler() {
	type Options struct {
		Level   LogLevel `short:"l" long:"level"`
		Verbose bool     `short:"v"`
	}

	var opts Options
	args := []string{"-v", "--level", "debug"}

	_, err := argsieve.Parse(&opts, args, nil)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Level: %s\n", opts.Level)
	fmt.Printf("Verbose: %t\n", opts.Verbose)
	// Output:
	// Level: debug
	// Verbose: true
}

func ExampleSift_chainedFlags() {
	type Options struct {
		Verbose bool   `short:"v"`
		Debug   bool   `short:"d"`
		Level   string `short:"l"`
	}

	var opts Options
	// -vdl combines -v, -d, and -l with attached value
	args := []string{"-vdlinfo"}

	if _, _, err := argsieve.Sift(&opts, args, nil, nil); err != nil {
		panic(err)
	}

	fmt.Printf("Verbose: %t, Debug: %t, Level: %s\n", opts.Verbose, opts.Debug, opts.Level)
	// Output:
	// Verbose: true, Debug: true, Level: info
}

func ExampleConfig_requirePositionalDelimiter() {
	type Options struct {
		Verbose bool `short:"v" long:"verbose"`
	}

	var opts Options
	// With RequirePositionalDelimiter, positionals must come after "--"
	args := []string{"-v", "--", "file1.txt", "file2.txt"}

	cfg := &argsieve.Config{RequirePositionalDelimiter: true}
	positional, err := argsieve.Parse(&opts, args, cfg)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Verbose: %t\n", opts.Verbose)
	fmt.Printf("Files: %v\n", positional)
	// Output:
	// Verbose: true
	// Files: [file1.txt file2.txt]
}
