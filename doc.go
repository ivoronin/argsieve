// Package argsieve provides argument parsing with known/unknown flag separation.
//
// argsieve is designed for CLI wrapper applications that need to intercept
// some flags while passing others through to an underlying command.
//
// # Two Parsing Modes
//
// [Sift] extracts known flags and passes unknown flags through - ideal for
// CLI wrappers that need to handle some flags while forwarding others to
// an underlying command.
//
// [Parse] is strict mode that errors on any unknown flag - use this when
// building standalone CLI tools.
//
// # Configuration
//
// Both [Sift] and [Parse] accept an optional [Config] parameter. Pass nil
// to use defaults.
//
// Use [Config.RequirePositionalDelimiter] to require that all positional
// arguments appear after the "--" delimiter:
//
//	cfg := &argsieve.Config{RequirePositionalDelimiter: true}
//	positional, err := argsieve.Parse(&opts, args, cfg)
//	// "-v filename" → error: positional before "--"
//	// "-v -- filename" → OK: positional after delimiter
//
// # Struct Tags
//
// Define flags using struct tags:
//
//	type Options struct {
//	    Region  string `short:"r" long:"region"`
//	    Verbose bool   `short:"v" long:"verbose"`
//	}
//
// # Supported Flag Formats
//
// Short flags: -v, -r value, -rvalue, -vdr (chained bools)
// Long flags: --verbose, --region value, --region=value
// Terminator: -- (everything after is positional)
//
// # Supported Field Types
//
//   - bool: flag presence sets true (no value required)
//   - string: requires a value
//   - [encoding.TextUnmarshaler]: custom parsing via UnmarshalText
//   - *T where T implements TextUnmarshaler: nil when absent, allocated when present
package argsieve
