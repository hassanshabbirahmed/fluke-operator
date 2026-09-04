package main

import "fmt"

// Takes a COPY of n. Changing it here does nothing to the caller.
func tryToSetByValue(n int) {
	n = 99
}

// Takes a pointer: the ADDRESS of the caller's int.
// *int in the parameter list = "I want a pointer to an int".
func setByPointer(n *int) {
	*n = 99 // *n means "the int living at that address" - assign through it
}

func main() {
	x := 1

	tryToSetByValue(x)
	fmt.Println("after tryToSetByValue:", x) // still 1

	setByPointer(&x)                        // &x = "the address of x"
	fmt.Println("after setByPointer:  ", x) // now 99

	// What a pointer actually is: print one.
	p := &x
	fmt.Println("p (an address):", p)   // something like 0xc00001c030
	fmt.Println("*p (the value): ", *p) // 99

	// The zero value of a pointer is nil - it points at nothing.
	var q *int
	fmt.Println("q is nil:", q == nil)
	// fmt.Println(*q) // this would CRASH: "invalid memory address or nil pointer dereference"
}
