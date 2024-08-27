package test

import (
	"net/http"
	"testing"

	"github.com/curtisnewbie/benchmarker"
)

func TestStartBenchmark(t *testing.T) {
	sendRequest := benchmarker.NewRequestSender(
		func() (*http.Request, error) {
			return http.NewRequest(http.MethodGet, "http://localhost:80", nil)
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
