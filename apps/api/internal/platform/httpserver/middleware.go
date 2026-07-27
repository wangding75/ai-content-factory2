package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// PrincipalProvider is the application-boundary seam for the current
// compatibility principal. Authentication can replace it without changing
// chapter-planning domain logic or handlers.
type PrincipalProvider interface {
	ActorID(context.Context) (string, bool)
}

type systemPrincipalProvider struct{}

func (systemPrincipalProvider) ActorID(context.Context) (string, bool) { return "system", true }

var currentPrincipal PrincipalProvider = systemPrincipalProvider{}

func actorIDFromRequest(r *http.Request) (string, bool) {
	return currentPrincipal.ActorID(r.Context())
}

func requestActorID(r *http.Request) string {
	actorID, _ := actorIDFromRequest(r)
	return actorID
}

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
