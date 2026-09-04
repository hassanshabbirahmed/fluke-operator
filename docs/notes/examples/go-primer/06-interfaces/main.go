package main

import "fmt"

// ---- an interface: a shape, defined as "must have these methods" ----
type Reconciler interface {
	Reconcile(name string) (string, error)
}

// ---- type 1: a struct with a method matching that shape ----
type FlukeController struct {
	DefaultImage string
}

// Method: receiver (c FlukeController) before the name. This alone makes
// FlukeController satisfy Reconciler - no "implements" needed.
func (c FlukeController) Reconcile(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("no name given")
	}
	return fmt.Sprintf("ensured Deployment %q with image %s", name, c.DefaultImage), nil
}

// ---- type 2: a totally different struct that ALSO matches the shape ----
type NoopController struct{}

func (n NoopController) Reconcile(name string) (string, error) {
	return "did nothing for " + name, nil
}

// A function that accepts ANYTHING satisfying Reconciler. It doesn't know or
// care which concrete struct it got.
func runOnce(r Reconciler, name string) {
	msg, err := r.Reconcile(name)
	if err != nil {
		fmt.Println("reconcile failed:", err)
		return
	}
	fmt.Println("ok:", msg)
}

func main() {
	runOnce(FlukeController{DefaultImage: "busybox"}, "demo")
	runOnce(NoopController{}, "demo")
	runOnce(FlukeController{DefaultImage: "busybox"}, "") // triggers the error path
}
