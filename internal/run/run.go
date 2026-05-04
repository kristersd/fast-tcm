package run

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
	"golang.org/x/sync/errgroup"

	"github.com/kristersd/fast-tcm/internal/generate"
	"github.com/kristersd/fast-tcm/internal/resolver"
)

var ErrDifferent = errors.New("some files are different or missing")

type Options struct {
	Pattern                  string
	OutDir                   string
	CamelCase                bool
	ExportType               string
	AllowArbitraryExtensions bool
	DropExtension            bool
	Silent                   bool
	ListDifferent            bool
}

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
		ExportType:               generate.ExportType(opts.ExportType),
	}

	var createdDirs sync.Map

	// We spawn more workers than CPUs as most of the time is spent on syscalls (read/write)
	numWorkers := max(32, min(runtime.NumCPU()*4, 256))

	g := new(errgroup.Group)
	g.SetLimit(numWorkers)

	var op func(string) error
	if opts.ListDifferent {
		op = func(f string) error {
			return checkFile(f, searchDir, res, cfg, baseDir, opts)
		}
	} else {
		op = func(f string) error {
			return writeFile(f, searchDir, res, cfg, baseDir, opts, &createdDirs)
		}
	}

	for _, batch := range chunkFiles(files, numWorkers) {
		g.Go(func() error {
			for _, f := range batch {
				if err := op(f); err != nil {
					return err
				}
			}
			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		if errors.Is(err, ErrDifferent) {
			return ErrDifferent
		}
		return err
	}

	return nil
}

// chunkFiles splits a slice into chunks of at most chunkSize.
func chunkFiles(files []string, chunkSize int) [][]string {
	if chunkSize <= 0 {
		chunkSize = 50
	}
	var chunks [][]string
	for i := 0; i < len(files); i += chunkSize {
		end := min(i+chunkSize, len(files))
		chunks = append(chunks, files[i:end])
	}
	return chunks
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

func writeFile(filePath string, searchDir string, res *resolver.Resolver, cfg generate.Config, baseDir string, opts Options, createdDirs *sync.Map) error {
	tokens, err := res.Resolve(filePath)
	if err != nil {
		return err
	}

	out, err := generate.Generate(tokens, cfg)
	if err != nil {
		return err
	}

	outName := computeOutputPath(filePath, searchDir, baseDir, cfg)

	// Skip writing if the file already exists with identical content.
	// This avoids unnecessary open/write/close syscalls on incremental runs.
	if existing, err := os.ReadFile(outName); err == nil {
		if strings.TrimSpace(string(existing)) == strings.TrimSpace(out.Formatted) {
			return nil
		}
	}

	dir := filepath.Dir(outName)
	if _, ok := createdDirs.Load(dir); !ok {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		createdDirs.Store(dir, struct{}{})
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
