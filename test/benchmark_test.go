package test

import (
	"io"
	"net/http"
	"testing"

	"github.com/curtisnewbie/benchmarker"
)

func TestStartBenchmark(t *testing.T) {
	unknownErr := func(err error) benchmarker.Result {
		return benchmarker.Result{
			HttpStatus: 0,
			Success:    false,
			Extra: map[string]any{
				"ERROR": err.Error(),
			},
		}
	}

	sendRequest := func(c *http.Client) benchmarker.Result {
		req, err := http.NewRequest(http.MethodGet, "http://localhost:80", nil)
		if err != nil {
			return unknownErr(err)
		}
		r, err := c.Do(req)
		if err != nil {
			return unknownErr(err)
		}
		defer r.Body.Close()

		_, err = io.ReadAll(r.Body)
		if err != nil {
			return unknownErr(err)
		}

		return benchmarker.Result{
			HttpStatus: r.StatusCode,
			Success:    r.StatusCode == 200,
		}
	}
	concurrent := 10
	round := 3
	benchmarker.StartBenchmark(concurrent, round, sendRequest)
}
