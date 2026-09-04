package main

import (
	"context"
	"fmt"
	"time"
)

// Every function that might take a while accepts a ctx as its FIRST argument.
// It watches ctx.Done() to know when to give up early.
func doSlowWork(ctx context.Context) error {
	select {
	case <-time.After(3 * time.Second): // pretend this is a slow API call
		fmt.Println("work finished normally")
		return nil
	case <-ctx.Done(): // this channel CLOSES when the context is cancelled/expires
		fmt.Println("work aborted:", ctx.Err())
		return ctx.Err()
	}
}

func main() {
	// Make a context that auto-cancels after 1 second - shorter than the "work".
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // ALWAYS defer cancel() - releases resources even if you return early

	err := doSlowWork(ctx)
	fmt.Println("returned error:", err)
}
