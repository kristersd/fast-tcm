package parser

import (
	"io"
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

// ExtractTokens parses CSS/PCSS content and extracts CSS Modules tokens.
func ExtractTokens(src []byte) (*Tokens, error) {
	l := css.NewLexer(parse.NewInputBytes(src))
	t := &Tokens{}

	var prevTyp css.TokenType
	var prevText []byte

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
			if string(text) == "@import" {
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
			s := string(text)
			if s == "export" && prevTyp == css.ColonToken && string(prevText) == ":" {
				err := handleExportBlock(l, t)
				if err != nil {
					return nil, err
				}
			}
		case css.FunctionToken:
			if string(text) == "global(" {
				// skip until matching )
				for {
					t2, _ := l.Next()
					if t2 == css.ErrorToken || t2 == css.RightParenthesisToken {
						break
					}
				}
			}
		case css.DelimToken:
			if text[0] == '.' {
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
		prevText = text
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
		if typ == css.FunctionToken && string(text) == "global(" {
			// skip until matching )
			for {
				t2, _ := l.Next()
				if t2 == css.ErrorToken || t2 == css.RightParenthesisToken {
					break
				}
			}
			continue
		}
		if typ == css.DelimToken && text[0] == '.' {
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
		if blockDepth > 1 {
			continue
		}

		if typ == css.IdentToken && string(text) == "composes" {
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
				if t3 == css.IdentToken && string(text3) == "from" {
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
	switch string(name) {
	case "@keyframes":
		return handleKeyframes(l, t)
	case "@value":
		return handleValue(l, t)
	}
	return skipAtRuleBlock(l)
}

func handleKeyframes(l *css.Lexer, t *Tokens) error {
	typ, text := l.Next()
	for typ == css.WhitespaceToken {
		typ, text = l.Next()
	}

	if typ != css.IdentToken {
		return skipAtRuleBlock(l)
	}

	name := string(text)

	// skip :global(...) wrapper
	if name == ":global" {
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
		return skipAtRuleBlock(l)
	}

	t.Keyframes = append(t.Keyframes, name)
	return skipAtRuleBlock(l)
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

func skipAtRuleBlock(l *css.Lexer) error {
	typ, _ := l.Next()
	for typ == css.WhitespaceToken {
		typ, _ = l.Next()
	}

	if typ != css.LeftBraceToken {
		return skipUntilSemicolon(l)
	}

	depth := 1
	for {
		typ, text := l.Next()
		if typ == css.ErrorToken {
			break
		}
		if text[0] == '{' {
			depth++
		} else if text[0] == '}' {
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
	for i := 0; i < len(uniq); i++ {
		for j := i + 1; j < len(uniq); j++ {
			if uniq[i] > uniq[j] {
				uniq[i], uniq[j] = uniq[j], uniq[i]
			}
		}
	}
	return uniq
}

// CamelCase converts a token to camelCase (lowercases first part).
func CamelCase(s string) string {
	parts := strings.Split(s, "-")
	if len(parts) == 0 {
		return s
	}
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
	if len(parts) == 0 {
		return s
	}
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

// ErrorToken wraps lexer errors.
type ErrorToken struct {
	Msg string
}

func (e *ErrorToken) Error() string {
	return e.Msg
}
