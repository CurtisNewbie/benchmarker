# benchmarker

## Demo

```golang
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
parallel := 10
round := 3
benchmarker.StartBenchmark(parallel, round, sendRequest)
```

## Output

```
Order: 1, Took: 1.159167ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 2, Took: 1.759167ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 3, Took: 1.787917ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 4, Took: 2.687959ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 5, Took: 3.182459ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 6, Took: 2.618958ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 7, Took: 3.118584ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 8, Took: 2.754875ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 9, Took: 2.756875ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 10, Took: 2.492708ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 11, Took: 1.154667ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 12, Took: 1.9035ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 13, Took: 867µs, Success: true, HttpStatus: 200, Extra: map[]
Order: 14, Took: 1.8965ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 15, Took: 679.583µs, Success: true, HttpStatus: 200, Extra: map[]
Order: 16, Took: 1.320791ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 17, Took: 1.445917ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 18, Took: 1.75275ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 19, Took: 1.003042ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 20, Took: 588.875µs, Success: true, HttpStatus: 200, Extra: map[]
Order: 21, Took: 1.369375ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 22, Took: 810.708µs, Success: true, HttpStatus: 200, Extra: map[]
Order: 23, Took: 1.468792ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 24, Took: 954.75µs, Success: true, HttpStatus: 200, Extra: map[]
Order: 25, Took: 612.208µs, Success: true, HttpStatus: 200, Extra: map[]
Order: 26, Took: 1.090875ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 27, Took: 1.958625ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 28, Took: 1.61025ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 29, Took: 1.376125ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 30, Took: 538.625µs, Success: true, HttpStatus: 200, Extra: map[]

--------- Count ---------------

total_count: 30
status_count: map[200:30]
success_count: map[true:30]

--------- Latency -------------

min: 538.625µs
max: 3.182459ms
median: 1.457354ms
avg: 1.624054ms
p75: 1.958625ms
p90: 2.754875ms
p95: 3.118584ms
p99: 3.182459ms

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