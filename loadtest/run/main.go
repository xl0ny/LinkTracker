package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/loadtest/config"
)

type opKind int

const (
	opGet opKind = iota
	opPost
)

const (
	reportFilePerms = 0o644
	p50Percent         = 50
	p99Percent         = 99
	hundredPercent     = 100
	httpClientErrCodes = 4
	httpServerErrCodes = 5
)

func (k opKind) String() string {
	if k == opGet {
		return "GET"
	}
	return "POST"
}

type sample struct {
	kind   opKind
	status int
	dur    time.Duration
	errMsg string
}

type stats struct {
	mu       sync.Mutex
	samples  []sample
	statuses map[int]int
	errs     map[string]int
}

type report struct {
	kind   opKind
	count  int
	rps    float64
	mean   time.Duration
	p50    time.Duration
	p99    time.Duration
	status map[int]int
	errs   map[string]int
}

func newStats() *stats {
	return &stats{statuses: map[int]int{}, errs: map[string]int{}}
}

func (s *stats) add(sm sample) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.samples = append(s.samples, sm)
	s.statuses[sm.status]++
	if sm.errMsg != "" {
		s.errs[sm.errMsg]++
	}
}

func (s *stats) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.samples)
}

func main() {
	cfgPath := flag.String("config", config.DefaultPath, "path to loadtest config.yaml")
	scenario := flag.String("scenario", "", "override run.scenario from config")
	output := flag.String("out", "", "override run.output from config")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("loadtest: %v", err)
	}
	opts := cfg.Run
	if *scenario != "" {
		opts.Scenario = *scenario
	}
	if *output != "" {
		opts.Output = *output
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deadline := time.Now().Add(opts.RampUp).Add(opts.Duration)
	log.Printf("loadtest: scenario=%s vus=%d rampup=%s duration=%s chats=%d ratio=%d target=%s",
		opts.Scenario, opts.VUs, opts.RampUp, opts.Duration, opts.Chats, opts.Ratio, opts.Target)

	st := newStats()
	wg, started := startVUs(ctx, st, opts, deadline)
	awaitDeadline(ctx, st, started, deadline, opts.ProgressInterval)
	cancel()
	wg.Wait()

	getRep := buildReport(st, opGet, opts.Duration)
	postRep := buildReport(st, opPost, opts.Duration)

	md := renderMarkdown(opts, getRep, postRep)
	fmt.Println(md)

	if opts.Output != "" {
		if err := appendReport(opts.Output, md); err != nil {
			log.Printf("loadtest: %v", err)
			return
		}
		log.Printf("loadtest: report appended to %s", opts.Output)
	}
}

func startVUs(ctx context.Context, st *stats, opts config.Run, deadline time.Time) (*sync.WaitGroup, *atomic.Int64) {
	var wg sync.WaitGroup
	var started atomic.Int64
	for i := range opts.VUs {
		wg.Go(func() {
			idx := i
			startAt := time.Duration(float64(opts.RampUp) * float64(idx) / float64(opts.VUs))
			select {
			case <-time.After(startAt):
			case <-ctx.Done():
				return
			}
			started.Add(1)
			runVU(ctx, deadline, idx, opts, st)
		})
	}
	return &wg, &started
}

func awaitDeadline(ctx context.Context, st *stats, started *atomic.Int64, deadline time.Time, progressInterval time.Duration) {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	ticker := time.NewTicker(progressInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Printf("loadtest: progress vus_started=%d samples=%d", started.Load(), st.count())
		case <-timer.C:
			return
		}
	}
}

func appendReport(path, md string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, reportFilePerms)
	if err != nil {
		return fmt.Errorf("open out: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err = f.WriteString("\n" + md + "\n"); err != nil {
		return fmt.Errorf("write out: %w", err)
	}
	return nil
}

func runVU(ctx context.Context, deadline time.Time, idx int, opts config.Run, st *stats) {
	rng := rand.New(rand.NewPCG(uint64(idx), uint64(time.Now().UnixNano())))
	client := &http.Client{Timeout: opts.HTTPClientTimeout}
	counter := 0
	for {
		if ctx.Err() != nil || time.Now().After(deadline) {
			return
		}
		chatID := opts.ChatIDBase + int64(rng.IntN(opts.Chats))
		var sm sample
		if counter%(opts.Ratio+1) == opts.Ratio {
			sm = doPost(ctx, client, opts.Target, chatID, rng)
		} else {
			sm = doGet(ctx, client, opts.Target, chatID)
		}
		st.add(sm)
		counter++
	}
}

func doGet(ctx context.Context, client *http.Client, target string, chatID int64) sample {
	t0 := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target+"/links", nil)
	if err != nil {
		return sample{kind: opGet, dur: time.Since(t0), errMsg: err.Error()}
	}
	req.Header.Set("Tg-Chat-Id", strconv.FormatInt(chatID, 10))
	resp, err := client.Do(req)
	dur := time.Since(t0)
	if err != nil {
		return sample{kind: opGet, dur: dur, errMsg: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return sample{kind: opGet, dur: dur, status: resp.StatusCode}
}

func doPost(ctx context.Context, client *http.Client, target string, chatID int64, rng *rand.Rand) sample {
	t0 := time.Now()
	url := fmt.Sprintf("https://example.com/loadtest/%d-%d", chatID, rng.Int64())
	body, _ := json.Marshal(map[string]any{
		"link":    url,
		"tags":    []string{"loadtest"},
		"filters": []string{},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target+"/links", bytes.NewReader(body))
	if err != nil {
		return sample{kind: opPost, dur: time.Since(t0), errMsg: err.Error()}
	}
	req.Header.Set("Tg-Chat-Id", strconv.FormatInt(chatID, 10))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	dur := time.Since(t0)
	if err != nil {
		return sample{kind: opPost, dur: dur, errMsg: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return sample{kind: opPost, dur: dur, status: resp.StatusCode}
}

func buildReport(s *stats, k opKind, stage time.Duration) report {
	s.mu.Lock()
	defer s.mu.Unlock()
	durs := make([]time.Duration, 0)
	status := map[int]int{}
	errs := map[string]int{}
	for _, sm := range s.samples {
		if sm.kind != k {
			continue
		}
		durs = append(durs, sm.dur)
		status[sm.status]++
		if sm.errMsg != "" {
			errs[sm.errMsg]++
		}
	}
	if len(durs) == 0 {
		return report{kind: k, status: status, errs: errs}
	}
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })

	var total time.Duration
	for _, d := range durs {
		total += d
	}
	mean := total / time.Duration(len(durs))
	p50 := durs[len(durs)*p50Percent/hundredPercent]
	p99 := durs[min(len(durs)-1, len(durs)*p99Percent/hundredPercent)]

	rps := float64(len(durs)) / stage.Seconds()
	return report{
		kind:   k,
		count:  len(durs),
		rps:    rps,
		mean:   mean,
		p50:    p50,
		p99:    p99,
		status: status,
		errs:   errs,
	}
}

func renderMarkdown(opts config.Run, get, post report) string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "## Scenario: %s\n\n", opts.Scenario)
	fmt.Fprintf(&b, "- VUs: %d\n- CPU: %d\n- ramp-up: %s\n- stage: %s\n- chats: %d\n- GET:POST ratio: %d:1\n\n",
		opts.VUs, runtime.NumCPU(), opts.RampUp, opts.Duration, opts.Chats, opts.Ratio)
	fmt.Fprintln(&b, "| Method | Count | RPS | mean | p50 | p99 | 200 | 4xx | 5xx | other |")
	fmt.Fprintln(&b, "|--------|-------|-----|------|-----|-----|-----|-----|-----|-------|")
	for _, r := range []report{get, post} {
		ok := r.status[http.StatusOK]
		c4xx, c5xx, other := classifyStatuses(r.status)
		fmt.Fprintf(&b, "| %s | %d | %.1f | %s | %s | %s | %d | %d | %d | %d |\n",
			r.kind, r.count, r.rps, r.mean, r.p50, r.p99, ok, c4xx, c5xx, other)
	}
	if len(get.errs)+len(post.errs) > 0 {
		fmt.Fprintln(&b, "\nMost frequent errors:")
		for _, r := range []report{get, post} {
			for msg, n := range r.errs {
				fmt.Fprintf(&b, "- %s %s × %d\n", r.kind, msg, n)
			}
		}
	}
	return b.String()
}

func classifyStatuses(status map[int]int) (c4xx, c5xx, other int) {
	for code, n := range status {
		switch {
		case code == http.StatusOK:
		case code/100 == httpClientErrCodes:
			c4xx += n
		case code/100 == httpServerErrCodes:
			c5xx += n
		default:
			other += n
		}
	}
	return c4xx, c5xx, other
}

