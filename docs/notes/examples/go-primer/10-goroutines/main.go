package main

import (
	"fmt"
	"sync"
)

func main() {
	var wg sync.WaitGroup // a counter that "wait" blocks on until it hits zero

	for i := 1; i <= 3; i++ {
		wg.Add(1)        // "one more thing to wait for"
		go func(n int) { // starts running concurrently, doesn't block main
			defer wg.Done() // "this one thing is done" - always defer it
			fmt.Println("worker", n, "running")
		}(i) // pass i in as an argument - don't capture the loop variable directly
	}

	wg.Wait() // block here until all 3 have called Done()
	fmt.Println("all workers finished")
}
