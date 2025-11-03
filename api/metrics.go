package api

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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

	// Métricas separadas de CPU e memória
	appCPU = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "app_cpu_percent",
		Help: "CPU percent used by this application",
	})
	dbCPU = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_cpu_percent",
		Help: "CPU percent used by database (PostgreSQL or MongoDB)",
	})
	appMem = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "app_memory_bytes",
		Help: "Memory used by this application in bytes",
	})
	dbMem = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_memory_bytes",
		Help: "Memory used by database in bytes",
	})
	procNetSent = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "app_network_sent_bytes",
		Help: "Network bytes sent by this application",
	})
	procNetRecv = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "app_network_recv_bytes",
		Help: "Network bytes received by this application",
	})
)

// Contador e duração das requisições HTTP
func registerMetric(e, m string, s int, i int64) {
	c := fmt.Sprintf("%d", s)
	requestCount.WithLabelValues(m, c, e).Inc()
	requestDuration.WithLabelValues(m, c, e).Observe(float64(time.Now().UnixMilli() - i))
}

// --- Coleta sob demanda ---
func collectAppMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Métricas do app
	totalAppMem := float64(m.Alloc)
	totalAppCPU := 0.0

	pid := int32(os.Getpid())
	if proc, err := process.NewProcess(pid); err == nil {
		if cpuPercent, err := proc.CPUPercent(); err == nil {
			totalAppCPU += cpuPercent
		}
		if memInfo, err := proc.MemoryInfo(); err == nil {
			totalAppMem += float64(memInfo.RSS)
		}
		if ioCounters, err := proc.IOCounters(); err == nil {
			procNetSent.Set(float64(ioCounters.WriteBytes))
			procNetRecv.Set(float64(ioCounters.ReadBytes))
		}
	}

	// Métricas do banco
	totalDBMem := 0.0
	totalDBCPU := 0.0
	processes, _ := process.Processes()
	for _, p := range processes {
		name, _ := p.Name()
		cmd, _ := p.Cmdline()
		name = strings.ToLower(name)
		cmd = strings.ToLower(cmd)

		if strings.Contains(name, "postgres") || strings.Contains(cmd, "postgres") ||
			strings.Contains(name, "mongod") || strings.Contains(cmd, "mongod") {

			if cpuPercent, err := p.CPUPercent(); err == nil {
				totalDBCPU += cpuPercent
			}
			if memInfo, err := p.MemoryInfo(); err == nil {
				totalDBMem += float64(memInfo.RSS)
			}
		}
	}

	appCPU.Set(totalAppCPU)
	dbCPU.Set(totalDBCPU)
	appMem.Set(totalAppMem)
	dbMem.Set(totalDBMem)

}

// --- Implementa o Collector customizado ---
type customCollector struct{}

func (c *customCollector) Describe(ch chan<- *prometheus.Desc) {
	requestCount.Describe(ch)
	requestDuration.Describe(ch)
	appCPU.Describe(ch)
	dbCPU.Describe(ch)
	appMem.Describe(ch)
	dbMem.Describe(ch)
	procNetSent.Describe(ch)
	procNetRecv.Describe(ch)
}

func (c *customCollector) Collect(ch chan<- prometheus.Metric) {
	collectAppMetrics()
	requestCount.Collect(ch)
	requestDuration.Collect(ch)
	appCPU.Collect(ch)
	dbCPU.Collect(ch)
	appMem.Collect(ch)
	dbMem.Collect(ch)
	procNetSent.Collect(ch)
	procNetRecv.Collect(ch)
}

func init() {
	prometheus.MustRegister(&customCollector{})
}
