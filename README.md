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
Order: 1, Took: 13.032791ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 2, Took: 14.089ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 3, Took: 15.168084ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 4, Took: 8.655709ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 5, Took: 11.956125ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 6, Took: 3.571167ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 7, Took: 15.091167ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 8, Took: 15.299834ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 9, Took: 11.925917ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 10, Took: 10.944333ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 11, Took: 7.628208ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 12, Took: 9.466166ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 13, Took: 12.553333ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 14, Took: 11.110625ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 15, Took: 7.528167ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 16, Took: 9.468917ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 17, Took: 12.566166ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 18, Took: 4.235625ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 19, Took: 13.418625ms, Success: true, HttpStatus: 200, Extra: map[]
Order: 20, Took: 13.499041ms, Success: true, HttpStatus: 200, Extra: map[]

--------- Count ---------------

total_count: 20
status_count: map[200:20]
success_count: map[true:20]

--------- Latency -------------

min: 3.571167ms
max: 15.299834ms
median: 11.941021ms
avg: 11.06045ms
p75: 13.418625ms
p90: 15.091167ms
p95: 15.168084ms
p99: 15.299834ms

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