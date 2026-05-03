package parser

import (
	"testing"
)

func TestExtractTokensSimpleClass(t *testing.T) {
	src := `.myClass { color: red; }`
	tokens, err := ExtractTokens([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens.Classes) != 1 || tokens.Classes[0] != "myClass" {
		t.Fatalf("expected [myClass], got %v", tokens.Classes)
	}
}

func TestExtractTokensMultipleClasses(t *testing.T) {
	src := `.foo { color: red; } .bar { color: blue; }`
	tokens, err := ExtractTokens([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens.Classes) != 2 {
		t.Fatalf("expected 2 classes, got %d", len(tokens.Classes))
	}
}

func TestExtractTokensKeyframes(t *testing.T) {
	src := `@keyframes fade { from { opacity: 0; } to { opacity: 1; } }`
	tokens, err := ExtractTokens([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens.Keyframes) != 1 || tokens.Keyframes[0] != "fade" {
		t.Fatalf("expected [fade], got %v", tokens.Keyframes)
	}
}

func TestExtractTokensGlobalKeyframes(t *testing.T) {
	src := `@keyframes :global(spin) { from { transform: rotate(0deg); } }`
	tokens, err := ExtractTokens([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens.Keyframes) != 0 {
		t.Fatalf("expected no keyframes, got %v", tokens.Keyframes)
	}
}

func TestExtractTokensMixedGlobalSelector(t *testing.T) {
	src := `.localComponent :global(.external-library-class) { color: red; }`
	tokens, err := ExtractTokens([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should only extract localComponent, NOT external-library-class
	if len(tokens.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d: %v", len(tokens.Classes), tokens.Classes)
	}
	if tokens.Classes[0] != "localComponent" {
		t.Fatalf("expected [localComponent], got %v", tokens.Classes)
	}
}

func TestExtractTokensValue(t *testing.T) {
	src := `@value primary: red;`
	tokens, err := ExtractTokens([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens.Values) != 1 || tokens.Values[0] != "primary" {
		t.Fatalf("expected [primary], got %v", tokens.Values)
	}
}

func TestExtractTokensExport(t *testing.T) {
	src := `:export { primary: red; secondary: blue; }`
	tokens, err := ExtractTokens([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens.Exports) != 2 {
		t.Fatalf("expected 2 exports, got %v", tokens.Exports)
	}
}

func TestExtractTokensComposes(t *testing.T) {
	src := `.myClass { composes: box from "./other.css"; }`
	tokens, err := ExtractTokens([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens.Composes) != 1 {
		t.Fatalf("expected 1 compose, got %d", len(tokens.Composes))
	}
	if tokens.Composes[0].From != "./other.css" {
		t.Fatalf("expected from ./other.css, got %s", tokens.Composes[0].From)
	}
}

func TestExtractTokensGlobalClass(t *testing.T) {
	src := `:global(.ignored) { color: red; } .local { color: blue; }`
	tokens, err := ExtractTokens([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	foundIgnored := false
	foundLocal := false
	for _, c := range tokens.Classes {
		if c == "ignored" {
			foundIgnored = true
		}
		if c == "local" {
			foundLocal = true
		}
	}
	if foundIgnored {
		t.Fatal("expected :global(.ignored) to be skipped")
	}
	if !foundLocal {
		t.Fatal("expected .local to be found")
	}
}

func TestNormalizeTokens(t *testing.T) {
	got := NormalizeTokens([]string{"b", "a", "b", "c"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestCamelCase(t *testing.T) {
	if CamelCase("my-class") != "myClass" {
		t.Fatalf("expected myClass, got %s", CamelCase("my-class"))
	}
	if CamelCase("my-class-name") != "myClassName" {
		t.Fatalf("expected myClassName, got %s", CamelCase("my-class-name"))
	}
}

func TestDashesCamelCase(t *testing.T) {
	if DashesCamelCase("My-Class") != "MyClass" {
		t.Fatalf("expected MyClass, got %s", DashesCamelCase("My-Class"))
	}
}

func TestExtractTokensAnimationKeywords(t *testing.T) {
	src := `@keyframes myAnimation { 0% { opacity: 0; } 100% { opacity: 1; } }
.animated { animation: myAnimation linear infinite; }`
	tokens, err := ExtractTokens([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should extract myAnimation keyframe, but not linear or infinite (known keywords)
	found := false
	for _, k := range tokens.Keyframes {
		if k == "myAnimation" {
			found = true
		}
		if k == "linear" || k == "infinite" {
			t.Fatalf("should not extract keyword %s", k)
		}
	}
	if !found {
		t.Fatalf("expected myAnimation keyframe, got %v", tokens.Keyframes)
	}
}

func TestExtractTokensMediaQuery(t *testing.T) {
	src := `@media (max-width: 600px) { .responsive { color: red; } }
.static { color: blue; }`
	tokens, err := ExtractTokens([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should extract both classes inside and outside media query
	if len(tokens.Classes) != 2 {
		t.Fatalf("expected 2 classes, got %d: %v", len(tokens.Classes), tokens.Classes)
	}
}

func TestIsValidIdentifier(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple", "myClass", true},
		{"camelCase", "MyClass", true},
		{"underscore", "_private", true},
		{"dollar", "$var", true},
		{"number after", "class1", true},
		{"leading number", "1class", false},
		{"hyphen", "my-class", false},
		{"space", "my class", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidIdentifier(tt.input)
			if got != tt.want {
				t.Fatalf("IsValidIdentifier(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
