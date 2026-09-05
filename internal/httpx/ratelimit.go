package httpx

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type limiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	perMin int
	window time.Duration
}

func RateLimit(perMin int, next http.Handler) http.Handler {
	l := &limiter{hits: map[string][]time.Time{}, perMin: perMin, window: time.Minute}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !l.allow(ip) {
			http.Error(w, `{"errors":[{"message":"rate limit exceeded"}]}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cut := now.Add(-l.window)
	arr := l.hits[key]
	kept := arr[:0]
	for _, t := range arr {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.perMin {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}
