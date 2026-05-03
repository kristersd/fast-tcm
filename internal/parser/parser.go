package parser

import (
	"bytes"
	"io"
	"slices"
	"strings"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

// Tokens holds extracted identifiers from a CSS Modules file.
type Tokens struct {
	Classes   []string
	Keyframes []string
	Values    []string
	Exports   []string
	Composes  []Compose
	Imports   []string // @import paths
}

// Compose represents a composes: ... from "..." declaration.
type Compose struct {
	Classes []string
	From    string
}

// Sentinel byte slices for hot-path token comparisons (zero-allocation).
var (
	bImport        = []byte("@import")
	bKeyframes     = []byte("@keyframes")
	bValue         = []byte("@value")
	bComposes      = []byte("composes")
	bFrom          = []byte("from")
	bAnimation     = []byte("animation")
	bAnimationName = []byte("animation-name")
	bGlobal        = []byte("global(")
	bExport        = []byte("export")
	bGlobalPseudo  = []byte(":global")
)

// knownKeywords is a set of CSS keywords that should not be treated as animation names.
var knownKeywords = map[string]bool{
	"none": true, "initial": true, "inherit": true, "unset": true,
	"revert": true, "revert-layer": true,
	"linear": true, "ease": true, "ease-in": true, "ease-out": true,
	"ease-in-out": true, "step-start": true, "step-end": true,
	"normal": true, "reverse": true, "alternate": true, "alternate-reverse": true,
	"forwards": true, "backwards": true, "both": true,
	"running": true, "paused": true,
	"infinite": true,
	"auto": true, "default": true,
}

// ExtractTokens parses CSS/PCSS content and extracts CSS Modules tokens.
func ExtractTokens(src []byte) (*Tokens, error) {
	l := css.NewLexer(parse.NewInputBytes(src))
	t := &Tokens{}

	var prevTyp css.TokenType

	for {
		typ, text := l.Next()
		if typ == css.ErrorToken {
			if l.Err() != nil && l.Err() != io.EOF {
				return nil, l.Err()
			}
			break
		}

		switch typ {
		case css.AtKeywordToken:
			if bytes.Equal(text, bImport) {
				err := handleImport(l, t)
				if err != nil {
					return nil, err
				}
			} else {
				err := handleAtRule(l, text, t)
				if err != nil {
					return nil, err
				}
			}

		case css.IdentToken:
			if bytes.Equal(text, bExport) && prevTyp == css.ColonToken {
				err := handleExportBlock(l, t)
				if err != nil {
					return nil, err
				}
			}
		case css.FunctionToken:
			if bytes.Equal(text, bGlobal) {
				// skip until matching )
				for {
					t2, _ := l.Next()
					if t2 == css.ErrorToken || t2 == css.RightParenthesisToken {
						break
					}
				}
			}
		case css.DelimToken:
			if len(text) > 0 && text[0] == '.' {
				className, err := readClassName(l)
				if err != nil {
					return nil, err
				}
				if className != "" {
					t.Classes = append(t.Classes, className)
					extractComposesFromCurrentRule(l, t, className)
				}
			}
		}

		prevTyp = typ
	}

	return t, nil
}

func extractComposesFromCurrentRule(l *css.Lexer, t *Tokens, className string) {
	// Find the opening { of this rule, and collect any additional class selectors
	for {
		typ, text := l.Next()
		if typ == css.ErrorToken {
			return
		}
		if typ == css.FunctionToken && bytes.Equal(text, bGlobal) {
			// skip until matching )
			for {
				t2, _ := l.Next()
				if t2 == css.ErrorToken || t2 == css.RightParenthesisToken {
					break
				}
			}
			continue
		}
		if typ == css.DelimToken && len(text) > 0 && text[0] == '.' {
			// Found another class selector, extract it
			className2, err := readClassName(l)
			if err == nil && className2 != "" {
				t.Classes = append(t.Classes, className2)
			}
			continue
		}
		if typ == css.LeftBraceToken {
			break
		}
		if typ == css.SemicolonToken || typ == css.RightBraceToken {
			return
		}
	}

	blockDepth := 1
	for {
		typ, text := l.Next()
		if typ == css.ErrorToken {
			return
		}
		if typ == css.LeftBraceToken {
			blockDepth++
			continue
		}
		if typ == css.RightBraceToken {
			blockDepth--
			if blockDepth == 0 {
				return
			}
			continue
		}
		// Skip global() blocks at any depth
		if typ == css.FunctionToken && bytes.Equal(text, bGlobal) {
			for {
				t2, _ := l.Next()
				if t2 == css.ErrorToken || t2 == css.RightParenthesisToken {
					break
				}
			}
			continue
		}

		// Extract class names from selectors at any depth
		if typ == css.DelimToken && len(text) > 0 && text[0] == '.' {
			className2, err := readClassName(l)
			if err == nil && className2 != "" {
				t.Classes = append(t.Classes, className2)
			}
			continue
		}

		// Only look for composes at the top level of the current block
		if blockDepth == 1 && typ == css.IdentToken && bytes.Equal(text, bComposes) {
			// expect :
			t2, _ := l.Next()
			for t2 == css.WhitespaceToken {
				t2, _ = l.Next()
			}
			if t2 != css.ColonToken {
				continue
			}

			var composed []string
			var from string
			for {
				t3, text3 := l.Next()
				if t3 == css.ErrorToken || t3 == css.SemicolonToken {
					break
				}
				if t3 == css.WhitespaceToken {
					continue
				}
				if t3 == css.IdentToken && bytes.Equal(text3, bFrom) {
					// read path string
					for {
						t4, text4 := l.Next()
						if t4 == css.ErrorToken || t4 == css.SemicolonToken {
							break
						}
						if t4 == css.StringToken {
							from = strings.Trim(string(text4), `"'`)
							break
						}
					}
					break
				}
				if t3 == css.IdentToken || t3 == css.DelimToken {
					composed = append(composed, string(text3))
				}
			}
			if len(composed) > 0 && from != "" {
				t.Composes = append(t.Composes, Compose{
					Classes: composed,
					From:    from,
				})
			}
			continue
		}

		// Extract animation names from animation / animation-name properties at any depth
		if typ == css.IdentToken && (bytes.Equal(text, bAnimation) || bytes.Equal(text, bAnimationName)) {
			// expect :
			t2, _ := l.Next()
			for t2 == css.WhitespaceToken {
				t2, _ = l.Next()
			}
			if t2 != css.ColonToken {
				continue
			}
			extractAnimationNames(l, t)
		}
	}
}

func extractAnimationNames(l *css.Lexer, t *Tokens) {
	for {
		typ, text := l.Next()
		if typ == css.ErrorToken || typ == css.SemicolonToken {
			break
		}
		if typ == css.WhitespaceToken {
			continue
		}
		// Skip function calls like var(...)
		if typ == css.FunctionToken {
			for {
				t2, _ := l.Next()
				if t2 == css.ErrorToken || t2 == css.RightParenthesisToken {
					break
				}
			}
			continue
		}
		if typ == css.IdentToken {
			name := string(text)
			if !knownKeywords[name] {
				t.Keyframes = append(t.Keyframes, name)
			}
		}
	}
}

func handleImport(l *css.Lexer, t *Tokens) error {
	typ, text := l.Next()
	for typ == css.WhitespaceToken {
		typ, text = l.Next()
	}
	if typ == css.StringToken {
		path := strings.Trim(string(text), `"'`)
		t.Imports = append(t.Imports, path)
	}
	return skipUntilSemicolon(l)
}

func handleAtRule(l *css.Lexer, name []byte, t *Tokens) error {
	switch {
	case bytes.Equal(name, bKeyframes):
		return handleKeyframes(l, t)
	case bytes.Equal(name, bValue):
		return handleValue(l, t)
	}
	return skipAtRuleBlock(l, t)
}

func handleKeyframes(l *css.Lexer, t *Tokens) error {
	typ, text := l.Next()
	for typ == css.WhitespaceToken {
		typ, text = l.Next()
	}

	if typ != css.IdentToken {
		return skipAtRuleBlock(l, t)
	}

	name := string(text)

	// skip :global(...) wrapper
	if bytes.Equal(text, bGlobalPseudo) {
		// skip whitespace then consume (
		typ, txt := l.Next()
		for typ == css.WhitespaceToken {
			typ, txt = l.Next()
		}
		_ = txt
		if typ == css.LeftParenthesisToken {
			// skip until matching )
			depth := 1
			for {
				typ, _ = l.Next()
				if typ == css.ErrorToken {
					break
				}
				if typ == css.LeftParenthesisToken {
					depth++
				} else if typ == css.RightParenthesisToken {
					depth--
					if depth == 0 {
						break
					}
				}
			}
		}
		return skipAtRuleBlock(l, t)
	}

	t.Keyframes = append(t.Keyframes, name)
	return skipAtRuleBlock(l, t)
}

func handleValue(l *css.Lexer, t *Tokens) error {
	typ, text := l.Next()
	for typ == css.WhitespaceToken {
		typ, text = l.Next()
	}

	if typ != css.IdentToken {
		return skipUntilSemicolon(l)
	}

	name := string(text)

	// peek next non-whitespace
	typ, _ = l.Next()
	for typ == css.WhitespaceToken {
		typ, _ = l.Next()
	}

	if typ == css.ColonToken {
		// @value name: value;
		t.Values = append(t.Values, name)
		return skipUntilSemicolon(l)
	}

	return skipUntilSemicolon(l)
}

func handleExportBlock(l *css.Lexer, t *Tokens) error {
	// skip to {
	for {
		typ, _ := l.Next()
		if typ == css.ErrorToken {
			return nil
		}
		if typ == css.LeftBraceToken {
			break
		}
	}

	for {
		typ, text := l.Next()
		if typ == css.ErrorToken {
			return nil
		}
		if typ == css.RightBraceToken {
			return nil
		}
		if typ == css.IdentToken {
			prop := string(text)
			// skip to ; or }
			for {
				t2, _ := l.Next()
				if t2 == css.ErrorToken || t2 == css.RightBraceToken || t2 == css.SemicolonToken {
					break
				}
			}
			t.Exports = append(t.Exports, prop)
		}
	}
}

func readClassName(l *css.Lexer) (string, error) {
	typ, text := l.Next()
	for typ == css.WhitespaceToken {
		typ, text = l.Next()
	}
	if typ != css.IdentToken {
		return "", nil
	}
	return string(text), nil
}

func skipAtRuleBlock(l *css.Lexer, t *Tokens) error {
	// Scan forward until we find { or ; (at-rules like @media have a prelude before {)
	for {
		typ, _ := l.Next()
		if typ == css.ErrorToken {
			return nil
		}
		if typ == css.LeftBraceToken {
			break
		}
		if typ == css.SemicolonToken {
			return nil
		}
	}

	depth := 1
	for {
		typ, text := l.Next()
		if typ == css.ErrorToken {
			break
		}
		if typ == css.FunctionToken && bytes.Equal(text, bGlobal) {
			for {
				t2, _ := l.Next()
				if t2 == css.ErrorToken || t2 == css.RightParenthesisToken {
					break
				}
			}
			continue
		}
		if typ == css.DelimToken && len(text) > 0 && text[0] == '.' {
			className, err := readClassName(l)
			if err == nil && className != "" {
				t.Classes = append(t.Classes, className)
			}
			continue
		}
		if len(text) > 0 && text[0] == '{' {
			depth++
		} else if len(text) > 0 && text[0] == '}' {
			depth--
			if depth == 0 {
				break
			}
		}
	}
	return nil
}

func skipUntilSemicolon(l *css.Lexer) error {
	for {
		typ, _ := l.Next()
		if typ == css.ErrorToken || typ == css.SemicolonToken {
			return nil
		}
	}
}

// NormalizeTokens deduplicates and sorts tokens alphabetically.
func NormalizeTokens(tokens []string) []string {
	seen := make(map[string]struct{}, len(tokens))
	uniq := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			uniq = append(uniq, t)
		}
	}
	slices.Sort(uniq)
	return uniq
}

// CamelCase converts a token to camelCase (lowercases first part).
func CamelCase(s string) string {
	parts := strings.Split(s, "-")
	result := strings.ToLower(parts[0])
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			result += strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return result
}

// DashesCamelCase only camelizes dashes, keeps first-part case.
func DashesCamelCase(s string) string {
	parts := strings.Split(s, "-")
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			result += strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return result
}

// IsValidIdentifier checks if s is a valid TypeScript/JavaScript identifier.
func IsValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !(r == '_' || r == '$' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
				return false
			}
		} else {
			if !(r == '_' || r == '$' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
				return false
			}
		}
	}
	return true
}

// ConvertKey applies camelCase transformation.
func ConvertKey(key string, camelCase bool, dashes bool) string {
	if dashes {
		return DashesCamelCase(key)
	}
	if camelCase {
		return CamelCase(key)
	}
	return key
}

// AllTokens returns merged list of classes, keyframes, values, exports.
func (t *Tokens) AllTokens() []string {
	all := make([]string, 0, len(t.Classes)+len(t.Keyframes)+len(t.Values)+len(t.Exports))
	all = append(all, t.Classes...)
	all = append(all, t.Keyframes...)
	all = append(all, t.Values...)
	all = append(all, t.Exports...)
	return NormalizeTokens(all)
}
