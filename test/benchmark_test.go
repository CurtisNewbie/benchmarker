package test

import (
	"net/http"
	"testing"
	"time"

	"github.com/curtisnewbie/benchmarker"
)

func TestStartBenchmark(t *testing.T) {
	sendRequest := benchmarker.NewRequestSender(
		func() (*http.Request, error) {
			return http.NewRequest(http.MethodGet, "http://localhost:8080", nil)
		},
		func(buf []byte, statusCode int) benchmarker.Result {
			return benchmarker.Result{
				HttpStatus: statusCode,
				Success:    statusCode == 200,
			}
		})
	concurrent := 3
	round := 10
	_, _, _ = benchmarker.StartBenchmark(benchmarker.BenchmarkSpec{
		Concurrent:  concurrent,
		Round:       round,
		SendReqFunc: sendRequest,
	})
}

func TestStartBenchmarkDur(t *testing.T) {
	sendRequest := benchmarker.NewRequestSender(
		func() (*http.Request, error) {
			return http.NewRequest(http.MethodGet, "http://localhost:8080", nil)
		},
		func(buf []byte, statusCode int) benchmarker.Result {
			return benchmarker.Result{
				HttpStatus: statusCode,
				Success:    statusCode == 200,
			}
		})
	concurrent := 100
	_, _, _ = benchmarker.StartBenchmark(benchmarker.BenchmarkSpec{
		Concurrent:        concurrent,
		Duration:          time.Second * 5,
		SendReqFunc:       sendRequest,
		DisablePlotGraphs: false,
		DisableOutputFile: false,
		LogStatFunc: []benchmarker.LogExtraStatFunc{
			func([]benchmarker.Benchmark) string {
				return "some extra stuff"
			},
		},
	})
}
