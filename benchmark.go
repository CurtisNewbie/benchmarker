package benchmarker

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/curtisnewbie/miso/util"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

var (
	defPlotWidth                        = 20 * vg.Inch
	defPlotHeight                       = 10 * vg.Inch
	defPlotSortedByRequestOrderFilename = "plots_sorted_by_request_order.png"
	defPlotSortedByLatencyFilename      = "plots_sorted_by_latency.png"
	defDataOutputFilename               = "benchmark_records.txt"
)

const (
	workerQueueSize = 1000 // rough estimate
)

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

type BenchmarkSpec struct {
	Concurrent  int
	Round       int
	Duration    time.Duration
	SendReqFunc SendRequestFunc
	LogStatFunc []LogExtraStatFunc

	DebugLog                       bool
	DisablePlotGraphs              bool
	DisablePlotInclMinMaxLabels    bool
	DisablePlotInclPercentileLines bool
	DisableOutputFile              bool

	PlotWidth                        font.Length
	PlotHeight                       font.Length
	PlotSortedByRequestOrderFilename string
	PlotSortedByLatencyFilename      string

	DataOutputFilename string
}

func StartBenchmark(spec BenchmarkSpec) ([]Benchmark, Stats, error) {
	durBased := false
	if spec.Duration > 0 {
		durBased = true
	} else {
		if spec.Round < 1 {
			spec.Round = 1
		}
	}

	if spec.PlotWidth == 0 {
		spec.PlotWidth = defPlotWidth
	}
	if spec.PlotHeight == 0 {
		spec.PlotHeight = defPlotHeight
	}
	if spec.PlotSortedByRequestOrderFilename == "" {
		spec.PlotSortedByRequestOrderFilename = defPlotSortedByRequestOrderFilename
	}
	if spec.PlotSortedByLatencyFilename == "" {
		spec.PlotSortedByLatencyFilename = defPlotSortedByLatencyFilename
	}
	if spec.DataOutputFilename == "" {
		spec.DataOutputFilename = defDataOutputFilename
	}

	pool := util.NewAsyncPool(spec.Concurrent, spec.Concurrent)
	aw := util.NewAwaitFutures[[]Benchmark](pool)

	var warmupWg sync.WaitGroup // for warmup
	warmupWg.Add(spec.Concurrent)

	var startTimeOnce sync.Once
	var startTime time.Time
	util.DebugPrintlnf(spec.DebugLog, "Creating workers: %v", time.Now())

	for i := 0; i < spec.Concurrent; i++ {
		wi := i
		aw.SubmitAsync(func() ([]Benchmark, error) {
			client := newClient()
			func() {
				defer warmupWg.Done()
				triggerOnce(client, spec.SendReqFunc)
			}()
			warmupWg.Wait() // synchronize all of them

			startTimeOnce.Do(func() { startTime = time.Now() })
			util.DebugPrintlnf(spec.DebugLog, "Worker-%d start ramping: %v", wi, time.Now())

			var localStore []Benchmark
			if durBased {
				localStore = make([]Benchmark, 0, workerQueueSize)
			} else {
				localStore = make([]Benchmark, 0, spec.Round)
			}

			if durBased {
				for time.Since(startTime) <= spec.Duration {
					localStore = append(localStore, triggerOnce(client, spec.SendReqFunc))
				}
			} else {
				for j := 0; j < spec.Round; j++ {
					localStore = append(localStore, triggerOnce(client, spec.SendReqFunc))
				}
			}
			return localStore, nil
		})
	}

	var size int
	if !durBased {
		size = spec.Concurrent * spec.Round
	} else {
		size = spec.Concurrent * workerQueueSize
	}
	benchmarks := make([]Benchmark, 0, size)
	futures := aw.Await()
	for _, f := range futures {
		b, _ := f.Get()
		benchmarks = append(benchmarks, b...)
	}

	endTime := time.Now()
	util.DebugPrintlnf(spec.DebugLog, "Benchmark endTime: %v", endTime)

	stats, err := printStats(spec, benchmarks, endTime.Sub(startTime), spec.LogStatFunc...)
	if err != nil {
		return benchmarks, stats, err
	}

	percStr := strings.Builder{}
	keys := util.MapKeys(stats.Percentiles)
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	for _, pk := range keys {
		pv := stats.Percentiles[pk]
		if percStr.Len() > 0 {
			percStr.WriteString(", ")
		}
		percStr.WriteString(fmt.Sprintf("P%d: %v", pk, pv.Record.Took))
	}

	titleStats := fmt.Sprintf("(Total %d Requests, Concurrency: %v, Max: %v, Min: %v, Avg: %v, Median: %v, %v)",
		len(benchmarks), spec.Concurrent, stats.Max, stats.Min, stats.Avg, stats.Med, percStr.String())

	if !spec.DisablePlotGraphs {
		util.Printlnf("\n--------- Plots ---------------\n")
		SortTimestamp(benchmarks) // already sorted by order in PrintStats(...)
		err := plotGraph(spec, benchmarks, stats, "Request Latency Plots - Sorted By Request Timestamp "+titleStats,
			"X - Sorted By Request Timestamp", spec.PlotSortedByRequestOrderFilename, false)
		if err != nil {
			return benchmarks, stats, err
		}

		util.Printlnf("Generated plot graph: %v", spec.PlotSortedByRequestOrderFilename)

		SortTook(benchmarks)
		err = plotGraph(spec, benchmarks, stats, "Request Latency Plots - Sorted By Latency "+titleStats,
			"X - Sorted By Latency", spec.PlotSortedByLatencyFilename, true)
		if err != nil {
			return benchmarks, stats, err
		}

		util.Printlnf("Generated plot graph: %v", spec.PlotSortedByLatencyFilename)
	}
	util.Printlnf("\n-------------------------------\n")

	return benchmarks, stats, nil
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

type Percentile struct {
	Record Benchmark
	Index  int // index of record when they are sorted by time duration.
}

type Stats struct {
	TotalTime     time.Duration
	TotalRequests int
	Throughput    float64
	StatusCount   map[int]int
	SuccessCount  map[bool]int
	Min           time.Duration
	Max           time.Duration
	Avg           time.Duration
	Med           time.Duration
	Percentiles   map[int]Percentile
}

func printStats(spec BenchmarkSpec, bench []Benchmark, totalTime time.Duration, logStatFunc ...LogExtraStatFunc) (Stats, error) {
	var (
		concurrent   = spec.Concurrent
		round        = spec.Round
		dur          = spec.Duration
		sum          time.Duration
		stats        Stats
		statusCount  = make(map[int]int, len(bench))
		successCount = make(map[bool]int, len(bench))
		total        = len(bench)
	)

	SortTook(bench) // sort by duration for calculating median
	if total > 0 {
		if total%2 == 0 {
			stats.Med = (bench[total/2].Took + bench[total/2-1].Took) / 2
		} else {
			stats.Med = bench[total/2].Took
		}
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
		sum += b.Took
	}

	if total > 0 {
		stats.Avg = sum / time.Duration(total)
	}

	SortTimestamp(bench) // sort by request order for readability

	if !spec.DisableOutputFile {
		f, err := util.ReadWriteFile(spec.DataOutputFilename)
		if err != nil {
			return stats, err
		}
		defer f.Close()
		_ = f.Truncate(0)
		for _, b := range bench {
			f.WriteString(fmt.Sprintf("Timestamp: %d, Took: %v, Success: %v, HttpStatus: %d, Extra: %+v\n", b.Timestamp,
				b.Took, b.Success, b.HttpStatus, b.Extra))
		}
	}

	SortTook(bench)
	util.Printlnf("\n--------- Brief ---------------\n")
	util.Printlnf("total_time: %v", totalTime)
	stats.TotalTime = totalTime
	util.Printlnf("total_requests: %v", total)
	stats.TotalRequests = total
	thrput := float64(total) / (float64(totalTime) / float64(time.Second))
	util.Printlnf("throughput: %.0f req/sec", thrput)
	stats.Throughput = thrput
	util.Printlnf("concurrency: %v", concurrent)
	if dur > 0 {
		util.Printlnf("duration: %v", dur)
	} else {
		util.Printlnf("rounds (for each worker): %v", round)
	}
	util.Printlnf("status_count: %v", statusCount)
	stats.StatusCount = statusCount
	util.Printlnf("success_count: %v", successCount)
	stats.SuccessCount = successCount
	util.Printlnf("\n--------- Latency -------------\n")
	util.Printlnf("min: %v", stats.Min)
	util.Printlnf("max: %v", stats.Max)
	util.Printlnf("median: %v", stats.Med)
	util.Printlnf("avg: %v", stats.Avg)

	stats.Percentiles = map[int]Percentile{}
	if total > 0 {
		for _, pv := range []int{75, 90, 95, 99} {
			stats.Percentiles[pv] = percentile(bench, float64(pv))
			util.Printlnf("P%d: %v", pv, stats.Percentiles[pv].Record.Took)
		}
	}

	if !spec.DisableOutputFile {
		util.Printlnf("\n--------- Data ----------------\n")
		util.Printlnf("data file: %v", spec.DataOutputFilename)
	}

	if len(logStatFunc) > 0 {
		util.Printlnf("\n--------- Extra ---------------\n")
		for _, f := range logStatFunc {
			output := f(bench)
			if output != "" {
				util.Printlnf(output)
			}
		}
	}
	return stats, nil
}

func triggerOnce(client *http.Client, send SendRequestFunc) Benchmark {
	timestamp := time.Now().UnixMicro()
	start := time.Now()
	r := send(client)
	took := time.Since(start)
	bench := Benchmark{
		Timestamp:  timestamp,
		Took:       took,
		Success:    r.Success,
		Extra:      r.Extra,
		HttpStatus: r.HttpStatus,
	}
	return bench
}

func plotGraph(spec BenchmarkSpec, bench []Benchmark, stat Stats, title string, xlabel string, fname string, drawPercentile bool) error {
	p := plot.New()
	p.Title.Text = "\n" + title
	p.Title.Padding = 0.1 * vg.Inch
	p.X.Label.Text = "\n" + xlabel + "\n"
	p.X.Label.Padding = 0.1 * vg.Inch
	p.Y.Label.Text = "\nRequest Latency (ms)\n"
	p.Y.Label.Padding = 0.1 * vg.Inch
	p.Y.Min = float64(stat.Min.Milliseconds())
	if p.Y.Min > 10 {
		p.Y.Min -= 10
	} else {
		p.Y.Min = 0
	}
	p.Y.Max = float64(stat.Max.Milliseconds()) + 1
	data := toXYs(bench)
	err := plotutil.AddLinePoints(p, "Latency (ms)", data)
	if err != nil {
		return err
	}

	if !spec.DisablePlotInclPercentileLines && drawPercentile {
		i := 1
		for k, v := range stat.Percentiles {
			drawPercentileLine(p, v.Index, fmt.Sprintf("P%d", k), i)
			i++
		}
	}

	if !spec.DisablePlotInclMinMaxLabels {
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
	return p.Save(spec.PlotWidth, spec.PlotHeight, fname)
}

func toXYs(bench []Benchmark) plotter.XYs {
	pts := make(plotter.XYs, 0, len(bench))
	for i := range bench {
		pts = append(pts, plotter.XY{
			X: float64(i),
			Y: float64(bench[i].Took.Milliseconds()),
		})
	}
	return pts
}

func percentile(bench []Benchmark, percentile float64) Percentile {
	idx := math.Ceil(percentile / 100.0 * float64(len(bench)))
	i := int(idx) - 1
	return Percentile{Record: bench[i], Index: i}
}

func drawPercentileLine(p *plot.Plot, index int, label string, color int) {
	xys := make(plotter.XYs, 2)
	xys[0] = plotter.XY{X: float64(index), Y: 0}
	xys[1] = plotter.XY{X: float64(index), Y: p.Y.Max}

	line, err := plotter.NewLine(xys)
	if err == nil {
		line.LineStyle.Color = plotutil.Color(color)
		p.Add(line)

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

func newClient() *http.Client {
	c := &http.Client{
		Timeout: 10 * time.Second,
	}
	c.Transport = &http.Transport{
		DisableKeepAlives: false,
	}
	return c
}
