# fast-tcm

A high-performance CSS Modules type definition generator written in Go. Generates `.d.ts` files for `.css` and `.pcss` (PostCSS) files with support for `@keyframes`, `@value`, `:export`, and `composes`.

Approximately **8.5x faster** than [typed-css-modules](https://github.com/Quramy/typed-css-modules).

## Installation

```bash
npm install -D fast-tcm
```

No Go installation required—the package includes prebuilt binaries for macOS (Intel/Apple Silicon), Linux (x86/ARM), and Windows (x86).

## Usage

```bash
npx ftcm src
```

This generates `.d.ts` files alongside `.css` and `.pcss` files in `src` and subdirectories.

## Supported Files

- `.css` (standard CSS)
- `.pcss` (PostCSS)

## Options

- `--exportType <type>` — Export format: `commonjs`, `default`, or `named` (default: `default`)
- `--camelCase` — Convert kebab-case to camelCase in generated types
- `--dropExtension` — Remove file extension from generated `.d.ts` names
- `--allowArbitraryExtensions` — Allow non-standard CSS extensions (e.g., `.module.css`)
- `--outDir <dir>` — Write `.d.ts` files to a different directory
- `--pattern <glob>` — Custom glob pattern to match files (default: `**/*.{css,pcss}`)
- `--listDifferent` — List files with out-of-date or missing `.d.ts` (exit code 1 if any found)
- `--watch` — Watch for file changes and regenerate on-the-fly

## Example

```bash
# Generate with camelCase and named exports
npx ftcm --exportType named --camelCase src
```

## License

MIT
