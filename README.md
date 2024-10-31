# benchmarker

## Demo

```golang
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
	benchmarker.StartBenchmark(benchmarker.BenchmarkSpec{
		Concurrent:  concurrent,
		Round:       round,
		SendReqFunc: sendRequest,
	})
}
```

If you need CLI support:

```golang
func main() {
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
	benchmarker.StartBenchmarkCli(benchmarker.BenchmarkSpec{
		SendReqFunc: sendRequest,
	})
}
```

Then run it as follows:

```sh
# concurrency 3, duration 10 seconds
go run main.go -dur 10s -conc 3
```

## Output

```
Timestamp: 1724394471847895, Took: 689.375µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471847912, Took: 1.699167ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471847925, Took: 1.532291ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471848585, Took: 1.543875ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471849457, Took: 531.583µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471849612, Took: 1.028917ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471849989, Took: 790.208µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471850129, Took: 1.04575ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471850641, Took: 1.24525ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471850780, Took: 1.898375ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471851175, Took: 3.458209ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471851887, Took: 2.706709ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471852678, Took: 1.3715ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471854050, Took: 1.595041ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471854594, Took: 1.292333ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471854633, Took: 1.7755ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471855646, Took: 925µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471855886, Took: 440.833µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471856327, Took: 312.25µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471856409, Took: 660.333µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471856571, Took: 530.708µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471856640, Took: 711.042µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471857069, Took: 1.661875ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471857102, Took: 1.049875ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471857351, Took: 601.792µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471857953, Took: 746.542µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471858152, Took: 1.403916ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471858731, Took: 517.708µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471859249, Took: 710.042µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724394471859959, Took: 480.625µs, Success: true, HttpStatus: 200, Extra: map[]


--------- Brief ---------------

total_time: 12.547042ms
total_requests: 30
throughput: 2391 req/sec
concurrency: 3
rounds (for each worker): 10
status_count: map[200:30]
success_count: map[true:30]

--------- Latency -------------

min: 312.25µs
max: 3.458209ms
median: 1.037333ms
avg: 1.16522ms
p75: 1.543875ms
p90: 1.7755ms
p95: 2.706709ms
p99: 3.458209ms

--------- Data ----------------

data file: benchmark_records.txt

--------- Plots ---------------

Generated plot graph: plots_sorted_by_request_order.png
Generated plot graph: plots_sorted_by_latency.png

-------------------------------
```

## Plots

`plots_sorted_by_latency.png`

<img src="./demo/plots_sorted_by_latency.png" height="500px" />

`plots_sorted_by_request_order.png`

<img src="./demo/plots_sorted_by_request_order.png" height="500px" />
