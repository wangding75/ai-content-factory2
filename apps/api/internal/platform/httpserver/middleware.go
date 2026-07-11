package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request_id"

var requestCounter uint64

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := fmt.Sprintf(
			"req_%d_%d",
			time.Now().UTC().UnixMilli(),
			atomic.AddUint64(&requestCounter, 1),
		)

		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestIDFrom(r *http.Request) string {
	value, _ := r.Context().Value(requestIDKey).(string)
	return value
}
