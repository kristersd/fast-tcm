package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/kristersd/fast-tcm/internal/generate"
	"github.com/kristersd/fast-tcm/internal/resolver"
)

// Options holds CLI options.
type Options struct {
	Pattern                  string
	OutDir                   string
	Watch                    bool
	CamelCase                bool
	NamedExports             bool
	ExportType               string
	AllowArbitraryExtensions bool
	DropExtension            bool
	Silent                   bool
	ListDifferent            bool
}

// Run executes the generation process.
func Run(searchDir string, opts Options) error {
	pattern := opts.Pattern
	if pattern == "" {
		pattern = filepath.ToSlash(filepath.Join(searchDir, "**/*.{css,pcss}"))
	} else if !filepath.IsAbs(pattern) {
		pattern = filepath.ToSlash(filepath.Join(searchDir, pattern))
	}

	baseDir := "."
	if opts.OutDir != "" {
		baseDir = opts.OutDir
	}

	files, err := doublestar.FilepathGlob(pattern)
	if err != nil {
		return fmt.Errorf("glob: %w", err)
	}

	if len(files) == 0 && !opts.Watch {
		return nil
	}

	res := resolver.NewResolver(searchDir, nil)
	cfg := generate.Config{
		CamelCase:                opts.CamelCase,
		AllowArbitraryExtensions: opts.AllowArbitraryExtensions,
		DropExtension:            opts.DropExtension,
		EOL:                      "\n",
	}

	if opts.NamedExports {
		cfg.ExportType = generate.ExportNamed
		cfg.CamelCase = true
	} else {
		cfg.ExportType = generate.ExportType(opts.ExportType)
	}

	if opts.CamelCase {
		cfg.CamelCase = true
	}

	if opts.ListDifferent {
		var hasDiff bool
		for _, f := range files {
			if err := processFile(f, searchDir, res, cfg, baseDir, opts, true); err != nil {
				if !opts.Silent {
					fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
				}
				hasDiff = true
			}
		}
		if hasDiff {
			os.Exit(1)
		}
		return nil
	}

	if !opts.Watch {
		for _, f := range files {
			if err := processFile(f, searchDir, res, cfg, baseDir, opts, false); err != nil {
				if !opts.Silent {
					fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
				}
			}
		}
		return nil
	}

	// Watch mode would use fsnotify; for now, stub
	fmt.Println("Watch mode not yet implemented")
	return nil
}

func processFile(filePath string, searchDir string, res *resolver.Resolver, cfg generate.Config, baseDir string, opts Options, checkOnly bool) error {
	tokens, err := res.Resolve(filePath)
	if err != nil {
		return err
	}

	out, err := generate.Generate(tokens, cfg)
	if err != nil {
		return err
	}

	rel, _ := filepath.Rel(searchDir, filePath)
	if rel == "" || strings.HasPrefix(rel, "..") {
		rel = filePath
	}
	outName := generate.OutputFileName(rel, cfg)
	if baseDir != "." {
		outName = filepath.Join(baseDir, outName)
	} else {
		outName = filepath.Join(filepath.Dir(filePath), filepath.Base(outName))
	}

	if checkOnly {
		existing, err := os.ReadFile(outName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Type file needs to be generated for '%s'\n", filePath)
			return fmt.Errorf("missing %s", outName)
		}
		if strings.TrimSpace(string(existing)) != strings.TrimSpace(out.Formatted) {
			fmt.Fprintf(os.Stderr, "[ERROR] Check type definitions for '%s'\n", outName)
			return fmt.Errorf("different %s", outName)
		}
		return nil
	}

	dir := filepath.Dir(outName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(outName, []byte(out.Formatted), 0644); err != nil {
		return err
	}

	if !opts.Silent {
		fmt.Println("Wrote " + outName)
	}
	return nil
}
