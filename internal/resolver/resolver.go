package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kristersd/fast-tcm/internal/parser"
)

// FileReader reads file contents by path.
type FileReader func(path string) ([]byte, error)

// Resolver resolves CSS Modules dependencies and collects all exported tokens.
type Resolver struct {
	read      FileReader
	cache     map[string]*parser.Tokens
	rootDir   string
}

// NewResolver creates a new resolver.
func NewResolver(rootDir string, read FileReader) *Resolver {
	if read == nil {
		read = os.ReadFile
	}
	return &Resolver{
		read:    read,
		cache:   make(map[string]*parser.Tokens),
		rootDir: rootDir,
	}
}

// ClearCache clears the parse cache.
func (r *Resolver) ClearCache() {
	r.cache = make(map[string]*parser.Tokens)
}

// Resolve parses a CSS file and resolves all composes/import dependencies.
func (r *Resolver) Resolve(filePath string) ([]string, error) {
	tokens, err := r.resolveFile(filePath)
	if err != nil {
		return nil, err
	}

	all := tokens.AllTokens()

	// resolve @import token merging (same as upstream behavior)
	for _, imp := range tokens.Imports {
		resolvedPath := r.resolveImportPath(filePath, imp)
		imported, err := r.Resolve(resolvedPath)
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

	if cached, ok := r.cache[absPath]; ok {
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

	r.cache[absPath] = tokens
	return tokens, nil
}

func (r *Resolver) resolveImportPath(fromFile, importPath string) string {
	if filepath.IsAbs(importPath) {
		return importPath
	}

	// handle node_modules style imports
	if !strings.HasPrefix(importPath, ".") {
		// try to resolve from rootDir/node_modules
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
