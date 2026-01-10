package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cnlangzi/botrate"
	"golang.org/x/time/rate"
)

func main() {
	limiter := botrate.New(
		botrate.WithLimit(rate.Every(10*time.Minute)),
		botrate.WithAnalyzerWindow(time.Minute),
		botrate.WithAnalyzerPageThreshold(50),
		botrate.WithAnalyzerQueueCap(10000),
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.UserAgent()
		ip := extractIP(r)

		if !limiter.Allow(ua, ip) {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		w.Write([]byte("Hello!"))
	})

	http.Handle("/", handler)
	fmt.Println("Server started on :8080")
	http.ListenAndServe(":8080", nil)
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}
