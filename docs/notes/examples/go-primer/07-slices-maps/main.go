package main

import "fmt"

func main() {
	// SLICES - like a Python list
	images := []string{"busybox", "nginx"}
	images = append(images, "redis") // must reassign - append returns a (maybe new) slice

	for i, img := range images {
		fmt.Printf("image[%d] = %s\n", i, img)
	}
	fmt.Println("len:", len(images))

	// MAPS - like a Python dict
	replicaCounts := map[string]int{
		"demo": 3,
	}
	replicaCounts["prod"] = 5

	// "comma ok" - the idiomatic way to check membership without a KeyError
	count, ok := replicaCounts["staging"]
	fmt.Println("staging:", count, "present:", ok) // 0 false - key absent

	count, ok = replicaCounts["demo"]
	fmt.Println("demo:", count, "present:", ok) // 3 true

	// ranging a map (order is NOT guaranteed - unlike a Python dict since 3.7)
	for name, n := range replicaCounts {
		_ = name
		_ = n
	}
	fmt.Println("total flukes tracked:", len(replicaCounts))
}
