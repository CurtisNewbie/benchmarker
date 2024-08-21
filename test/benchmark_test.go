package test

import (
	"net/http"
	"testing"

	"github.com/curtisnewbie/benchmarker"
)

func TestStartBenchmark(t *testing.T) {
	sendRequest := func(c *http.Client) benchmarker.Result {
		req, err := http.NewRequest(http.MethodGet, "http://localhost:80", nil)
		if err != nil {
			return benchmarker.Result{
				HttpStatus: 0,
				Success:    false,
			}
		}
		r, err := c.Do(req)
		if err != nil {
			return benchmarker.Result{
				HttpStatus: 0,
				Success:    false,
			}
		}
		defer r.Body.Close()

		return benchmarker.Result{
			HttpStatus: r.StatusCode,
			Success:    r.StatusCode == 200,
		}
	}
	concurrent := 10
	round := 3
	benchmarker.StartBenchmark(concurrent, round, sendRequest)
}
