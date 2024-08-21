package benchmarker

import (
	"fmt"
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
	BenchmarkClient                  = &http.Client{Timeout: 15 * time.Second}
	PlotWidth                        = 20 * vg.Inch
	PlotHeight                       = 10 * vg.Inch
	PlotSortedByRequestOrderFilename = "plots_sorted_by_request_order.png"
	PlotSortedByLatencyFilename      = "plots_sorted_by_latency.png"
)

func init() {
	largeNum := 100000
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = largeNum
	transport.MaxIdleConnsPerHost = largeNum
	transport.IdleConnTimeout = time.Minute * 30
	BenchmarkClient.Transport = transport
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

type SendRequestFunc func(c *http.Client) Result
type LogExtraStatFunc func([]Benchmark) string

func StartBenchmark(parallel int, round int, sendReqFunc SendRequestFunc, logStatFunc ...LogExtraStatFunc) {
	if round < 2 {
		round = 2
	}

	store := NewBenchmarkStore(parallel * (round - 1))
	pool := util.NewAsyncPool(parallel*2, parallel)
	order := -parallel // first round is only used to warmup.
	for i := 0; i < round; i++ {
		k := i
		aw := util.NewAwaitFutures[any](pool)
		for i := 0; i < parallel; i++ {
			order += 1
			j := order
			aw.SubmitAsync(func() (any, error) {
				triggerOnce(store, sendReqFunc, j, k > 0)
				return nil, nil
			})
		}
		aw.Await()
	}

	stats := PrintStats(store.bench, logStatFunc...)
	titleStats := fmt.Sprintf("(Total %d Requests, Max: %v, Min: %v, Avg: %v, Median: %v)", len(store.bench), stats.Max, stats.Min, stats.Avg, stats.Med)

	// SortOrder(store.bench) // already sorted by order in PrintStats(...)
	Plot(store.bench, stats.Min, stats.Max, "Request Latency Plots - Sorted By Request Order "+titleStats, PlotSortedByRequestOrderFilename)
	util.Printlnf("Generated plot graph: %v", PlotSortedByRequestOrderFilename)

	SortTime(store.bench)
	Plot(store.bench, stats.Min, stats.Max, "Request Latency Plots - Sorted By Latency "+titleStats, PlotSortedByLatencyFilename)
	util.Printlnf("Generated plot graph: %v", PlotSortedByLatencyFilename)
}

func triggerOnce(store *BenchmarkStore, send SendRequestFunc, order int, record bool) {
	start := time.Now()
	r := send(BenchmarkClient)
	took := time.Since(start)
	if !record {
		return
	}
	bench := Benchmark{
		Order:      order,
		Took:       took,
		Success:    r.Success,
		Extra:      r.Extra,
		HttpStatus: r.HttpStatus,
	}
	store.Add(bench)
}

type Benchmark struct {
	Order      int
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

func SortTime(bench []Benchmark) []Benchmark {
	sort.Slice(bench, func(i, j int) bool { return bench[i].Took < bench[j].Took })
	return bench
}

func SortOrder(bench []Benchmark) []Benchmark {
	sort.Slice(bench, func(i, j int) bool { return bench[i].Order < bench[j].Order })
	return bench
}

type Stats struct {
	Min time.Duration
	Max time.Duration
	Avg time.Duration
	Med time.Duration
}

func PrintStats(bench []Benchmark, logStatFunc ...LogExtraStatFunc) Stats {
	var (
		stats        Stats
		statusCount  = make(map[int]int, len(bench))
		successCount = make(map[bool]int, len(bench))
	)

	SortTime(bench) // sort by duration for calculating median
	if len(bench)%2 == 0 {
		stats.Med = (bench[len(bench)/2].Took + bench[len(bench)/2-1].Took) / 2
	} else {
		stats.Med = bench[len(bench)/2].Took
	}

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
		stats.Avg += b.Took
	}

	SortOrder(bench) // sort by request order for readability
	for _, b := range bench {
		util.Printlnf("Order: %d, Took: %v, Success: %v, HttpStatus: %d, Extra: %+v", b.Order, b.Took, b.Success, b.HttpStatus, b.Extra)
	}

	util.Printlnf("\n-------------------------------\n")
	util.Printlnf("total: %v", len(bench))
	util.Printlnf("min: %v", stats.Min)
	util.Printlnf("max: %v", stats.Max)
	util.Printlnf("median: %v", stats.Med)
	util.Printlnf("avg: %v", stats.Avg/time.Duration(len(bench)))
	util.Printlnf("status_count: %v", statusCount)
	util.Printlnf("success_count: %v", successCount)
	for _, f := range logStatFunc {
		output := f(bench)
		if output != "" {
			util.Printlnf(output)
		}
	}
	return stats
}

func Plot(bench []Benchmark, min time.Duration, max time.Duration, title string, name string) {
	p := plot.New()
	p.Title.Text = title
	p.X.Label.Text = "X"
	p.Y.Label.Text = "Request Latency (ms)"
	p.Y.Min = float64(min.Milliseconds())

	data := ToXYs(bench)
	util.DebugPrintlnf(Debug, "plot data: %+v", data)

	err := plotutil.AddLinePoints(p, "Latency (ms)", data)
	util.Must(err)

	err = p.Save(PlotWidth, PlotHeight, name)
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
