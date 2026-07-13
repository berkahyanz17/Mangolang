package cmd

import (
	"flag"
	"fmt"
	"os"
	"sort"
)

// Command represents a single CLI subcommand.
type Command struct {
	Name        string
	Description string
	Run         func(args []string) error
}

var registry = map[string]*Command{}

// Register adds a subcommand to the CLI.
func Register(c *Command) {
	registry[c.Name] = c
}

const appName = "mycli"
const appVersion = "0.1.0"

// Execute parses os.Args and dispatches to the right subcommand.
func Execute() error {
	if len(os.Args) < 2 {
		printUsage()
		return nil
	}

	switch os.Args[1] {
	case "-h", "--help", "help":
		printUsage()
		return nil
	case "-v", "--version", "version":
		fmt.Printf("%s version %s\n", appName, appVersion)
		return nil
	}

	cmdName := os.Args[1]
	c, ok := registry[cmdName]
	if !ok {
		printUsage()
		return fmt.Errorf("unknown command %q", cmdName)
	}

	return c.Run(os.Args[2:])
}

func printUsage() {
	fmt.Printf("%s - a CLI tool\n\n", appName)
	fmt.Printf("Usage:\n  %s <command> [flags]\n\n", appName)
	fmt.Println("Available commands:")

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		fmt.Printf("  %-12s %s\n", name, registry[name].Description)
	}

	fmt.Println("\nFlags:")
	fmt.Println("  -h, --help       show help")
	fmt.Println("  -v, --version    show version")
}

// NewFlagSet is a small helper subcommands can use for their own flags.
func NewFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	return fs
}
