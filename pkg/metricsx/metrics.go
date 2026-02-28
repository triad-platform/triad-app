package metricsx

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type durationMetric struct {
	sumNanos atomic.Int64
	count    atomic.Int64
}

type Registry struct {
	namespace string

	mu        sync.RWMutex
	counters  map[string]*atomic.Int64
	durations map[string]*durationMetric
}

func NewRegistry(namespace string) *Registry {
	return &Registry{
		namespace: strings.TrimSpace(namespace),
		counters:  map[string]*atomic.Int64{},
		durations: map[string]*durationMetric{},
	}
}

func (r *Registry) Inc(name string) {
	r.Add(name, 1)
}

func (r *Registry) Add(name string, delta int64) {
	key := r.metricKey(name)

	r.mu.Lock()
	c, ok := r.counters[key]
	if !ok {
		c = &atomic.Int64{}
		r.counters[key] = c
	}
	r.mu.Unlock()

	c.Add(delta)
}

func (r *Registry) ObserveDuration(name string, d time.Duration) {
	key := r.metricKey(name)

	r.mu.Lock()
	m, ok := r.durations[key]
	if !ok {
		m = &durationMetric{}
		r.durations[key] = m
	}
	r.mu.Unlock()

	m.sumNanos.Add(d.Nanoseconds())
	m.count.Add(1)
}

func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		r.mu.RLock()
		counterKeys := make([]string, 0, len(r.counters))
		for k := range r.counters {
			counterKeys = append(counterKeys, k)
		}
		durationKeys := make([]string, 0, len(r.durations))
		for k := range r.durations {
			durationKeys = append(durationKeys, k)
		}
		sort.Strings(counterKeys)
		sort.Strings(durationKeys)

		for _, key := range counterKeys {
			c := r.counters[key]
			fmt.Fprintf(w, "# TYPE %s counter\n", key)
			fmt.Fprintf(w, "%s %d\n", key, c.Load())
		}

		for _, key := range durationKeys {
			dm := r.durations[key]
			sumSeconds := float64(dm.sumNanos.Load()) / float64(time.Second)
			count := dm.count.Load()

			fmt.Fprintf(w, "# TYPE %s_seconds summary\n", key)
			fmt.Fprintf(w, "%s_seconds_sum %.6f\n", key, sumSeconds)
			fmt.Fprintf(w, "%s_seconds_count %d\n", key, count)
		}
		r.mu.RUnlock()
	})
}

func (r *Registry) metricKey(name string) string {
	name = strings.TrimSpace(name)
	if r.namespace == "" {
		return sanitize(name)
	}
	return sanitize(r.namespace + "_" + name)
}

func sanitize(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}
