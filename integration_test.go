package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kristersd/fast-tcm/internal/generate"
	"github.com/kristersd/fast-tcm/internal/resolver"
)

var ftcmBinary string

func TestMain(m *testing.M) {
	var err error
	ftcmBinary, err = os.MkdirTemp("", "ftcm-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(ftcmBinary)

	binaryPath := filepath.Join(ftcmBinary, "ftcm")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/ftcm")
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build ftcm binary: %v\n%s", err, output)
		os.Exit(1)
	}
	ftcmBinary = binaryPath

	code := m.Run()
	os.Exit(code)
}

func runOnFixture(t *testing.T, filePath string, cfg generate.Config) string {
	t.Helper()
	res := resolver.NewResolver("testdata/upstream", nil)
	tokens, err := res.Resolve(filePath)
	if err != nil {
		t.Fatalf("resolve %s: %v", filePath, err)
	}
	out, err := generate.Generate(tokens, cfg)
	if err != nil {
		t.Fatalf("generate %s: %v", filePath, err)
	}
	return out.Formatted
}

func TestUpstreamTestStyleCommonJS(t *testing.T) {
	got := runOnFixture(t, "testdata/upstream/testStyle.css", generate.Config{ExportType: generate.ExportCommonJS, EOL: "\n"})
	want := `declare const styles: {
  readonly "myClass": string;
};
export = styles;

`
	if got != want {
		t.Fatalf("mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestUpstreamKebabedCamelCase(t *testing.T) {
	got := runOnFixture(t, "testdata/upstream/kebabed.css", generate.Config{ExportType: generate.ExportCommonJS, CamelCase: true, EOL: "\n"})
	want := `declare const styles: {
  readonly "myClass": string;
};
export = styles;

`
	if got != want {
		t.Fatalf("mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestUpstreamKebabedUpperCaseDashes(t *testing.T) {
	got := runOnFixture(t, "testdata/upstream/kebabedUpperCase.css", generate.Config{ExportType: generate.ExportCommonJS, Dashes: true, EOL: "\n"})
	want := `declare const styles: {
  readonly "MyClass": string;
};
export = styles;

`
	if got != want {
		t.Fatalf("mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestUpstreamEmpty(t *testing.T) {
	got := runOnFixture(t, "testdata/upstream/empty.css", generate.Config{ExportType: generate.ExportCommonJS, EOL: "\n"})
	if got != "export {};" {
		t.Fatalf("expected export {}, got %q", got)
	}
}

func TestUpstreamComposer(t *testing.T) {
	got := runOnFixture(t, "testdata/upstream/composer.css", generate.Config{ExportType: generate.ExportCommonJS, EOL: "\n"})
	want := `declare const styles: {
  readonly "root": string;
};
export = styles;

`
	if got != want {
		t.Fatalf("mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestUpstreamCombined(t *testing.T) {
	got := runOnFixture(t, "testdata/upstream/combined/combined.css", generate.Config{ExportType: generate.ExportCommonJS, EOL: "\n"})
	want := `declare const styles: {
  readonly "block": string;
  readonly "box": string;
  readonly "myClass": string;
};
export = styles;

`
	if got != want {
		t.Fatalf("mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestUpstreamNamedExports(t *testing.T) {
	got := runOnFixture(t, "testdata/upstream/testStyle.css", generate.Config{ExportType: generate.ExportNamed, EOL: "\n"})
	want := `export const __esModule: true;
export const myClass: string;
`
	if got != want {
		t.Fatalf("mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestUpstreamDefaultExport(t *testing.T) {
	got := runOnFixture(t, "testdata/upstream/testStyle.css", generate.Config{ExportType: generate.ExportDefault, EOL: "\n"})
	want := `declare const styles: {
  readonly "myClass": string;
};
export default styles;

`
	if got != want {
		t.Fatalf("mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestExtensionKeyframes(t *testing.T) {
	got := runOnFixture(t, "testdata/extensions/keyframes.css", generate.Config{ExportType: generate.ExportCommonJS, EOL: "\n"})
	want := `declare const styles: {
  readonly "fade": string;
  readonly "myClass": string;
};
export = styles;

`
	if got != want {
		t.Fatalf("mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestExtensionPCSS(t *testing.T) {
	got := runOnFixture(t, "testdata/extensions/simple.pcss", generate.Config{ExportType: generate.ExportCommonJS, EOL: "\n"})
	want := `declare const styles: {
  readonly "myClass": string;
};
export = styles;

`
	if got != want {
		t.Fatalf("mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestExtensionPCSSOutputName(t *testing.T) {
	name := generate.OutputFileName("style.pcss", generate.Config{})
	if name != "style.pcss.d.ts" {
		t.Fatalf("expected style.pcss.d.ts, got %s", name)
	}
}

func TestCLIWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "test.css")
	if err := os.WriteFile(src, []byte(".foo { color: red; }"), 0644); err != nil {
		t.Fatal(err)
	}

	cliPath := filepath.Join(tmpDir, "ftcm")
	if err := runCmd("go", "build", "-o", cliPath, "./cmd/ftcm"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmdOutput(cliPath, tmpDir)
	if err != nil {
		t.Fatalf("cli failed: %v\n%s", err, out)
	}

	expected := filepath.Join(tmpDir, "test.css.d.ts")
	content, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("expected file %s: %v", expected, err)
	}
	if !strings.Contains(string(content), "foo") {
		t.Fatalf("expected output to contain foo, got:\n%s", string(content))
	}
}

func TestCLICamelCase(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "test.css")
	if err := os.WriteFile(src, []byte(".my-class { color: red; }"), 0644); err != nil {
		t.Fatal(err)
	}

	cliPath := filepath.Join(tmpDir, "ftcm")
	if err := runCmd("go", "build", "-o", cliPath, "./cmd/ftcm"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmdOutput(cliPath, "-c", tmpDir)
	if err != nil {
		t.Fatalf("cli failed: %v\n%s", err, out)
	}

	expected := filepath.Join(tmpDir, "test.css.d.ts")
	content, err := os.ReadFile(expected)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "myClass") {
		t.Fatalf("expected camelCase myClass, got:\n%s", string(content))
	}
}

func TestCLIExportTypeDefault(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "test.css")
	if err := os.WriteFile(src, []byte(".foo { color: red; }"), 0644); err != nil {
		t.Fatal(err)
	}

	cliPath := filepath.Join(tmpDir, "ftcm")
	if err := runCmd("go", "build", "-o", cliPath, "./cmd/ftcm"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmdOutput(cliPath, "--exportType", "default", tmpDir)
	if err != nil {
		t.Fatalf("cli failed: %v\n%s", err, out)
	}

	expected := filepath.Join(tmpDir, "test.css.d.ts")
	content, err := os.ReadFile(expected)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "export default styles;") {
		t.Fatalf("expected default export, got:\n%s", string(content))
	}
}

func TestCLIPCSSDiscovery(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "test.pcss")
	if err := os.WriteFile(src, []byte(".foo { color: red; }"), 0644); err != nil {
		t.Fatal(err)
	}

	cliPath := filepath.Join(tmpDir, "ftcm")
	if err := runCmd("go", "build", "-o", cliPath, "./cmd/ftcm"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmdOutput(cliPath, tmpDir)
	if err != nil {
		t.Fatalf("cli failed: %v\n%s", err, out)
	}

	expected := filepath.Join(tmpDir, "test.pcss.d.ts")
	content, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("expected file %s: %v", expected, err)
	}
	if !strings.Contains(string(content), "foo") {
		t.Fatalf("expected output to contain foo, got:\n%s", string(content))
	}
}

func TestCLIListDifferent(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "test.css")
	if err := os.WriteFile(src, []byte(".foo { color: red; }"), 0644); err != nil {
		t.Fatal(err)
	}

	cliPath := filepath.Join(tmpDir, "ftcm")
	if err := runCmd("go", "build", "-o", cliPath, "./cmd/ftcm"); err != nil {
		t.Fatal(err)
	}

	// first generate
	if _, err := runCmdOutput(cliPath, tmpDir); err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	// listDifferent should pass now
	out, err := runCmdOutput(cliPath, "-l", tmpDir)
	if err != nil {
		t.Fatalf("listDifferent failed: %v\n%s", err, out)
	}

	// modify the d.ts
	dts := filepath.Join(tmpDir, "test.css.d.ts")
	if err := os.WriteFile(dts, []byte("wrong"), 0644); err != nil {
		t.Fatal(err)
	}

	// listDifferent should fail now
	_, err = runCmdOutput(cliPath, "-l", tmpDir)
	if err == nil {
		t.Fatal("expected listDifferent to fail after modification")
	}
}

func TestCLIOUTDir(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	outDir := filepath.Join(tmpDir, "dist")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(srcDir, "test.css")
	if err := os.WriteFile(src, []byte(".foo { color: red; }"), 0644); err != nil {
		t.Fatal(err)
	}

	cliPath := filepath.Join(tmpDir, "ftcm")
	if err := runCmd("go", "build", "-o", cliPath, "./cmd/ftcm"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmdOutput(cliPath, "-o", outDir, srcDir)
	if err != nil {
		t.Fatalf("cli failed: %v\n%s", err, out)
	}

	expected := filepath.Join(outDir, "test.css.d.ts")
	content, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("expected file %s: %v", expected, err)
	}
	if !strings.Contains(string(content), "foo") {
		t.Fatalf("expected output to contain foo, got:\n%s", string(content))
	}
}

func TestCLIConcurrentImports(t *testing.T) {
	tmpDir := t.TempDir()

	// Shared stylesheet imported by multiple entries
	shared := filepath.Join(tmpDir, "shared.css")
	if err := os.WriteFile(shared, []byte(".sharedClass { color: blue; }\n@import \"./base.css\";"), 0644); err != nil {
		t.Fatal(err)
	}

	base := filepath.Join(tmpDir, "base.css")
	if err := os.WriteFile(base, []byte(".baseClass { margin: 0; }"), 0644); err != nil {
		t.Fatal(err)
	}

	// Multiple entry files that all import shared.css
	for i := 0; i < 10; i++ {
		src := filepath.Join(tmpDir, fmt.Sprintf("file%d.css", i))
		content := fmt.Sprintf(".class%d { color: red; }\n@import \"./shared.css\";", i)
		if err := os.WriteFile(src, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cliPath := filepath.Join(tmpDir, "ftcm")
	if err := runCmd("go", "build", "-o", cliPath, "./cmd/ftcm"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmdOutput(cliPath, tmpDir)
	if err != nil {
		t.Fatalf("cli failed: %v\n%s", err, out)
	}

	// Verify every entry file got a correct .d.ts with both its own class and shared tokens
	for i := 0; i < 10; i++ {
		expected := filepath.Join(tmpDir, fmt.Sprintf("file%d.css.d.ts", i))
		content, err := os.ReadFile(expected)
		if err != nil {
			t.Fatalf("expected file %s: %v", expected, err)
		}
		if !strings.Contains(string(content), fmt.Sprintf("class%d", i)) {
			t.Fatalf("expected output to contain class%d, got:\n%s", i, string(content))
		}
		if !strings.Contains(string(content), "sharedClass") {
			t.Fatalf("expected output to contain sharedClass, got:\n%s", string(content))
		}
		if !strings.Contains(string(content), "baseClass") {
			t.Fatalf("expected output to contain baseClass, got:\n%s", string(content))
		}
	}
}

func TestCLIVersion(t *testing.T) {
	output, err := runCmdOutput(ftcmBinary, "--version")
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if !strings.Contains(string(output), "ftcm version") {
		t.Fatalf("expected version output, got: %s", output)
	}
}

func TestCLIRootDotSearch(t *testing.T) {
	tmpDir := t.TempDir()
	cssFile := filepath.Join(tmpDir, "test.css")
	if err := os.WriteFile(cssFile, []byte(".myClass { color: red; }"), 0644); err != nil {
		t.Fatalf("write css: %v", err)
	}

	outDir := filepath.Join(tmpDir, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("mkdir outDir: %v", err)
	}

	if err := runCmd(ftcmBinary, "-o", outDir, tmpDir); err != nil {
		t.Fatalf("ftcm failed: %v", err)
	}

	dtsFile := filepath.Join(outDir, "test.css.d.ts")
	if _, err := os.Stat(dtsFile); os.IsNotExist(err) {
		t.Fatalf("expected output file %s not found", dtsFile)
	}
}

func TestCLIInvalidExportType(t *testing.T) {
	tmpDir := t.TempDir()
	cssFile := filepath.Join(tmpDir, "test.css")
	if err := os.WriteFile(cssFile, []byte(".myClass { color: red; }"), 0644); err != nil {
		t.Fatalf("write css: %v", err)
	}

	// Should fail with invalid export type
	output, err := runCmdOutput(ftcmBinary, "--exportType", "invalid", tmpDir)
	if err == nil {
		t.Fatalf("expected error for invalid exportType, got success")
	}
	if !strings.Contains(string(output), "invalid") || !strings.Contains(string(output), "export") {
		t.Fatalf("expected error message about exportType, got: %s", output)
	}
}

func runCmd(name string, arg ...string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(name, arg...)
	cmd.Dir = wd
	return cmd.Run()
}

func runCmdOutput(name string, arg ...string) ([]byte, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(name, arg...)
	cmd.Dir = wd
	return cmd.CombinedOutput()
}
