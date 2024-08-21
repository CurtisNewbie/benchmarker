# benchmarker

## Demo

```golang
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
```

## Output

```
Timestamp: 1724233863043200, Took: 30.068917ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863046793, Took: 10.990917ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863052979, Took: 17.169125ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863057784, Took: 25.15925ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863057821, Took: 19.340375ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863061849, Took: 10.376875ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863062767, Took: 23.098583ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863062772, Took: 23.056292ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863065420, Took: 5.681333ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863070149, Took: 27.529833ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863071101, Took: 22.903083ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863072227, Took: 18.088833ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863073270, Took: 14.90025ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863077162, Took: 15.736166ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863078156, Took: 24.156916ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863082944, Took: 26.08575ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863085828, Took: 7.092625ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863085866, Took: 12.902042ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863086884, Took: 7.136209ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863088171, Took: 13.030667ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863090316, Took: 9.655708ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863092898, Took: 9.403916ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863092921, Took: 17.272958ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863094005, Took: 12.813875ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863094020, Took: 16.179666ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863097679, Took: 11.385458ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863098768, Took: 9.167917ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863102314, Took: 6.761667ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863109076, Took: 3.37025ms, Success: true, HttpStatus: 200, Extra: map[]
Timestamp: 1724233863110200, Took: 3.3235ms, Success: true, HttpStatus: 200, Extra: map[]


--------- Brief ---------------

concurrency: 10
round: 3
status_count: map[200:30]
success_count: map[true:30]

--------- Latency -------------

min: 3.3235ms
max: 30.068917ms
median: 13.965458ms
avg: 15.127965ms
p75: 22.903083ms
p90: 25.15925ms
p95: 27.529833ms
p99: 30.068917ms

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
