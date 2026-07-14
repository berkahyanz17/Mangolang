package cmd

import "fmt"

func init() {
	Register(&Command{
		Name:        "greet",
		Description: "Print a greeting message",
		Run:         runGreet,
	})
}

func runGreet(args []string) error {
	fs := NewFlagSet("greet")
	name := fs.String("name", "world", "name to greet")
	loud := fs.Bool("loud", false, "shout the greeting")
	fs.Parse(args)

	msg := fmt.Sprintf("Hello, %s!", *name)
	if *loud {
		msg = fmt.Sprintf("%s!!!", msg)
	}
	fmt.Println(msg)
	return nil
}
