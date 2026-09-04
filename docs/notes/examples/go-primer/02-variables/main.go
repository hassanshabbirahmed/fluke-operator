package main

import "fmt"

func main() {
	// Two ways to make a variable:

	// 1. Full form: "var" NAME TYPE = VALUE
	var replicas int = 3

	// 2. Short form: NAME := VALUE   (Go infers the type from the value)
	//    Only usable inside a function. This is what you'll write 95% of the time.
	name := "fluke-sample"

	// Every type has a "zero value" - what you get if you don't assign one.
	// There is no None/nil for basic types: an int is 0, a string is "", a bool is false.
	var ready bool // zero value: false

	fmt.Println(name, "wants", replicas, "replicas; ready =", ready)

	// Types are fixed. This line is a COMPILE error - uncomment to see:
	// replicas = "lots"

	// Go will not mix number types automatically. Also a compile error:
	// var ratio float64 = 2.5
	// fmt.Println(replicas * ratio)

	// You convert explicitly, by wrapping in the type name like a function call:
	var ratio float64 = 2.5
	fmt.Println(float64(replicas) * ratio)
}
