package argsieve

import (
	"encoding"
	"errors"
	"fmt"
	"iter"
	"reflect"
	"slices"
	"strings"
)

// ErrParse indicates a parsing error such as a missing value for a flag
// that requires one, or (in strict mode) an unknown flag.
//
// Use [errors.Is] to check for parsing errors:
//
//	if errors.Is(err, argsieve.ErrParse) {
//	    // Handle parsing error
//	}
var ErrParse = errors.New("argument parsing error")

// textUnmarshalerType is used to check if a type implements encoding.TextUnmarshaler.
var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

// Config holds optional settings for argument parsing.
// Pass nil to use defaults.
type Config struct {
	// RequirePositionalDelimiter when true requires all positional arguments
	// to appear after the "--" delimiter. Positional arguments before "--"
	// will cause a parse error.
	RequirePositionalDelimiter bool

	// StopAtFirstPositional when true stops flag parsing at the first
	// positional argument. All subsequent arguments are treated as positional,
	// even if they look like flags.
	StopAtFirstPositional bool
}

// fieldInfo holds a reference to a struct field and whether it needs an argument.
type fieldInfo struct {
	field    reflect.Value
	needsArg bool
	isPtr    bool // true if field is a pointer to TextUnmarshaler
}

// sieve separates known flags from unknown flags and positional arguments.
type sieve struct {
	fields                     map[string]fieldInfo // flag name → field info
	passthrough                map[string]struct{}
	remaining                  []string
	positional                 []string
	strict                     bool
	requirePositionalDelimiter bool
	stopAtFirstPositional      bool
	delimiterSeen              bool
}

// Sift extracts known flags from args into target, returning unknown flags
// and positional arguments separately.
//
// This is the primary function for CLI wrapper applications. Known flags
// (those matching struct tags) are parsed into target. Unknown flags are
// returned in remaining, allowing you to forward them to another command.
//
// The passthroughWithArg parameter lists unknown flags that consume a value.
// Without this hint, an unknown flag's value would be treated as positional.
//
// The cfg parameter allows optional configuration. Pass nil to use defaults.
// When cfg.RequirePositionalDelimiter is true, positional arguments must
// appear after the "--" delimiter or an error is returned.
//
// Example:
//
//	type Options struct {
//	    Config string `short:"c" long:"config"`
//	    Debug  bool   `short:"d"`
//	}
//	var opts Options
//	remaining, positional, err := argsieve.Sift(&opts, os.Args[1:], []string{"-x"}, nil)
//	// opts.Config contains the parsed value
//	// remaining holds unknown flags like ["-x", "value"]
//	// positional holds non-flag arguments
//
// Panics if target is not a pointer to struct or if any tagged field
// has an unsupported type.
func Sift(target any, args []string, passthroughWithArg []string, cfg *Config) (remaining, positional []string, err error) {
	s := &sieve{
		fields:      make(map[string]fieldInfo),
		passthrough: make(map[string]struct{}),
	}

	if cfg != nil {
		s.requirePositionalDelimiter = cfg.RequirePositionalDelimiter
		s.stopAtFirstPositional = cfg.StopAtFirstPositional
	}

	s.extractFields(target)

	for _, p := range passthroughWithArg {
		s.passthrough[p] = struct{}{}
	}

	return s.parse(args)
}

// Parse parses args into target in strict mode, returning only positional arguments.
//
// Unlike [Sift], Parse returns an error if any unknown flags are encountered.
// Use this for standalone CLI tools where all flags should be defined.
//
// The cfg parameter allows optional configuration. Pass nil to use defaults.
// When cfg.RequirePositionalDelimiter is true, positional arguments must
// appear after the "--" delimiter or an error is returned.
//
// Example:
//
//	type Options struct {
//	    Output  string `short:"o" long:"output"`
//	    Verbose bool   `short:"v"`
//	}
//	var opts Options
//	positional, err := argsieve.Parse(&opts, os.Args[1:], nil)
//	if errors.Is(err, argsieve.ErrParse) {
//	    // Handle unknown flag or missing value
//	}
//
// Panics if target is not a pointer to struct or if any tagged field
// has an unsupported type.
func Parse(target any, args []string, cfg *Config) (positional []string, err error) {
	s := &sieve{
		fields:      make(map[string]fieldInfo),
		passthrough: make(map[string]struct{}),
		strict:      true,
	}

	if cfg != nil {
		s.requirePositionalDelimiter = cfg.RequirePositionalDelimiter
		s.stopAtFirstPositional = cfg.StopAtFirstPositional
	}

	s.extractFields(target)

	_, positional, err = s.parse(args)

	return positional, err
}

// Helper methods for cleaner append patterns.
func (s *sieve) addRemaining(args ...string)  { s.remaining = append(s.remaining, args...) }
func (s *sieve) addPositional(args ...string) { s.positional = append(s.positional, args...) }

// extractFields reads struct tags and stores field references.
// Panics if target is not a pointer to a struct.
func (s *sieve) extractFields(target any) {
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		panic(fmt.Sprintf("argsieve: target must be a pointer to struct, got %T", target))
	}

	s.extractFieldsFromValue(v.Elem())
}

// extractFieldsFromValue recursively extracts fields from a struct value,
// including fields from embedded structs.
func (s *sieve) extractFieldsFromValue(v reflect.Value) {
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		fieldType := t.Field(i)
		fieldValue := v.Field(i)

		// Recursively process embedded structs
		if fieldType.Anonymous && fieldType.Type.Kind() == reflect.Struct {
			s.extractFieldsFromValue(fieldValue)
			continue
		}

		short := fieldType.Tag.Get("short")
		long := fieldType.Tag.Get("long")

		// Skip fields without tags
		if short == "" && long == "" {
			continue
		}

		// Determine field type and whether it needs an argument
		kind := fieldType.Type.Kind()
		var info fieldInfo

		switch {
		case kind == reflect.Bool:
			info = fieldInfo{field: fieldValue, needsArg: false}
		case kind == reflect.String:
			info = fieldInfo{field: fieldValue, needsArg: true}
		case kind == reflect.Ptr:
			// Pointer to TextUnmarshaler - nil when flag absent, allocated when present
			elemType := fieldType.Type.Elem()
			if reflect.PointerTo(elemType).Implements(textUnmarshalerType) {
				info = fieldInfo{field: fieldValue, needsArg: true, isPtr: true}
			} else {
				panic(fmt.Sprintf("argsieve: pointer field %s must point to type implementing encoding.TextUnmarshaler",
					fieldType.Name))
			}
		case fieldValue.CanAddr() && reflect.PointerTo(fieldType.Type).Implements(textUnmarshalerType):
			// Field's pointer type implements encoding.TextUnmarshaler
			info = fieldInfo{field: fieldValue, needsArg: true}
		default:
			panic(fmt.Sprintf("argsieve: field %s has unsupported type %s (must be string, bool, or implement encoding.TextUnmarshaler)",
				fieldType.Name, fieldType.Type))
		}

		if short != "" {
			s.fields[short] = info
		}

		if long != "" {
			s.fields[long] = info
		}
	}
}

// setField assigns a value to a field based on its type.
// Returns an error if TextUnmarshaler.UnmarshalText fails.
func (s *sieve) setField(info fieldInfo, value string) error {
	// Handle pointer fields - allocate and set
	if info.isPtr {
		elemType := info.field.Type().Elem()
		newVal := reflect.New(elemType)
		if tu, ok := newVal.Interface().(encoding.TextUnmarshaler); ok {
			if err := tu.UnmarshalText([]byte(value)); err != nil {
				return err
			}
			info.field.Set(newVal)
			return nil
		}
	}

	// Try TextUnmarshaler for value types
	if info.field.CanAddr() {
		if tu, ok := info.field.Addr().Interface().(encoding.TextUnmarshaler); ok {
			return tu.UnmarshalText([]byte(value))
		}
	}

	// Fall back to built-in types
	if info.needsArg {
		info.field.SetString(value)
	} else {
		info.field.SetBool(true)
	}

	return nil
}

// handleLong processes --name or --name=value arguments.
func (s *sieve) handleLong(arg string, next func() (string, bool)) error {
	name, eqValue, hasEquals := strings.Cut(arg[2:], "=")

	info, known := s.fields[name]

	// Unknown flag - reject in strict mode or check passthrough list
	if !known {
		if s.strict {
			return fmt.Errorf("%w: unknown option --%s", ErrParse, name)
		}

		_, isPassthrough := s.passthrough["--"+name]

		if isPassthrough && !hasEquals {
			if value, ok := next(); ok {
				s.addRemaining(arg, value)

				return nil
			}
		}

		s.addRemaining(arg)

		return nil
	}

	// Known bool flag
	if !info.needsArg {
		return s.setField(info, "")
	}

	// Known string flag with equals
	if hasEquals {
		if err := s.setField(info, eqValue); err != nil {
			return fmt.Errorf("%w: invalid value for --%s: %v", ErrParse, name, err)
		}

		return nil
	}

	// Known string flag - needs argument from next arg
	value, ok := next()
	if !ok {
		return fmt.Errorf("%w: missing value for --%s", ErrParse, name)
	}

	if err := s.setField(info, value); err != nil {
		return fmt.Errorf("%w: invalid value for --%s: %v", ErrParse, name, err)
	}

	return nil
}

// handleShort processes -x, -xvalue, or -xyz combined arguments.
func (s *sieve) handleShort(arg string, next func() (string, bool)) error {
	flags := arg[1:]

	for j := 0; j < len(flags); j++ {
		flag := string(flags[j])
		tail := flags[j+1:]

		info, known := s.fields[flag]

		// Handle unknown flag first (guard clause)
		if !known {
			if err := s.handleUnknownShort(flag, tail, next); err != nil {
				return err
			}

			if len(tail) > 0 {
				return nil // tail consumed by passthrough
			}

			continue
		}

		// Known bool flag
		if !info.needsArg {
			if err := s.setField(info, ""); err != nil {
				return err
			}

			continue
		}

		// Known string flag - value attached
		if len(tail) > 0 {
			if err := s.setField(info, tail); err != nil {
				return fmt.Errorf("%w: invalid value for -%s: %v", ErrParse, flag, err)
			}

			return nil
		}

		// Known string flag - value in next arg
		value, ok := next()
		if !ok {
			return fmt.Errorf("%w: missing value for -%s", ErrParse, flag)
		}

		if err := s.setField(info, value); err != nil {
			return fmt.Errorf("%w: invalid value for -%s: %v", ErrParse, flag, err)
		}

		return nil
	}

	return nil
}

// handleUnknownShort handles unknown short flags, checking passthrough list.
func (s *sieve) handleUnknownShort(flag, tail string, next func() (string, bool)) error {
	if s.strict {
		return fmt.Errorf("%w: unknown option -%s", ErrParse, flag)
	}

	prefixedFlag := "-" + flag
	_, isPassthrough := s.passthrough[prefixedFlag]

	if isPassthrough {
		if len(tail) > 0 {
			s.addRemaining("-" + flag + tail)

			return nil
		}

		if value, ok := next(); ok {
			s.addRemaining(prefixedFlag, value)

			return nil
		}
	}

	s.addRemaining(prefixedFlag)

	return nil
}

// parse separates args into known flags (bound to target), unknown flags, and positionals.
// Arguments after "--" are treated as positional (the "--" itself is not included).
func (s *sieve) parse(args []string) (remaining, positional []string, err error) {
	next, stop := iter.Pull(slices.Values(args))
	defer stop()

	for arg, ok := next(); ok; arg, ok = next() {
		switch {
		case arg == "--":
			s.delimiterSeen = true
			// Drain remaining args as positional (don't pass "--" through)
			for arg, ok := next(); ok; arg, ok = next() {
				s.addPositional(arg)
			}

		case strings.HasPrefix(arg, "--"):
			if err := s.handleLong(arg, next); err != nil {
				return nil, nil, err
			}

		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			if err := s.handleShort(arg, next); err != nil {
				return nil, nil, err
			}

		default:
			if s.requirePositionalDelimiter && !s.delimiterSeen {
				return nil, nil, fmt.Errorf("%w: positional argument %q not allowed before \"--\" delimiter", ErrParse, arg)
			}
			s.addPositional(arg)
			if s.stopAtFirstPositional {
				// Drain remaining args as positional
				for arg, ok := next(); ok; arg, ok = next() {
					s.addPositional(arg)
				}
			}
		}
	}

	return s.remaining, s.positional, nil
}
