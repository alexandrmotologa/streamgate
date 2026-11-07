package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type BenchStats struct {
	TotalDelivered  uint64
	TotalErrors     uint64
	ConnectedCount  int64
	PublishCount    uint64
	LatenciesMs     []float64
	latenciesMu     sync.Mutex
}

func main() {
	target := flag.String("target", "http://127.0.0.1:8080", "Streamgate target URL")
	clients := flag.Int("clients", 1000, "Number of concurrent SSE subscriber connections")
	channel := flag.String("channel", "bench.load", "Channel name for fan-out benchmark")
	publishRate := flag.Int("rate", 20, "Number of broadcast events published per second")
	duration := flag.Duration("duration", 10*time.Second, "Benchmark run duration")
	flag.Parse()

	fmt.Println()
	fmt.Println("  ================================================================")
	fmt.Println("   STREAMGATE HIGH-CONCURRENCY FAN-OUT BENCHMARK")
	fmt.Println("  ================================================================")
	fmt.Printf("   Target:          %s\n", *target)
	fmt.Printf("   Subscribers:     %d concurrent streams\n", *clients)
	fmt.Printf("   Channel:         %s\n", *channel)
	fmt.Printf("   Publish Rate:    %d msgs/sec\n", *publishRate)
	fmt.Printf("   Duration:        %v\n", *duration)
	fmt.Println("  ----------------------------------------------------------------")

	stats := &BenchStats{
		LatenciesMs: make([]float64, 0, 50000),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Ramp up subscribers
	fmt.Printf("[1/3] Spawning %d concurrent SSE subscriber connections...\n", *clients)
	rampStart := time.Now()

	var wg sync.WaitGroup
	// Concurrency limiter for establishing connections
	connSem := make(chan struct{}, 1000)

	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        *clients + 100,
			MaxIdleConnsPerHost: *clients + 100,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression: true,
		},
	}

	for i := 0; i < *clients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			connSem <- struct{}{}
			defer func() { <-connSem }()

			url := fmt.Sprintf("%s/v1/subscribe?channel=%s", *target, *channel)
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				atomic.AddUint64(&stats.TotalErrors, 1)
				return
			}

			resp, err := httpClient.Do(req)
			if err != nil {
				atomic.AddUint64(&stats.TotalErrors, 1)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				atomic.AddUint64(&stats.TotalErrors, 1)
				return
			}

			atomic.AddInt64(&stats.ConnectedCount, 1)
			reader := bufio.NewReader(resp.Body)

			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				line = strings.TrimSpace(line)

				if strings.HasPrefix(line, "data: ") {
					atomic.AddUint64(&stats.TotalDelivered, 1)
					payload := strings.TrimPrefix(line, "data: ")

					// Extract sent_at timestamp to compute latency
					var msg map[string]any
					if json.Unmarshal([]byte(payload), &msg) == nil {
						if sentAtVal, ok := msg["sent_at"].(float64); ok {
							nowMs := float64(time.Now().UnixNano()) / 1e6
							latMs := nowMs - sentAtVal
							if latMs >= 0 && latMs < 5000 {
								// Sample latency
								if atomic.LoadUint64(&stats.TotalDelivered)%10 == 0 {
									stats.latenciesMu.Lock()
									if len(stats.LatenciesMs) < 50000 {
										stats.LatenciesMs = append(stats.LatenciesMs, latMs)
									}
									stats.latenciesMu.Unlock()
								}
							}
						}
					}
				}
			}
		}(i)
	}

	// Wait until most connections are established
	timeout := time.After(15 * time.Second)
WaitForConn:
	for {
		select {
		case <-timeout:
			break WaitForConn
		case <-time.After(200 * time.Millisecond):
			connected := atomic.LoadInt64(&stats.ConnectedCount)
			if int(connected) >= int(float64(*clients)*0.95) {
				break WaitForConn
			}
		}
	}

	connected := atomic.LoadInt64(&stats.ConnectedCount)
	fmt.Printf("      -> %d / %d subscribers connected in %.2fs\n", connected, *clients, time.Since(rampStart).Seconds())

	// 2. Start publisher
	fmt.Printf("[2/3] Broadcasting at %d msgs/sec for %v...\n", *publishRate, *duration)
	pubClient := &http.Client{Timeout: 5 * time.Second}
	pubURL := fmt.Sprintf("%s/api/v1/publish", *target)

	interval := time.Second / time.Duration(*publishRate)
	pubTicker := time.NewTicker(interval)
	defer pubTicker.Stop()

	testDeadline := time.After(*duration)
	var pubCount uint64

PublishLoop:
	for {
		select {
		case <-testDeadline:
			break PublishLoop
		case <-pubTicker.C:
			nowMs := float64(time.Now().UnixNano()) / 1e6
			body, _ := json.Marshal(map[string]any{
				"channel": *channel,
				"event":   "bench.tick",
				"data": map[string]any{
					"seq":     pubCount,
					"sent_at": nowMs,
				},
			})

			req, _ := http.NewRequest("POST", pubURL, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := pubClient.Do(req)
			if err == nil {
				resp.Body.Close()
				pubCount++
			}
		}
	}

	// 3. Collect gateway telemetry
	fmt.Println("[3/3] Querying gateway operational telemetry...")
	var memAllocMB string
	var sysAllocMB string
	var goroutines int

	statsReq, err := http.Get(fmt.Sprintf("%s/api/v1/stats", *target))
	if err == nil {
		defer statsReq.Body.Close()
		var st map[string]any
		if json.NewDecoder(statsReq.Body).Decode(&st) == nil {
			if rt, ok := st["runtime"].(map[string]any); ok {
				if m, ok := rt["alloc_mb"].(string); ok {
					memAllocMB = m
				}
				if sys, ok := rt["sys_bytes"].(float64); ok {
					sysAllocMB = fmt.Sprintf("%.2f", sys/(1024*1024))
				}
				if g, ok := rt["goroutines"].(float64); ok {
					goroutines = int(g)
				}
			}
		}
	}

	// Stop subscribers
	cancel()
	time.Sleep(500 * time.Millisecond)

	// Compute latency percentiles
	stats.latenciesMu.Lock()
	sort.Float64s(stats.LatenciesMs)
	n := len(stats.LatenciesMs)

	p50 := 0.0
	p95 := 0.0
	p99 := 0.0
	if n > 0 {
		p50 = stats.LatenciesMs[int(float64(n)*0.50)]
		p95 = stats.LatenciesMs[int(float64(n)*0.95)]
		p99 = stats.LatenciesMs[int(float64(n)*0.99)]
	}
	stats.latenciesMu.Unlock()

	totalDelivered := atomic.LoadUint64(&stats.TotalDelivered)
	rateAchieved := float64(totalDelivered) / duration.Seconds()

	fmt.Println()
	fmt.Println("  ================================================================")
	fmt.Println("   BENCHMARK RESULTS & MEMORY PROFILE")
	fmt.Println("  ================================================================")
	fmt.Printf("   Connected Subscribers:  %d\n", connected)
	fmt.Printf("   Events Ingested:        %d\n", pubCount)
	fmt.Printf("   Total Event Fan-Outs:   %d deliveries\n", totalDelivered)
	fmt.Printf("   Effective Throughput:   %.1f msgs/sec fan-out\n", rateAchieved)
	fmt.Printf("   Delivery Latency p50:   %.2f ms\n", p50)
	fmt.Printf("   Delivery Latency p95:   %.2f ms\n", p95)
	fmt.Printf("   Delivery Latency p99:   %.2f ms\n", p99)
	fmt.Println("  ----------------------------------------------------------------")
	fmt.Printf("   Gateway Heap Alloc:     %s MB (Budget: < 45 MB)\n", memAllocMB)
	fmt.Printf("   Gateway Sys Alloc:      %s MB\n", sysAllocMB)
	fmt.Printf("   Active Goroutines:      %d\n", goroutines)
	fmt.Println("  ================================================================")
	fmt.Println()
}
