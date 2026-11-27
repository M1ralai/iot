package logger

import (
	"log"
	"net/http"
	"time"
)

func LoggingHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[%s] %q %v", r.Method, r.URL.String(), time.Since(start))
	}
}
