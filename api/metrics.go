package api

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

var (
	metricLabels = []string{"method", "status_code", "endpoint"}

	requestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "total_requests",
		Help: "The total number of requests served",
	}, metricLabels)

	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "request_duration",
		Help: "The duration of requests in milliseconds",
	}, metricLabels)

	errorCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "error_requests_total",
		Help: "Total number of requests that returned an error per endpoint",
	}, []string{"endpoint", "status_code"})

	appCPU = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "app_cpu_percent",
		Help: "CPU percent used by this application",
	})
	appMem = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "app_memory_bytes",
		Help: "Memory used by this application in bytes",
	})
	procNetSent = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "app_network_sent_bytes",
		Help: "Network bytes sent (system-wide, not per process)",
	})
	procNetRecv = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "app_network_recv_bytes",
		Help: "Network bytes received (system-wide, not per process)",
	})
)

// --- Função auxiliar ---
func safeCPUPercent(p *process.Process) float64 {
	if _, err := p.CPUPercent(); err == nil {
		time.Sleep(200 * time.Millisecond)
		if v, err := p.CPUPercent(); err == nil {
			return v
		}
	}
	return 0
}

// --- Coleta de rede ---
func collectNetworkMetrics() {
	if counters, err := net.IOCounters(false); err == nil && len(counters) > 0 {
		procNetSent.Set(float64(counters[0].BytesSent))
		procNetRecv.Set(float64(counters[0].BytesRecv))
	}
}

// --- Contagem de requisições e erros ---
func registerMetric(e, m string, s int, i int64) {
	status := fmt.Sprintf("%d", s)
	requestCount.WithLabelValues(m, status, e).Inc()
	requestDuration.WithLabelValues(m, status, e).Observe(float64(time.Now().UnixMilli() - i))

	if s >= 400 {
		errorCount.WithLabelValues(e, status).Inc()
	}
}

// --- Coleta de métricas do app ---
func collectAppMetrics() {
	collectNetworkMetrics()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	totalMem := float64(m.Alloc)
	totalCPU := 0.0

	pid := int32(os.Getpid())
	if proc, err := process.NewProcess(pid); err == nil {
		totalCPU = safeCPUPercent(proc)
		if memInfo, err := proc.MemoryInfo(); err == nil {
			totalMem += float64(memInfo.RSS)
		}
	}

	appCPU.Set(totalCPU)
	appMem.Set(totalMem)
}

// --- Collector customizado ---
type customCollector struct{}

func (c *customCollector) Describe(ch chan<- *prometheus.Desc) {
	requestCount.Describe(ch)
	requestDuration.Describe(ch)
	errorCount.Describe(ch)
	appCPU.Describe(ch)
	appMem.Describe(ch)
	procNetSent.Describe(ch)
	procNetRecv.Describe(ch)
}

func (c *customCollector) Collect(ch chan<- prometheus.Metric) {
	collectAppMetrics()

	requestCount.Collect(ch)
	requestDuration.Collect(ch)
	errorCount.Collect(ch)
	appCPU.Collect(ch)
	appMem.Collect(ch)
	procNetSent.Collect(ch)
	procNetRecv.Collect(ch)
}

func init() {
	prometheus.MustRegister(&customCollector{})
}
