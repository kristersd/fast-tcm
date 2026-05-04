package generate

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kristersd/fast-tcm/internal/parser"
)

type ExportType string

const (
	ExportCommonJS ExportType = "commonjs"
	ExportDefault  ExportType = "default"
	ExportNamed    ExportType = "named"
)

type Config struct {
	CamelCase                bool
	Dashes                   bool
	ExportType               ExportType
	AllowArbitraryExtensions bool
	DropExtension            bool
	EOL                      string
}

type Output struct {
	Tokens    []string
	Formatted string
}

// Generate produces TypeScript definitions from raw tokens.
// Tokens are assumed to already be normalized (deduplicated and sorted).
func Generate(tokens []string, cfg Config) (*Output, error) {
	converted := make([]string, 0, len(tokens))
	seen := make(map[string]struct{})

	for _, t := range tokens {
		key := parser.ConvertKey(t, cfg.CamelCase, cfg.Dashes)
		if cfg.ExportType == ExportNamed {
			// named exports require valid identifiers
			if !parser.IsValidIdentifier(key) {
				continue
			}
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		converted = append(converted, key)
	}

	var formatted string
	et := cfg.ExportType
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
	case ExportCommonJS:
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
	default:
		return nil, fmt.Errorf("unsupported export type: %s", et)
	}

	return &Output{
		Tokens:    converted,
		Formatted: formatted,
	}, nil
}

func OutputFileName(inputPath string, cfg Config) string {
	ext := filepath.Ext(inputPath)
	base := inputPath
	if cfg.DropExtension || cfg.AllowArbitraryExtensions {
		base = strings.TrimSuffix(inputPath, ext)
	}
	if cfg.AllowArbitraryExtensions {
		return base + ".d" + ext + ".ts"
	}
	return base + ".d.ts"
}
