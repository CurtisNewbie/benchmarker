package main

import (
	"github.com/curtisnewbie/benchmarker"
)

func main() {
	_, err := benchmarker.StartBenchmarkCmd()
	if err != nil {
		panic(err)
	}
}
