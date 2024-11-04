package test

import (
	"net/http"
	"testing"
	"time"

	"github.com/curtisnewbie/benchmarker"
)

func TestStartBenchmark(t *testing.T) {
	_, _, _ = benchmarker.StartBenchmark(benchmarker.BenchmarkSpec{
		Concurrent: 3,
		Round:      10,
		BuildReqFunc: func() (*http.Request, error) {
			return http.NewRequest(http.MethodGet, "http://localhost:8080", nil)
		},
	})
}

func TestStartBenchmarkDur(t *testing.T) {
	_, _, _ = benchmarker.StartBenchmark(benchmarker.BenchmarkSpec{
		Concurrent:        100,
		Duration:          time.Second * 5,
		DisablePlotGraphs: false,
		DisableOutputFile: false,
		LogStatFunc: []benchmarker.LogExtraStatFunc{
			func([]benchmarker.Benchmark) string {
				return "some extra stuff"
			},
		},
		BuildReqFunc: func() (*http.Request, error) {
			return http.NewRequest(http.MethodGet, "http://localhost:8080", nil)
		},
	})
}

func TestStartBenchmarkCli(t *testing.T) {
	_, err := benchmarker.StartBenchmarkCli(benchmarker.BenchmarkSpec{
		BuildReqFunc: func() (*http.Request, error) {
			return http.NewRequest(http.MethodGet, "http://localhost:8080", nil)
		},
	})
	if err != nil {
		panic(err)
	}
}
