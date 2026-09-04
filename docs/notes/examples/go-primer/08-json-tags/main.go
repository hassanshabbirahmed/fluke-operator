package main

import (
	"encoding/json"
	"fmt"
)

// The string in backticks after each field is a "struct tag".
// `json:"name"` tells encoding/json what key to use instead of the Go field name.
// ",omitempty" means: leave this out entirely if it's the zero value.
type FlukeSpec struct {
	Replicas         int    `json:"replicas"`
	FlukesPerReplica int    `json:"flukesPerReplica"`
	Image            string `json:"image,omitempty"`
}

func main() {
	spec := FlukeSpec{Replicas: 3, FlukesPerReplica: 2}
	// Image left unset -> "" -> zero value -> omitempty drops it entirely

	// Marshal: Go struct -> JSON bytes. Kubernetes's YAML is just JSON's cousin;
	// the CRD machinery you'll use later works exactly like this under the hood.
	out, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		panic(err) // marshaling a plain struct like this basically never fails
	}
	fmt.Println(string(out))

	// Unmarshal: JSON bytes -> Go struct (e.g. reading a CR from the API server).
	incoming := []byte(`{"replicas": 5, "flukesPerReplica": 1, "image": "redis"}`)
	var parsed FlukeSpec
	if err := json.Unmarshal(incoming, &parsed); err != nil { // note: &parsed - json.Unmarshal needs a POINTER to fill in
		panic(err)
	}
	fmt.Printf("parsed: %+v\n", parsed)
}
