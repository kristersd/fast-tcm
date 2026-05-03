package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kristersd/fast-tcm/internal/run"
)

func main() {
	var opts run.Options

	flag.StringVar(&opts.Pattern, "p", "", "Glob pattern with css files")
	flag.StringVar(&opts.Pattern, "pattern", "", "Glob pattern with css files")
	flag.StringVar(&opts.OutDir, "o", "", "Output directory")
	flag.StringVar(&opts.OutDir, "outDir", "", "Output directory")
	flag.BoolVar(&opts.Watch, "w", false, "Watch input directory's css files or pattern")
	flag.BoolVar(&opts.Watch, "watch", false, "Watch input directory's css files or pattern")
	flag.BoolVar(&opts.ListDifferent, "l", false, "List any files that are different than those that would be generated")
	flag.BoolVar(&opts.ListDifferent, "listDifferent", false, "List any files that are different than those that would be generated")
	flag.BoolVar(&opts.CamelCase, "c", false, "Camelize CSS token names")
	flag.BoolVar(&opts.CamelCase, "camelCase", false, "Camelize CSS token names")
	flag.BoolVar(&opts.NamedExports, "e", false, "Use named exports (deprecated, use --exportType)")
	flag.BoolVar(&opts.NamedExports, "namedExports", false, "Use named exports (deprecated, use --exportType)")
	flag.StringVar(&opts.ExportType, "exportType", "commonjs", "Export type: commonjs, default, named")
	flag.BoolVar(&opts.AllowArbitraryExtensions, "a", false, "Use .d.css.ts extension for arbitrary extensions")
	flag.BoolVar(&opts.AllowArbitraryExtensions, "allowArbitraryExtensions", false, "Use .d.css.ts extension for arbitrary extensions")
	flag.BoolVar(&opts.DropExtension, "d", false, "Drop the input files extension")
	flag.BoolVar(&opts.DropExtension, "dropExtension", false, "Drop the input files extension")
	flag.BoolVar(&opts.Silent, "s", false, "Silent output")
	flag.BoolVar(&opts.Silent, "silent", false, "Silent output")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ftcm [options] <search directory>\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	args := flag.Args()
	if len(args) < 1 && opts.Pattern == "" {
		flag.Usage()
		os.Exit(1)
	}

	searchDir := "."
	if len(args) > 0 {
		searchDir = args[0]
	}

	if err := run.Run(searchDir, opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
