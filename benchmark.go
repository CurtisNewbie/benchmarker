package benchmarker

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/curtisnewbie/miso/util"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

var (
	Debug                            bool
	PlotWidth                        = 20 * vg.Inch
	PlotHeight                       = 10 * vg.Inch
	PlotSortedByRequestOrderFilename = "plots_sorted_by_request_order.png"
	PlotSortedByLatencyFilename      = "plots_sorted_by_latency.png"
	DataOutputFilename               = "data_output.txt"
)

func newClient() *http.Client {
	c := &http.Client{
		Timeout: 10 * time.Second,
	}
	c.Transport = &http.Transport{
		DisableKeepAlives: false,
	}
	return c
}

type BenchmarkStore struct {
	bench []Benchmark
	mu    sync.Mutex
}

func (b *BenchmarkStore) Add(bm Benchmark) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.bench = append(b.bench, bm)
}

func NewBenchmarkStore(cnt int) *BenchmarkStore {
	return &BenchmarkStore{
		bench: make([]Benchmark, 0, cnt),
	}
}

type BuildRequestFunc func() (*http.Request, error)
type ParseResponseFunc func(buf []byte, statusCode int) Result

func NewRequestSender(buildReq BuildRequestFunc, parseRes ParseResponseFunc) SendRequestFunc {
	errResult := func(err error, httpStatus int) Result {
		return Result{
			HttpStatus: httpStatus,
			Success:    false,
			Extra: map[string]any{
				"ERROR": err.Error(),
			},
		}
	}

	return func(c *http.Client) Result {
		req, err := buildReq()
		if err != nil {
			return errResult(err, 0)
		}

		res, err := c.Do(req)
		if err != nil {
			return errResult(err, res.StatusCode)
		}

		buf, err := io.ReadAll(res.Body)
		if err != nil {
			return errResult(err, res.StatusCode)
		}

		return parseRes(buf, res.StatusCode)
	}
}

type SendRequestFunc func(c *http.Client) Result
type LogExtraStatFunc func([]Benchmark) string

func StartBenchmark(concurrent int, round int, sendReqFunc SendRequestFunc, logStatFunc ...LogExtraStatFunc) {
	if round < 1 {
		round = 1
	}
	round += 1 // for warmup

	store := NewBenchmarkStore(concurrent * (round - 1))
	pool := util.NewAsyncPool(concurrent, concurrent)
	aw := util.NewAwaitFutures[any](pool)

	for i := 0; i < concurrent; i++ {
		aw.SubmitAsync(func() (any, error) {
			client := newClient()
			for j := 0; j < round; j++ {
				triggerOnce(client, store, sendReqFunc, j > 0)
			}
			return nil, nil
		})
	}
	aw.Await()

	stats := PrintStats(concurrent, round, store.bench, logStatFunc...)
	titleStats := fmt.Sprintf("(Total %d Requests, Concurrency: %v, Max: %v, Min: %v, Avg: %v, Median: %v)", len(store.bench), concurrent,
		stats.Max, stats.Min, stats.Avg, stats.Med)
	util.Printlnf("\n--------- Plots ---------------\n")

	SortTimestamp(store.bench) // already sorted by order in PrintStats(...)
	Plot(store.bench, stats.Min, stats.Max, "Request Latency Plots - Sorted By Request Timestamp "+titleStats,
		"X - Sorted By Request Timestamp", PlotSortedByRequestOrderFilename)
	util.Printlnf("Generated plot graph: %v", PlotSortedByRequestOrderFilename)

	SortTook(store.bench)
	Plot(store.bench, stats.Min, stats.Max, "Request Latency Plots - Sorted By Latency "+titleStats,
		"X - Sorted By Latency", PlotSortedByLatencyFilename)
	util.Printlnf("Generated plot graph: %v", PlotSortedByLatencyFilename)
	util.Printlnf("\n-------------------------------\n")
}

func triggerOnce(client *http.Client, store *BenchmarkStore, send SendRequestFunc, record bool) {
	timestamp := time.Now().UnixMicro()
	start := time.Now()
	r := send(client)
	took := time.Since(start)
	if !record {
		return // warmup only
	}
	bench := Benchmark{
		Timestamp:  timestamp,
		Took:       took,
		Success:    r.Success,
		Extra:      r.Extra,
		HttpStatus: r.HttpStatus,
	}
	store.Add(bench)
}

type Benchmark struct {
	Timestamp  int64
	Took       time.Duration
	Success    bool
	Extra      map[string]any
	HttpStatus int
}

type Result struct {
	HttpStatus int
	Success    bool
	Extra      map[string]any
}

func SortTook(bench []Benchmark) []Benchmark {
	sort.Slice(bench, func(i, j int) bool { return bench[i].Took < bench[j].Took })
	return bench
}

func SortTimestamp(bench []Benchmark) []Benchmark {
	sort.Slice(bench, func(i, j int) bool { return bench[i].Timestamp < bench[j].Timestamp })
	return bench
}

type Stats struct {
	Min time.Duration
	Max time.Duration
	Avg time.Duration
	Med time.Duration
}

func PrintStats(concurrent int, round int, bench []Benchmark, logStatFunc ...LogExtraStatFunc) Stats {
	var (
		sum          time.Duration
		stats        Stats
		statusCount  = make(map[int]int, len(bench))
		successCount = make(map[bool]int, len(bench))
	)

	SortTook(bench) // sort by duration for calculating median
	if len(bench)%2 == 0 {
		stats.Med = (bench[len(bench)/2].Took + bench[len(bench)/2-1].Took) / 2
	} else {
		stats.Med = bench[len(bench)/2].Took
	}

	f, err := util.ReadWriteFile(DataOutputFilename)
	_ = f.Truncate(0)
	util.Must(err)
	for i, b := range bench {
		if i == 0 {
			stats.Min = b.Took
			stats.Max = b.Took
		} else {
			if b.Took > stats.Max {
				stats.Max = b.Took
			}
			if b.Took < stats.Min {
				stats.Min = b.Took
			}
		}
		statusCount[b.HttpStatus]++
		successCount[b.Success]++
		sum += b.Took
	}
	stats.Avg = sum / time.Duration(len(bench))

	SortTimestamp(bench) // sort by request order for readability
	for _, b := range bench {
		f.WriteString(fmt.Sprintf("Timestamp: %d, Took: %v, Success: %v, HttpStatus: %d, Extra: %+v\n", b.Timestamp,
			b.Took, b.Success, b.HttpStatus, b.Extra))
	}

	SortTook(bench)

	util.Printlnf("\n--------- Brief ---------------\n")
	util.Printlnf("total_requests: %v", concurrent*(round-1))
	util.Printlnf("concurrency: %v", concurrent)
	util.Printlnf("round: %v", round-1)
	util.Printlnf("status_count: %v", statusCount)
	util.Printlnf("success_count: %v", successCount)
	util.Printlnf("\n--------- Latency -------------\n")
	util.Printlnf("min: %v", stats.Min)
	util.Printlnf("max: %v", stats.Max)
	util.Printlnf("median: %v", stats.Med)
	util.Printlnf("avg: %v", stats.Avg)
	util.Printlnf("p75: %v", percentile(bench, 75))
	util.Printlnf("p90: %v", percentile(bench, 90))
	util.Printlnf("p95: %v", percentile(bench, 95))
	util.Printlnf("p99: %v", percentile(bench, 99))
	util.Printlnf("\n--------- Data ----------------\n")
	util.Printlnf("data file: %v", DataOutputFilename)

	if len(logStatFunc) > 0 {
		util.Printlnf("\n--------- Extra ---------------\n")
		for _, f := range logStatFunc {
			output := f(bench)
			if output != "" {
				util.Printlnf(output)
			}
		}
	}
	return stats
}

func Plot(bench []Benchmark, min time.Duration, max time.Duration, title string, xlabel string, fname string) {
	p := plot.New()
	p.Title.Text = "\n" + title
	p.X.Label.Text = "\n" + xlabel + "\n"
	p.Y.Label.Text = "\nRequest Latency (ms)\n"
	p.Y.Min = float64(min.Milliseconds())

	data := ToXYs(bench)
	util.DebugPrintlnf(Debug, "plot data: %+v", data)

	err := plotutil.AddLinePoints(p, "Latency (ms)", data)
	util.Must(err)

	err = p.Save(PlotWidth, PlotHeight, fname)
	util.Must(err)
}

func ToXYs(bench []Benchmark) plotter.XYs {
	pts := make(plotter.XYs, 0, len(bench))
	for i := range bench {
		pts = append(pts, plotter.XY{
			X: float64(i),
			Y: float64(bench[i].Took.Milliseconds()),
		})
	}
	return pts
}

func percentile(bench []Benchmark, percentile float64) time.Duration {
	idx := math.Ceil(percentile / 100.0 * float64(len(bench)))
	return bench[int(idx)-1].Took
}
