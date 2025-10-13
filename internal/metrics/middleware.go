package metrics

import (
	"net/http"
	"time"
)

/*
HTTP指标中间件

这个中间件用于记录HTTP请求的指标，包括请求计数和延迟。
它可以被集成到HTTP服务器中，自动收集所有请求的指标。
*/

// MetricsMiddleware 返回一个HTTP中间件，用于记录请求指标
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// 创建一个包装的ResponseWriter来捕获状态码
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // 默认状态码
		}
		
		// 调用下一个处理器
		next.ServeHTTP(wrapped, r)
		
		// 记录指标
		duration := time.Since(start)
		metrics := GetMetrics()
		
		metrics.RecordHTTPRequest(r.Method, r.URL.Path, http.StatusText(wrapped.statusCode))
		metrics.ObserveHTTPRequestDuration(r.Method, r.URL.Path, duration)
	})
}

// responseWriter 包装http.ResponseWriter以捕获状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader 捕获状态码
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}