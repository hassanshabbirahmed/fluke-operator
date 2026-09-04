package main

import "fmt"

// "type NAME struct { ... }" defines a brand-new type: a bundle of named fields,
// each with its own fixed type. Think Python @dataclass, or a dict with a locked shape.

// Field names starting with a CAPITAL letter are "exported" - visible to other
// packages. lowercase = private to this package. (Go's only visibility rule.)

type FlukeSpec struct {
	Replicas         int
	FlukesPerReplica int
	Image            string
}

type FlukeStatus struct {
	ReadyReplicas int
	Phase         string
}

// Real Kubernetes resources nest a Spec (what you want) and a Status (what is).
// You'll see this exact shape when we design the Fluke CRD.
type Fluke struct {
	Name   string
	Spec   FlukeSpec
	Status FlukeStatus
}

func main() {
	// Build one with a "composite literal": Type{Field: value, ...}
	f := Fluke{
		Name: "demo",
		Spec: FlukeSpec{
			Replicas:         3,
			FlukesPerReplica: 2,
			Image:            "busybox",
		},
		// Status left out entirely -> it's the zero value: ReadyReplicas 0, Phase ""
	}

	// Access fields with a dot.
	fmt.Printf("%s wants %d replicas x %d flukes each\n",
		f.Name, f.Spec.Replicas, f.Spec.FlukesPerReplica)
	fmt.Printf("status: %d ready, phase %q\n",
		f.Status.ReadyReplicas, f.Status.Phase)

	// %+v prints a struct with field names - great for debugging.
	fmt.Printf("whole thing: %+v\n", f)
}
