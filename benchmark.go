package benchmarker

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/curtisnewbie/miso/util"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

var (
	Debug                            = false
	GeneratePlots                    = true
	PlotWidth                        = 20 * vg.Inch
	PlotHeight                       = 10 * vg.Inch
	PlotSortedByRequestOrderFilename = "plots_sorted_by_request_order.png"
	PlotSortedByLatencyFilename      = "plots_sorted_by_latency.png"
	PlotInclMinMaxLabels             = true
	PlotInclPercentileLines          = true
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
			if res != nil {
				return errResult(err, res.StatusCode)
			}
			return errResult(err, 0)
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

	store := NewBenchmarkStore(concurrent * round)
	pool := util.NewAsyncPool(concurrent, concurrent)
	aw := util.NewAwaitFutures[any](pool)
	var warmupWg sync.WaitGroup // for warmup
	warmupWg.Add(concurrent)

	var startSet int32 = 0
	var startTime time.Time
	util.DebugPrintlnf(Debug, "Creating workers: %v", time.Now())

	for i := 0; i < concurrent; i++ {
		wi := i
		aw.SubmitAsync(func() (any, error) {
			client := newClient()
			triggerOnce(client, store, sendReqFunc, false)
			warmupWg.Done()
			warmupWg.Wait() // synchronize all of them

			if atomic.CompareAndSwapInt32(&startSet, 0, 1) {
				startTime = time.Now()
			}
			util.DebugPrintlnf(Debug, "Worker-%d start ramping: %v", wi, time.Now())

			for j := 0; j < round; j++ {
				triggerOnce(client, store, sendReqFunc, true)
			}
			return nil, nil
		})
	}
	aw.Await()
	endTime := time.Now()
	util.DebugPrintlnf(Debug, "Benchmark endTime: %v", endTime)

	stats := PrintStats(concurrent, round, store.bench, endTime.Sub(startTime), logStatFunc...)
	titleStats := fmt.Sprintf("(Total %d Requests, Concurrency: %v, Max: %v, Min: %v, Avg: %v, Median: %v, p75: %v, p90: %v, p95: %v, p99: %v)",
		len(store.bench), concurrent, stats.Max, stats.Min, stats.Avg, stats.Med, stats.P75.Took, stats.P90.Took, stats.P95.Took, stats.P99.Took)

	if GeneratePlots {
		util.Printlnf("\n--------- Plots ---------------\n")
		SortTimestamp(store.bench) // already sorted by order in PrintStats(...)
		Plot(store.bench, stats, "Request Latency Plots - Sorted By Request Timestamp "+titleStats,
			"X - Sorted By Request Timestamp", PlotSortedByRequestOrderFilename, false)
		util.Printlnf("Generated plot graph: %v", PlotSortedByRequestOrderFilename)

		SortTook(store.bench)
		Plot(store.bench, stats, "Request Latency Plots - Sorted By Latency "+titleStats,
			"X - Sorted By Latency", PlotSortedByLatencyFilename, true)
		util.Printlnf("Generated plot graph: %v", PlotSortedByLatencyFilename)
	}
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
	Min      time.Duration
	Max      time.Duration
	Avg      time.Duration
	Med      time.Duration
	P99      Benchmark
	P95      Benchmark
	P90      Benchmark
	P75      Benchmark
	P99Index int
	P95Index int
	P90Index int
	P75Index int
}

func PrintStats(concurrent int, round int, bench []Benchmark, totalTime time.Duration, logStatFunc ...LogExtraStatFunc) Stats {
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
	totalReq := concurrent * round
	util.Printlnf("\n--------- Brief ---------------\n")
	util.Printlnf("total_time: %v", totalTime)
	util.Printlnf("total_requests: %v", totalReq)
	util.Printlnf("throughput: %.0f req/sec", float64(totalReq)/(float64(totalTime)/float64(time.Second)))
	util.Printlnf("concurrency: %v", concurrent)
	util.Printlnf("round: %v", round)
	util.Printlnf("status_count: %v", statusCount)
	util.Printlnf("success_count: %v", successCount)
	util.Printlnf("\n--------- Latency -------------\n")
	util.Printlnf("min: %v", stats.Min)
	util.Printlnf("max: %v", stats.Max)
	util.Printlnf("median: %v", stats.Med)
	util.Printlnf("avg: %v", stats.Avg)

	p75, p75i := percentile(bench, 75)
	p90, p90i := percentile(bench, 90)
	p95, p95i := percentile(bench, 95)
	p99, p99i := percentile(bench, 99)
	stats.P75 = p75
	stats.P90 = p90
	stats.P95 = p95
	stats.P99 = p99
	stats.P75Index = p75i
	stats.P90Index = p90i
	stats.P95Index = p95i
	stats.P99Index = p99i
	util.Printlnf("p75: %v", p75.Took)
	util.Printlnf("p90: %v", p90.Took)
	util.Printlnf("p95: %v", p95.Took)
	util.Printlnf("p99: %v", p99.Took)
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

func Plot(bench []Benchmark, stat Stats, title string, xlabel string, fname string, drawPercentile bool) {
	p := plot.New()
	p.Title.Text = "\n" + title
	p.X.Label.Text = "\n" + xlabel + "\n"
	p.Y.Label.Text = "\nRequest Latency (ms)\n"
	p.Y.Min = float64(stat.Min.Milliseconds())
	if p.Y.Min > 10 {
		p.Y.Min -= 10
	} else {
		p.Y.Min = 0
	}
	p.Y.Max = float64(stat.Max.Milliseconds()) + 10

	data := ToXYs(bench)
	util.DebugPrintlnf(Debug, "plot data: %+v", data)

	err := plotutil.AddLinePoints(p, "Latency (ms)", data)
	util.Must(err)

	if PlotInclPercentileLines && drawPercentile {
		drawPercentileLine(p, stat.P99Index, "P99", 1)
		drawPercentileLine(p, stat.P95Index, "P95", 2)
		drawPercentileLine(p, stat.P90Index, "P90", 3)
		drawPercentileLine(p, stat.P75Index, "P75", 4)
	}

	if PlotInclMinMaxLabels {
		labels := make([]string, len(bench))
		for i, b := range bench {
			if b.Took >= stat.Max {
				labels[i] = b.Took.String()
			} else if b.Took <= stat.Min {
				labels[i] = b.Took.String()
			}
		}
		bl, err := plotter.NewLabels(plotter.XYLabels{
			XYs:    data,
			Labels: labels,
		})
		if err == nil {
			p.Add(bl)
		}
	}

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

func percentile(bench []Benchmark, percentile float64) (Benchmark, int) {
	idx := math.Ceil(percentile / 100.0 * float64(len(bench)))
	i := int(idx) - 1
	return bench[i], i
}

func drawPercentileLine(p *plot.Plot, index int, label string, color int) {
	xys := make(plotter.XYs, 2)
	xys[0] = plotter.XY{X: float64(index), Y: 0}
	lineTop := p.Y.Max
	if lineTop > 1 {
		lineTop -= 1
	} else if lineTop > .5 {
		lineTop -= .5
	}
	xys[1] = plotter.XY{X: float64(index), Y: lineTop}

	line, err := plotter.NewLine(xys)
	if err == nil {
		line.LineStyle.Color = plotutil.Color(color)
		p.Add(line)

		xys[1].Y += 0.1
		lables := make([]string, 2)
		lables[1] = label
		lineLabels, err := plotter.NewLabels(plotter.XYLabels{
			XYs:    xys,
			Labels: lables,
		})
		if err == nil {
			p.Add(lineLabels)
		} else {
			util.Printlnf("failed to add percentile line labels, %v", err)
		}
	} else {
		util.Printlnf("failed to add percentile line, %v", err)
	}
}
