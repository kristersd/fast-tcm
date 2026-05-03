package run

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"golang.org/x/sync/errgroup"

	"github.com/kristersd/fast-tcm/internal/generate"
	"github.com/kristersd/fast-tcm/internal/resolver"
)

// ErrDifferent is returned by Run when --listDifferent finds differences.
var ErrDifferent = errors.New("some files are different or missing")

// Options holds CLI options.
type Options struct {
	Pattern                  string
	OutDir                   string
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

	files, err := collectFiles(searchDir, pattern)
	if err != nil {
		return fmt.Errorf("glob: %w", err)
	}

	if len(files) == 0 {
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

	// Use errgroup with bounded concurrency
	g := new(errgroup.Group)
	g.SetLimit(32) // reasonable default limit

	if opts.ListDifferent {
		for _, f := range files {
			f := f // capture loop variable
			g.Go(func() error {
				return checkFile(f, searchDir, res, cfg, baseDir, opts)
			})
		}
	} else {
		for _, f := range files {
			f := f // capture loop variable
			g.Go(func() error {
				return writeFile(f, searchDir, res, cfg, baseDir, opts)
			})
		}
	}

	err = g.Wait()
	if err != nil {
		// Check if it's our sentinel error
		if errors.Is(err, ErrDifferent) {
			return ErrDifferent
		}
		return err
	}

	return nil
}

func collectFiles(searchDir string, pattern string) ([]string, error) {
	// If searchDir is a single file, use it directly
	info, err := os.Stat(searchDir)
	if err == nil && !info.IsDir() {
		return []string{searchDir}, nil
	}

	var files []string

	err = filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Skip symlinks entirely (both files and directories)
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden directories (but not the search root itself)
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != searchDir {
			return filepath.SkipDir
		}

		// Skip directories (continue walking)
		if d.IsDir() {
			return nil
		}

		// Match file against pattern
		matched, err := doublestar.Match(pattern, filepath.ToSlash(path))
		if err == nil && matched {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

func checkFile(filePath string, searchDir string, res *resolver.Resolver, cfg generate.Config, baseDir string, opts Options) error {
	tokens, err := res.Resolve(filePath)
	if err != nil {
		return err
	}

	out, err := generate.Generate(tokens, cfg)
	if err != nil {
		return err
	}

	outName := computeOutputPath(filePath, searchDir, baseDir, cfg)

	existing, err := os.ReadFile(outName)
	if err != nil {
		if !opts.Silent {
			fmt.Fprintf(os.Stderr, "[ERROR] Type file needs to be generated for '%s'\n", filePath)
		}
		return ErrDifferent
	}
	if strings.TrimSpace(string(existing)) != strings.TrimSpace(out.Formatted) {
		if !opts.Silent {
			fmt.Fprintf(os.Stderr, "[ERROR] Check type definitions for '%s'\n", outName)
		}
		return ErrDifferent
	}
	return nil
}

func writeFile(filePath string, searchDir string, res *resolver.Resolver, cfg generate.Config, baseDir string, opts Options) error {
	tokens, err := res.Resolve(filePath)
	if err != nil {
		return err
	}

	out, err := generate.Generate(tokens, cfg)
	if err != nil {
		return err
	}

	outName := computeOutputPath(filePath, searchDir, baseDir, cfg)

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

func computeOutputPath(filePath, searchDir, baseDir string, cfg generate.Config) string {
	rel, _ := filepath.Rel(searchDir, filePath)
	if rel == "" || strings.HasPrefix(rel, "..") {
		rel = filePath
	}
	// If searchDir was a single file, use just the basename
	if filePath == searchDir {
		rel = filepath.Base(filePath)
	}
	outName := generate.OutputFileName(rel, cfg)
	if baseDir != "." {
		outName = filepath.Join(baseDir, outName)
	} else {
		outName = filepath.Join(filepath.Dir(filePath), filepath.Base(outName))
	}
	return outName
}
