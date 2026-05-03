package generate

import (
	"testing"
)

func TestGenerateCommonJS(t *testing.T) {
	out, err := Generate([]string{"myClass"}, Config{ExportType: ExportCommonJS, EOL: "\n"})
	if err != nil {
		t.Fatal(err)
	}
	want := `declare const styles: {
  readonly "myClass": string;
};
export = styles;

`
	if out.Formatted != want {
		t.Fatalf("expected:\n%s\ngot:\n%s", want, out.Formatted)
	}
}

func TestGenerateDefault(t *testing.T) {
	out, err := Generate([]string{"myClass"}, Config{ExportType: ExportDefault, EOL: "\n"})
	if err != nil {
		t.Fatal(err)
	}
	want := `declare const styles: {
  readonly "myClass": string;
};
export default styles;

`
	if out.Formatted != want {
		t.Fatalf("expected:\n%s\ngot:\n%s", want, out.Formatted)
	}
}

func TestGenerateNamed(t *testing.T) {
	out, err := Generate([]string{"myClass"}, Config{ExportType: ExportNamed, EOL: "\n"})
	if err != nil {
		t.Fatal(err)
	}
	want := `export const __esModule: true;
export const myClass: string;
`
	if out.Formatted != want {
		t.Fatalf("expected:\n%s\ngot:\n%s", want, out.Formatted)
	}
}

func TestGenerateEmpty(t *testing.T) {
	out, err := Generate([]string{}, Config{ExportType: ExportCommonJS, EOL: "\n"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Formatted != "export {};" {
		t.Fatalf("expected export {}, got %q", out.Formatted)
	}
}

func TestGenerateCamelCase(t *testing.T) {
	out, err := Generate([]string{"my-class"}, Config{ExportType: ExportCommonJS, CamelCase: true, EOL: "\n"})
	if err != nil {
		t.Fatal(err)
	}
	want := `declare const styles: {
  readonly "myClass": string;
};
export = styles;

`
	if out.Formatted != want {
		t.Fatalf("expected:\n%s\ngot:\n%s", want, out.Formatted)
	}
}

func TestOutputFileName(t *testing.T) {
	if got := OutputFileName("style.css", Config{}); got != "style.css.d.ts" {
		t.Fatalf("expected style.css.d.ts, got %s", got)
	}
	if got := OutputFileName("style.css", Config{DropExtension: true}); got != "style.d.ts" {
		t.Fatalf("expected style.d.ts, got %s", got)
	}
	if got := OutputFileName("style.css", Config{AllowArbitraryExtensions: true}); got != "style.d.css.ts" {
		t.Fatalf("expected style.d.css.ts, got %s", got)
	}
	if got := OutputFileName("style.pcss", Config{}); got != "style.pcss.d.ts" {
		t.Fatalf("expected style.pcss.d.ts, got %s", got)
	}
}
