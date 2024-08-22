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
	benchmarker.StartBenchmark(concurrent, round, sendRequest)
}
```

## Output

```
Timestamp: 1724315849189229, Took: 3.671291ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849192901, Took: 2.508584ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849193780, Took: 1.6275ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849193814, Took: 2.531166ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849195409, Took: 1.277083ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849195410, Took: 5.626291ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849196346, Took: 4.750209ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849196686, Took: 4.349958ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849201037, Took: 1.441166ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849201038, Took: 716.791µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849201096, Took: 1.389583ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849201755, Took: 1.171667ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849202479, Took: 2.215375ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849202486, Took: 888.375µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849202930, Took: 1.899708ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849203375, Took: 2.326917ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849204695, Took: 1.014625ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849204830, Took: 610.041µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849205440, Took: 1.134791ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849205703, Took: 924.459µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849205710, Took: 800.125µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849206511, Took: 582.792µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849206576, Took: 1.000708ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849206627, Took: 814µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849207094, Took: 1.107208ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849207442, Took: 628.75µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849207577, Took: 2.008667ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849208071, Took: 598.375µs, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849208202, Took: 1.131833ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724315849208670, Took: 444µs, Success: true, HttpStatus: 200, Extra: map[]


--------- Brief ---------------

total_requests: 30
concurrency: 3
round: 10
status_count: map[200:30]
success_count: map[true:30]

--------- Latency -------------

min: 444µs
max: 5.626291ms
median: 1.153229ms
avg: 1.706401ms
p75: 2.215375ms
p90: 3.671291ms
p95: 4.750209ms
p99: 5.626291ms

--------- Data ----------------

data file: data_output.txt

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
