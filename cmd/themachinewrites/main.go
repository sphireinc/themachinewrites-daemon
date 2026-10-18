package main

import (
	"fmt"
	"io"
	"os"
)

const usage = `Usage: themachinewrites <command>

Commands:
  run              run the supervisor until canceled
  once             execute one bounded cycle
  validate-config  validate daemon configuration
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}

	switch args[0] {
	case "run", "once", "validate-config":
		fmt.Fprintf(stdout, "%s command not implemented\n", args[0])
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
