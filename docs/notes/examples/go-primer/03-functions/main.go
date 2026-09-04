package main

import (
	"errors"
	"fmt"
)

// A normal function: func NAME(PARAMS) RETURNTYPE { ... }
func double(n int) int {
	return n * 2
}

// The Go signature you'll see everywhere: return a result AND an error.
// "error" is a built-in type. nil means "no error" (nil = Bash exit 0).
func safeDivide(a, b int) (int, error) {
	if b == 0 {
		// errors.New builds a basic error. Return a zero result + the error.
		return 0, errors.New("cannot divide by zero")
	}
	return a / b, nil // success: real result + nil error
}

func main() {
	fmt.Println("double(21) =", double(21))

	// Calling a (result, error) function. You get BOTH back, always.
	result, err := safeDivide(10, 2)
	if err != nil { // the single most common line in Go
		fmt.Println("error:", err)
		return
	}
	fmt.Println("10 / 2 =", result)

	// Now the failing path:
	result, err = safeDivide(10, 0)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("10 / 0 =", result) // never reached
}
