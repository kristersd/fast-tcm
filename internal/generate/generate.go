package generate

import (
	"fmt"
	"path"
	"strings"

	"github.com/kristersd/fast-tcm/internal/parser"
)

// ExportType controls how declarations are exported.
type ExportType string

const (
	ExportCommonJS ExportType = "commonjs"
	ExportDefault  ExportType = "default"
	ExportNamed    ExportType = "named"
)

// Config holds generation options.
type Config struct {
	CamelCase                  bool
	Dashes                     bool
	NamedExports               bool
	ExportType                 ExportType
	AllowArbitraryExtensions   bool
	DropExtension              bool
	EOL                        string
}

// Output represents a generated .d.ts file.
type Output struct {
	Tokens           []string
	Formatted        string
	OutputFileName   string
}

// Generate produces TypeScript definitions from raw tokens.
func Generate(tokens []string, cfg Config) (*Output, error) {
	converted := make([]string, 0, len(tokens))
	seen := make(map[string]bool)

	for _, t := range tokens {
		key := parser.ConvertKey(t, cfg.CamelCase, cfg.Dashes)
		if cfg.NamedExports || cfg.ExportType == ExportNamed {
			// named exports require valid identifiers
			if !parser.IsValidIdentifier(key) {
				continue
			}
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		converted = append(converted, key)
	}

	converted = parser.NormalizeTokens(converted)

	var formatted string
	et := cfg.ExportType
	if cfg.NamedExports && et == "" {
		et = ExportNamed
	}
	if et == "" {
		et = ExportCommonJS
	}

	switch et {
	case ExportNamed:
		if len(converted) == 0 {
			formatted = "export {};"
		} else {
			lines := []string{"export const __esModule: true;"}
			for _, k := range converted {
				lines = append(lines, fmt.Sprintf("export const %s: string;", k))
			}
			formatted = strings.Join(lines, cfg.EOL) + cfg.EOL
		}
	case ExportDefault:
		if len(converted) == 0 {
			formatted = "export {};"
		} else {
			lines := []string{"declare const styles: {"}
			for _, k := range converted {
				lines = append(lines, fmt.Sprintf(`  readonly "%s": string;`, k))
			}
			lines = append(lines, "};")
			lines = append(lines, "export default styles;")
			lines = append(lines, "")
			formatted = strings.Join(lines, cfg.EOL) + cfg.EOL
		}
	default: // commonjs
		if len(converted) == 0 {
			formatted = "export {};"
		} else {
			lines := []string{"declare const styles: {"}
			for _, k := range converted {
				lines = append(lines, fmt.Sprintf(`  readonly "%s": string;`, k))
			}
			lines = append(lines, "};")
			lines = append(lines, "export = styles;")
			lines = append(lines, "")
			formatted = strings.Join(lines, cfg.EOL) + cfg.EOL
		}
	}

	return &Output{
		Tokens:    converted,
		Formatted: formatted,
	}, nil
}

// OutputFileName computes the output filename.
func OutputFileName(inputPath string, cfg Config) string {
	ext := path.Ext(inputPath)
	base := inputPath
	if cfg.DropExtension || cfg.AllowArbitraryExtensions {
		base = strings.TrimSuffix(inputPath, ext)
	}
	if cfg.AllowArbitraryExtensions {
		return base + ".d" + ext + ".ts"
	}
	return base + ".d.ts"
}
