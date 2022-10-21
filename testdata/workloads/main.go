package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"golang.org/x/sync/errgroup"
)

type Workload interface {
	Request() error
}

func main() {
	cmd := Cmd{}
	flag.StringVar(&cmd.CPUProfile, "cpuprofile", "", "Path to output CPU profile")
	flag.DurationVar(&cmd.Duration, "duration", time.Second, "Duration for running the workload")
	flag.BoolVar(&cmd.Timeline, "timeline", false, "Add trace_id, span_id and goroutine_id labels")
	flag.IntVar(&cmd.Concurrency, "concurrency", runtime.GOMAXPROCS(0), "Number of goroutines executing the workload")
	flag.Parse()
	cmd.Workload = flag.Arg(0)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

type Cmd struct {
	CPUProfile  string
	Workload    string
	Duration    time.Duration
	Concurrency int
	Timeline    bool
}

func (c *Cmd) Run() (err error) {
	stop := c.startCPUProfile()
	defer stop()

	var workload Workload
	switch c.Workload {
	case "json":
		workload = JSONWorkload{}
	case "printf":
		workload = PrintfWorkload{}
	default:
		return fmt.Errorf("unknown workload: %q", c.Workload)
	}

	var doneCh = make(chan struct{})
	var eg errgroup.Group
	for i := 0; i < c.Concurrency; i++ {
		setTimelineLabels := timelineLabelGenerator(fmt.Sprintf("%d", i))
		eg.Go(func() error {
			for {
				select {
				case <-doneCh:
					return nil
				default:
					if c.Timeline {
						setTimelineLabels()
					}
					if err := workload.Request(); err != nil {
						return err
					}
				}
			}
		})
	}

	time.Sleep(c.Duration)
	close(doneCh)

	if err := eg.Wait(); err != nil {
		return err
	}
	return stop()
}

func timelineLabelGenerator(goroutineID string) func() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return func() {
		labels := pprof.Labels(
			"trace_id", randomHexID(r, 16), // like otel
			"span_id", randomHexID(r, 8), // like otel
			"goroutine_id", goroutineID, // go cpu profiler doesn't add this yet, so we do it
		)
		ctx := pprof.WithLabels(context.Background(), labels)
		pprof.SetGoroutineLabels(ctx)
	}
}

// return a random id of size length as hex (lenght * 2)
func randomHexID(r *rand.Rand, length int) string {
	id := make([]byte, length)
	r.Read(id)
	return fmt.Sprintf("%x", id)
}

func (c *Cmd) startCPUProfile() func() error {
	if c.CPUProfile == "" {
		return func() error { return nil }
	}
	file, err := os.Create(c.CPUProfile)
	if err != nil {
		return func() error { return err }
	}
	if err := pprof.StartCPUProfile(file); err != nil {
		file.Close()
		return func() error { return err }
	}
	return func() error {
		pprof.StopCPUProfile()
		return file.Close()
	}
}
