package benchmarker

import (
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/curtisnewbie/miso/util"
	"github.com/spf13/cast"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

var (
	defPlotWidth                        = 30 * vg.Inch
	defPlotHeight                       = 15 * vg.Inch
	defPlotSortedByRequestOrderFilename = "plot_sorted_by_request_order.png"
	defPlotSortedByLatencyFilename      = "plot_sorted_by_latency.png"
	defPlotSuccessRateFilename          = "plot_success_rate.png"
	defDataOutputFilename               = "benchmark_records.txt"
)

var (
	// rough estimate on how many benchmark results will be created by one goroutine, increase it if necessary.
	ResultQueueSize = 1000
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
	PlotSuccessRateFilename          string
	DataOutputFilename               string

	benchmarkTime string
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
	if spec.PlotSuccessRateFilename == "" {
		spec.PlotSuccessRateFilename = defPlotSuccessRateFilename
	}
	if spec.DataOutputFilename == "" {
		spec.DataOutputFilename = defDataOutputFilename
	}
	spec.benchmarkTime = util.Now().FormatClassicLocale()

	pool := util.NewAsyncPool(spec.Concurrent, spec.Concurrent)
	aw := util.NewAwaitFutures[[]Benchmark](pool)

	var warmupWg sync.WaitGroup // for warmup
	warmupWg.Add(spec.Concurrent)

	var startTimeOnce sync.Once
	var startTime time.Time
	util.DebugPrintlnf(spec.DebugLog, "Creating workers: %v", time.Now())

	var cmu sync.Mutex
	var successCount int64 = 0
	var failCount int64 = 0

	updateCount := func(success bool) float64 {
		cmu.Lock()
		if success {
			successCount += 1
		} else {
			failCount += 1
		}
		s := successCount
		f := failCount
		cmu.Unlock()

		return float64(s) / float64(s+f)
	}

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
				localStore = make([]Benchmark, 0, ResultQueueSize)
			} else {
				localStore = make([]Benchmark, 0, spec.Round)
			}

			if durBased {
				for time.Since(startTime) <= spec.Duration {
					b := triggerOnce(client, spec.SendReqFunc)
					b.successRate = updateCount(b.Success)
					localStore = append(localStore, b)
				}
			} else {
				for j := 0; j < spec.Round; j++ {
					b := triggerOnce(client, spec.SendReqFunc)
					b.successRate = updateCount(b.Success)
					localStore = append(localStore, b)
				}
			}
			return localStore, nil
		})
	}

	var size int
	if !durBased {
		size = spec.Concurrent * spec.Round
	} else {
		size = spec.Concurrent * ResultQueueSize
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
		SortTimestamp(benchmarks)
		err := plotGraph(spec, benchmarks, stats, spec.benchmarkTime+" - Request Latency Plot "+titleStats,
			"X - Sorted By Request Timestamp", spec.PlotSortedByRequestOrderFilename, false)
		if err != nil {
			return benchmarks, stats, err
		}
		util.Printlnf("Generated plot graph: %v", spec.PlotSortedByRequestOrderFilename)

		err = plotSuccessRateGraph(spec, benchmarks, spec.benchmarkTime+" - Success Rate Plot "+titleStats,
			"X - Sorted By Request Timestamp", spec.PlotSuccessRateFilename)
		if err != nil {
			return benchmarks, stats, err
		}
		util.Printlnf("Generated plot graph: %v", spec.PlotSuccessRateFilename)

		SortTook(benchmarks)
		err = plotGraph(spec, benchmarks, stats, spec.benchmarkTime+" - Latency Percentile Plot "+titleStats,
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
	Timestamp   int64
	Took        time.Duration
	Success     bool
	Extra       map[string]any
	HttpStatus  int
	successRate float64
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

	SortTook(bench)
	stats.TotalTime = totalTime
	stats.TotalRequests = total
	stats.Throughput = float64(total) / (float64(totalTime) / float64(time.Second))
	stats.StatusCount = statusCount
	stats.SuccessCount = successCount

	sl := util.SLPinter{}
	sl.Printlnf("\nBenchmark Time: %v", spec.benchmarkTime)
	sl.Printlnf("\n--------- Brief ---------------\n")
	sl.Printlnf("total_time: %v", totalTime)
	sl.Printlnf("total_requests: %v", total)
	sl.Printlnf("throughput: %.0f req/sec", stats.Throughput)
	sl.Printlnf("concurrency: %v", concurrent)
	if dur > 0 {
		sl.Printlnf("duration: %v", dur)
	} else {
		sl.Printlnf("rounds (for each worker): %v", round)
	}
	sl.Printlnf("status_count: %v", statusCount)
	sl.Printlnf("success_count: %v", successCount)
	sl.Printlnf("\n--------- Latency -------------\n")
	sl.Printlnf("min: %v", stats.Min)
	sl.Printlnf("max: %v", stats.Max)
	sl.Printlnf("median: %v", stats.Med)
	sl.Printlnf("avg: %v", stats.Avg)

	stats.Percentiles = map[int]Percentile{}
	if total > 0 {
		for _, pv := range []int{75, 90, 95, 99} {
			stats.Percentiles[pv] = percentile(bench, float64(pv))
			sl.Printlnf("P%d: %v", pv, stats.Percentiles[pv].Record.Took)
		}
	}

	if !spec.DisableOutputFile {
		sl.Printlnf("\n--------- Data ----------------\n")
		sl.Printlnf("data file: %v", spec.DataOutputFilename)
		sl.WriteString("\n")
	} else if len(logStatFunc) < 1 {
		sl.WriteString("\n")
	}

	if len(logStatFunc) > 0 {
		sl.Printlnf("\n--------- Extra ---------------\n")
		for _, f := range logStatFunc {
			output := f(bench)
			if output != "" {
				sl.Printlnf(output)
			}
		}
		sl.WriteString("\n")
	}
	print(sl.String())

	// sort by request order for readability in data output file
	SortTimestamp(bench)
	if !spec.DisableOutputFile {
		f, err := util.ReadWriteFile(spec.DataOutputFilename)
		if err != nil {
			return stats, err
		}
		defer f.Close()
		_ = f.Truncate(0)

		sl.Printlnf("-------------------------------\n\n")
		f.WriteString(sl.String())
		for _, b := range bench {
			f.WriteString(fmt.Sprintf("Timestamp: %d, Took: %v, Success: %v (%.2f%%), HttpStatus: %d, Extra: %+v\n", b.Timestamp,
				b.Took, b.Success, b.successRate*100, b.HttpStatus, b.Extra))
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
	p.X.Max = float64(len(bench) + 1)
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
	err := plotutil.AddLinePoints(p, data)
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

func plotSuccessRateGraph(spec BenchmarkSpec, bench []Benchmark, title string, xlabel string, fname string) error {
	p := plot.New()
	p.Title.Text = "\n" + title
	p.Title.Padding = 0.1 * vg.Inch
	p.X.Label.Text = "\n" + xlabel + "\n"
	p.X.Label.Padding = 0.1 * vg.Inch
	p.Y.Label.Text = "\nSuccess Rate (%)\n"
	p.Y.Label.Padding = 0.1 * vg.Inch
	p.X.Max = float64(len(bench) + 1)
	p.Y.Min = 0
	p.Y.Max = 101

	drawSuccessRateLine(p, toSuccessRateXYs(bench), 1)
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

func toSuccessRateXYs(bench []Benchmark) plotter.XYs {
	pts := make(plotter.XYs, 0, len(bench))
	for i := range bench {
		pts = append(pts, plotter.XY{
			X: float64(i),
			Y: cast.ToFloat64(fmt.Sprintf("%.2f", bench[i].successRate*100)),
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

func drawSuccessRateLine(p *plot.Plot, dat plotter.XYs, color int) {

	// find min, max
	var min, max float64 = math.MaxFloat64, 0
	var mini, maxi int
	for i, xy := range dat {
		if xy.Y < min {
			mini = i
			min = xy.Y
		}
		if xy.Y >= max {
			maxi = i
			max = xy.Y
		}
	}

	line, err := plotter.NewLine(dat)
	if err == nil {
		line.LineStyle.Color = plotutil.Color(color)
		p.Add(line)

		if lineLabels, err := plotter.NewLabels(plotter.XYLabels{
			XYs:    []plotter.XY{{X: float64(mini), Y: min + 1}},
			Labels: []string{cast.ToString(min) + "%"},
		}); err == nil {
			p.Add(lineLabels)
		}

		if lineLabels, err := plotter.NewLabels(plotter.XYLabels{
			XYs:    []plotter.XY{{X: float64(maxi), Y: max + 1}},
			Labels: []string{cast.ToString(max) + "%"},
		}); err == nil {
			p.Add(lineLabels)
		}

		if lineLabels, err := plotter.NewLabels(plotter.XYLabels{
			XYs:    []plotter.XY{{X: float64(1), Y: dat[0].Y + 1}},
			Labels: []string{"Success Rate (%)"},
		}); err == nil {
			p.Add(lineLabels)
		}

	} else {
		util.Printlnf("failed to draw success rate line, %v", err)
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

var (
	debug     = flag.Bool("debug", false, "Enable debug log")
	conc      = flag.Int("conc", 1, "Concurrency")
	concGroup = flag.String("concgroup", "", "Concurrency Groups (e.g., '1,30,50', is equivalent to running the benchmark three times with concurrency 1, 30 and 50)")
	round     = flag.Int("round", 2, "Round")
	duration  = flag.Duration("dur", 0, "Duration")
)

type CliBenchmarkResult struct {
	Benchmarks []Benchmark
	Stats      Stats
}

func StartBenchmarkCli(spec BenchmarkSpec) ([]CliBenchmarkResult, error) {
	flag.Parse()
	spec.Concurrent = *conc
	spec.Round = *round
	spec.Duration = *duration
	spec.DebugLog = *debug

	if util.IsBlankStr(*concGroup) {
		res := make([]CliBenchmarkResult, 1)
		b, s, err := StartBenchmark(spec)
		res = append(res, CliBenchmarkResult{
			Benchmarks: b,
			Stats:      s,
		})
		return res, err
	}

	tok := strings.Split(*concGroup, ",")

	if spec.PlotSortedByRequestOrderFilename == "" {
		spec.PlotSortedByRequestOrderFilename = defPlotSortedByRequestOrderFilename
	}
	if spec.PlotSortedByLatencyFilename == "" {
		spec.PlotSortedByLatencyFilename = defPlotSortedByLatencyFilename
	}
	if spec.DataOutputFilename == "" {
		spec.DataOutputFilename = defDataOutputFilename
	}

	res := make([]CliBenchmarkResult, len(*concGroup))
	for _, t := range tok {
		if util.IsBlankStr(t) {
			continue
		}
		c := cast.ToInt(t)
		if c < 1 {
			continue
		}
		cp := spec        // this is a value copy
		cp.Concurrent = c // change concurrency value
		cs := cast.ToString(c)
		cp.PlotSortedByRequestOrderFilename = "conc" + cs + "_" + cp.PlotSortedByRequestOrderFilename
		cp.PlotSortedByLatencyFilename = "conc" + cs + "_" + cp.PlotSortedByLatencyFilename
		cp.PlotSuccessRateFilename = "conc" + cs + "_" + cp.PlotSuccessRateFilename
		cp.DataOutputFilename = "conc" + cs + "_" + cp.DataOutputFilename

		b, s, err := StartBenchmark(cp)
		res = append(res, CliBenchmarkResult{
			Benchmarks: b,
			Stats:      s,
		})
		if err != nil {
			return res, err
		}
	}
	return res, nil
}
