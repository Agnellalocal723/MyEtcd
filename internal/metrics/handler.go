package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

/*
指标导出HTTP处理器

这个模块提供HTTP端点来导出Prometheus格式的指标。
默认端点为 /metrics，可以通过Prometheus服务器抓取。
*/

// NewHandler 创建一个新的指标处理器
func NewHandler() http.Handler {
	// 创建一个自定义的注册表，包含我们所有的指标
	registry := prometheus.NewRegistry()
	
	// 注册默认的Go指标
	registry.MustRegister(prometheus.NewGoCollector())
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	
	// 注册我们的自定义指标
	metrics := GetMetrics()
	registry.MustRegister(
		metrics.httpRequestsTotal,
		metrics.httpRequestDuration,
		metrics.storageOperationsTotal,
		metrics.storageOperationDuration,
		metrics.storageKeysTotal,
		metrics.storageSizeBytes,
		metrics.raftMessagesTotal,
		metrics.raftMessageDuration,
		metrics.raftLogEntriesTotal,
		metrics.raftLogIndex,
		metrics.raftTerm,
		metrics.raftState,
		metrics.watchersTotal,
		metrics.watchEventsTotal,
		metrics.leasesTotal,
		metrics.leaseExpirationsTotal,
		metrics.goRoutines,
		metrics.gcPauseDuration,
	)
	
	// 返回一个Prometheus HTTP处理器
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// SetupMetricsServer 设置指标服务器
func SetupMetricsServer(addr string) error {
	http.Handle("/metrics", NewHandler())
	return http.ListenAndServe(addr, nil)
}