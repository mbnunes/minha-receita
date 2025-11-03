package api

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/shirou/gopsutil/v3/process"
)

var (
	metricLabels = []string{"method", "status_code", "endpoint"}
	requestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "total_requests",
		Help: "The total number of requests served",
	}, metricLabels)
	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "request_duration",
		Help: "The duration of requests in milliseconds",
	}, metricLabels)
)

// --- Métricas da aplicação (CPU, memória, rede) ---
var (
	procCPU = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_cpu_percent",
		Help: "CPU usage percent of this application",
	})
	procMem = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_memory_bytes",
		Help: "Memory usage of this application in bytes",
	})
	procNetSent = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_network_sent_bytes",
		Help: "Network bytes sent by this application",
	})
	procNetRecv = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_network_recv_bytes",
		Help: "Network bytes received by this application",
	})
)

func registerMetric(e, m string, s int, i int64) {
	c := fmt.Sprintf("%d", s)
	requestCount.WithLabelValues(m, c, e).Inc()
	requestDuration.WithLabelValues(m, c, e).Observe(float64(time.Now().UnixMilli() - i))
}

func collectAppMetrics() {
	// Memória do Go
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	procMem.Set(float64(m.Alloc))

	// CPU e rede do processo
	pid := int32(os.Getpid())
	proc, err := process.NewProcess(pid)
	if err != nil {
		return
	}

	// CPU %
	cpuPercent, err := proc.CPUPercent()
	if err == nil {
		procCPU.Set(cpuPercent)
	}

	// Rede
	ioCounters, err := proc.IOCounters()
	if err == nil {
		procNetSent.Set(float64(ioCounters.WriteBytes))
		procNetRecv.Set(float64(ioCounters.ReadBytes))
	}
}

// --- Inicia coleta automática ao importar o pacote ---
func init() {
	go func() {
		for {
			collectAppMetrics()
			time.Sleep(5 * time.Second) // atualiza a cada 5 segundos
		}
	}()
}
