package greeter

import "fmt"

// Greet prints a greeting for the given name.
func Greet(name string) {
	fmt.Println(message(name))
}

func message(name string) string {
	return "hello, " + name
}
