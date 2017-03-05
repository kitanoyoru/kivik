package serve

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/flimzy/kivik/logger"
)

func (s *Service) log(level logger.LogLevel, format string, args ...interface{}) {
	if s.LogWriter == nil {
		return
	}
	msg := strings.TrimSpace(fmt.Sprintf(format, args...))
	s.LogWriter.WriteLog(level, msg)
}

// Debug logs a debug message to the registered logger.
func (s *Service) Debug(format string, args ...interface{}) {
	s.log(logger.LogLevelDebug, format, args...)
}

// Info logs an informational message to the registered logger.
func (s *Service) Info(format string, args ...interface{}) {
	s.log(logger.LogLevelInfo, format, args...)
}

// Warn logs a warning message to the registered logger.
func (s *Service) Warn(format string, args ...interface{}) {
	s.log(logger.LogLevelWarn, format, args...)
}

// Error logs an error message to the registered logger.
func (s *Service) Error(format string, args ...interface{}) {
	s.log(logger.LogLevelError, format, args...)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func requestLogger(s *Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		ip := r.RemoteAddr
		ip = ip[0:strings.LastIndex(ip, ":")]
		s.Info("%s - - %s %s %d", ip, r.Method, r.URL.String(), sw.status)
	})
}
