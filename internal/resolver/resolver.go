package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"

	"github.com/kristersd/fast-tcm/internal/parser"
)

type FileReader func(path string) ([]byte, error)

type Resolver struct {
	read    FileReader
	cache   sync.Map // key string -> *parser.Tokens
	group   singleflight.Group
	rootDir string
}

func NewResolver(rootDir string, read FileReader) *Resolver {
	if read == nil {
		read = os.ReadFile
	}
	return &Resolver{
		read:    read,
		rootDir: rootDir,
	}
}

func (r *Resolver) Resolve(filePath string) ([]string, error) {
	return r.resolveWithVisiting(filePath, nil)
}

func (r *Resolver) resolveWithVisiting(filePath string, visiting []string) ([]string, error) {
	tokens, err := r.resolveFile(filePath)
	if err != nil {
		return nil, err
	}

	all := tokens.AllTokens()

	// Fast path: no imports means no cycle resolution needed
	if len(tokens.Imports) == 0 {
		return all, nil
	}

	// resolve @import token merging (same as upstream behavior)
	for _, imp := range tokens.Imports {
		resolvedPath := r.resolveImportPath(filePath, imp)
		absPath, _ := filepath.Abs(resolvedPath)

		// Cycle detection: linear scan over visiting slice (shallow chains, rare cycles)
		found := false
		for _, v := range visiting {
			if v == absPath {
				found = true
				break
			}
		}
		if found {
			fmt.Fprintf(os.Stderr, "[WARN] circular import detected: %s\n", absPath)
			continue
		}

		imported, err := r.resolveWithVisiting(resolvedPath, append(visiting, absPath))
		if err != nil {
			// upstream is forgiving on import resolution failures in some cases
			continue
		}
		all = append(all, imported...)
	}

	return parser.NormalizeTokens(all), nil
}

func (r *Resolver) resolveFile(filePath string) (*parser.Tokens, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	// Check cache first
	if cached, ok := r.cache.Load(absPath); ok {
		return cached.(*parser.Tokens), nil
	}

	// Use singleflight to ensure only one parse per file under race conditions.
	val, err, _ := r.group.Do(absPath, func() (interface{}, error) {
		// Double-check cache inside singleflight to avoid redundant work if another
		// caller already parsed and stored the result.
		if cached, ok := r.cache.Load(absPath); ok {
			return cached, nil
		}

		src, err := r.read(filePath)
		if err != nil {
			return nil, err
		}

		tokens, err := parser.ExtractTokens(src)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", filePath, err)
		}

		r.cache.Store(absPath, tokens)
		return tokens, nil
	})

	if err != nil {
		return nil, err
	}

	return val.(*parser.Tokens), nil
}

func (r *Resolver) resolveImportPath(fromFile, importPath string) string {
	if filepath.IsAbs(importPath) {
		return importPath
	}

	if !strings.HasPrefix(importPath, ".") {
		modPath := filepath.Join(r.rootDir, "node_modules", importPath)
		if _, err := os.Stat(modPath); err == nil {
			return modPath
		}
		return importPath
	}

	dir := filepath.Dir(fromFile)
	resolved := filepath.Join(dir, importPath)
	resolved, _ = filepath.Abs(resolved)
	return resolved
}
