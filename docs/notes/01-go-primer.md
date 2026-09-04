# Lesson 1 — Go primer for this project

Resolves [Lesson 1 — Go primer for this project](https://github.com/hassanshabbirahmed/fluke-operator/issues/2).

Not a full Go course — just the subset this project actually leans on, pitched at
someone comfortable with Bash and a bit of Python, rusty on programming in general,
and specifically wary of pointers and interfaces. Runnable examples for every
concept live in [`examples/go-primer/`](./examples/go-primer/) (one Go module,
one subfolder per concept, `go run ./NN-name` from inside that folder).

## The big shift coming from Bash/Python

Go is **compiled** and **statically typed**. Nothing runs until the whole program
type-checks; a typo like calling an undefined function is a build failure, not a
runtime crash three lines in. This feels stricter at first and becomes something
you lean on — a large class of bugs simply can't reach a running program.

## 1. Packages & modules

- **package**: a folder of `.go` files sharing a `package <name>` declaration.
  Closest thing you know: a Python module/folder.
- **`package main`**: the special package that produces a runnable binary. Must
  have exactly one `func main()`.
- **module**: a whole project, named and version-pinned by one `go.mod` at its
  root (`go mod init <name>`). Roughly `pyproject.toml` + a lockfile, but
  mandatory.
- **Running code**: `go run ./some/package` — must be invoked from inside the
  module (where `go.mod` lives or below it); Go resolves the package by its
  `./`-relative path, not by absolute filesystem path.

## 2. Variables & types

```go
var replicas int = 3   // full form
name := "fluke-sample"  // short form — infers the type; what you'll write 95% of the time
var ready bool          // no value given -> the ZERO VALUE (false for bool)
```

- Every variable has one fixed type, forever. No automatic mixing
  (`int * float64` won't compile) — convert explicitly: `float64(replicas)`.
- There is no "unset" for basic types — no `None`. A declared-but-unassigned
  variable is its type's zero value (`0`, `""`, `false`).
- **An unused local variable is a compile error.** Deliberate — dead code can't
  silently accumulate.

## 3. Functions, multiple returns, and errors-as-values

```go
func safeDivide(a, b int) (int, error) {
	if b == 0 {
		return 0, fmt.Errorf("cannot divide %d by zero", a)
	}
	return a / b, nil // nil = "no error"
}

result, err := safeDivide(10, 0)
if err != nil {
	return // handle it right here — nothing "jumps" the way an exception does
}
```

Go has no exceptions for ordinary failures. A function that can fail returns
`(result, error)`, and the caller checks `err != nil` immediately, every time.
`nil` is Go's "nothing" (like Python's `None`); for `error` specifically, `nil`
means success. `errors.New("...")` builds a plain error; `fmt.Errorf("...%d...", x)`
adds formatting.

## 4. Structs

```go
type FlukeSpec struct {
	Replicas         int
	FlukesPerReplica int
	Image            string
}

f := FlukeSpec{Replicas: 3, FlukesPerReplica: 2, Image: "busybox"}
fmt.Println(f.Replicas)
```

- A struct is a fixed set of named, typed fields — closest to a Python
  `@dataclass`.
- **Capitalization is Go's only visibility rule.** `Replicas` (capital) is
  exported (visible to other packages); `replicas` (lowercase) is private to
  the defining package. No `public`/`private` keywords.
- Real Kubernetes resources nest a `Spec` (what you want) and a `Status` (what
  the controller has observed) — the shape the `Fluke` CRD will follow.

## 5. Pointers

The one idea: **a pointer holds where a value lives, not the value itself.**

| Symbol | Reads as | Does |
|---|---|---|
| `*int` | "pointer to an int" | a type: an address, not a value |
| `&x` | "address of x" | produce a pointer to `x` |
| `*p` | "the value at p" | dereference — follow the pointer to the value |

```go
func setByPointer(n *int) { *n = 99 }

x := 1
setByPointer(&x)
fmt.Println(x) // 99
```

Go copies everything passed to a function **by default** — unlike Python, which
quietly passes lists/dicts/objects by reference but numbers/strings by value.
If a function must modify the caller's value, it takes a pointer, visibly. The
zero value of a pointer is `nil`; dereferencing a `nil` pointer (`*p` where
`p == nil`) is a runtime panic — the one real hazard pointers add.

**Why it matters here:** optional CRD fields are often pointers (`Replicas
*int32`) so `nil` ("not set") can be told apart from `0` ("set to zero") — a
plain `int` can't express that difference. The reconcile loop is also handed a
`*Fluke`, so writes to `.Status` land on the real object, not a throwaway copy.

## 6. Methods & interfaces

```go
func (f Fluke) Describe() string { ... }   // value receiver: gets a COPY
func (f *Fluke) MarkReady()      { ... }   // pointer receiver: can modify the real f
```

A method is a function with a **receiver** — Go's explicit, up-front `self`.
Same pointer rule as above: a method that mutates the struct needs a pointer
receiver.

```go
type Reconciler interface {
	Reconcile(name string) (string, error)
}
```

An interface is just a list of required method signatures. **Any type with
those methods satisfies the interface automatically** — no `implements`
keyword, no declaration of intent, checked by the compiler at every call site.
This is duck typing with a compile-time guarantee. `error` itself is secretly
`interface { Error() string }`.

**Why it matters here:** controller-runtime's `Reconciler` is exactly such an
interface (one method: `Reconcile(ctx, req) (Result, error)`). Your controller
becomes a reconciler purely by having a method with that shape — there's no
registration step for "being" a Reconciler.

## 7. Slices & maps

```go
images := []string{"busybox", "nginx"}
images = append(images, "redis")   // append RETURNS a (maybe new) slice — reassign it!

for i, v := range images { ... }

ages := map[string]int{"alice": 30}
ages["bob"] = 25
v, ok := ages["carol"]             // "comma ok": ok is false if the key's absent
```

- **slice** = Go's dynamic list (Python `list`). `append` doesn't reliably
  modify in place — always reassign its result.
- **map** = Go's dict. The "comma ok" idiom (`v, ok := m[key]`) replaces a
  try/except around a missing key.
- Map iteration order is random every run — never rely on it (unlike Python
  3.7+ dicts, which preserve insertion order).

## 8. Struct tags & JSON/YAML

```go
type FlukeSpec struct {
	Replicas int    `json:"replicas"`
	Image    string `json:"image,omitempty"`
}
```

A **struct tag** (the backtick string after a field) is metadata the
`encoding/json` package reads to decide the marshaled key name and behaviour.
`,omitempty` drops the field entirely if it's the zero value. **This is how a
Go struct becomes the YAML you `kubectl apply`** — Kubernetes's CRD machinery
works on exactly this mechanism.

The trap: `,omitempty` on a plain `int` can't tell "explicitly set to 0" apart
from "never set" — both vanish identically. That's the real reason optional-
but-meaningful Kubernetes fields are pointers (`*int32`), not plain numbers:
`nil` unambiguously means "not set."

`json.Unmarshal(bytes, &parsed)` takes `&parsed` — a pointer, because it has
to write into your struct (Concept 5 again).

## 9. `context.Context`

No real Bash/Python equivalent — new vocabulary, not new mechanics (it's just
an interface, per Concept 6). It carries a cancellation/deadline signal (and a
few request-scoped values like a logger) **down** a call chain. Convention:
first parameter of almost every controller-runtime function —
`Reconcile(ctx context.Context, req Request)`. You rarely construct a root
context in operator code; you're handed one per reconcile and thread it
through everything you call.

```go
ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
defer cancel() // always defer the cancel func right after creating ctx

select {
case <-doSomething():
case <-ctx.Done(): // closes when ctx is cancelled or its deadline passes
	return ctx.Err()
}
```

Two extra pieces of syntax worth naming: **`defer`** schedules a call to run
right before the enclosing function returns, however it returns — Go's answer
to a `finally`/context-manager cleanup, written inline where the resource is
acquired. **`select`** waits on multiple channels at once and runs whichever
fires first — mostly met in exactly this "race real work against `ctx.Done()`"
shape.

## 10. Goroutines (kept light)

`go someFunc()` starts `someFunc` running concurrently; execution continues to
the next line immediately, without waiting.

```go
var wg sync.WaitGroup
for i := 1; i <= 3; i++ {
	wg.Add(1)
	go func(n int) {
		defer wg.Done()
		fmt.Println("worker", n)
	}(i)
}
wg.Wait() // blocks until all three call Done()
```

You will rarely hand-write goroutines *inside* `Reconcile` — controller-runtime
already runs a pool of workers calling `Reconcile` concurrently and manages its
own work queue. Recognize `go func() { ... }()` when it shows up (e.g. a
background task started in `main.go`) rather than expecting to write it.

## Reading a real reconcile loop

Everything above, wearing operator clothes — this is close to what
`kubebuilder create api` generates:

```go
package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	flukev1alpha1 "github.com/hassanshabbirahmed/fluke-operator/api/v1alpha1"
)

// A struct (Concept 4) holding this controller's dependencies.
type FlukeReconciler struct {
	client.Client // an embedded interface field — FlukeReconciler gains its methods
}

// A method (Concept 6) with a POINTER receiver (Concept 5): reconciling can't
// touch cluster state through a copy. Its signature — ctx first (Concept 9),
// returns (Result, error) (Concept 3) — is exactly what the Reconciler
// interface requires. Nothing declares the relationship; having this one
// method with this exact shape IS what makes it true.
func (r *FlukeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// req is a struct (Concept 4) holding Namespace + Name.

	var fluke flukev1alpha1.Fluke
	if err := r.Get(ctx, req.NamespacedName, &fluke); err != nil {
		// &fluke — a pointer (Concept 5), because Get must WRITE into it.
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil // deleted — nothing to do, not an error
		}
		return ctrl.Result{}, err // (Concept 3): zero Result + the real error
	}

	// fluke.Spec.Replicas — dot-chaining into nested structs (Concept 4).
	var dep appsv1.Deployment
	err := r.Get(ctx, req.NamespacedName, &dep)
	if apierrors.IsNotFound(err) {
		dep = buildDeployment(&fluke)
		return ctrl.Result{}, r.Create(ctx, &dep)
	} else if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
```

## What's next

Lesson 2 (ticket [Set up the local Kubernetes environment](https://github.com/hassanshabbirahmed/fluke-operator/issues/5))
gets a real cluster running. Lesson 3 scaffolds this project for real with
`kubebuilder create api`, and the generated `Reconcile` will read almost
exactly like the walkthrough above.
