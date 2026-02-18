package dnsproxy

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/AdguardTeam/dnsproxy/upstream"
	"github.com/miekg/dns"
)

const (
	healthCheckInterval      = 10 * time.Second
	unhealthyRecheckInterval = 30 * time.Second
	probeTimeout             = 3 * time.Second
	failureThreshold         = 3
	latencyAlpha             = 0.3
)

// UpstreamState tracks the health and performance of a single upstream.
type UpstreamState struct {
	Upstream   upstream.Upstream
	Address    string
	Healthy    bool
	AvgLatency time.Duration
	Failures   int
	LastCheck  time.Time
	mu         sync.RWMutex
}

// HealthAwareUpstream implements upstream.Upstream by routing each query to
// the fastest healthy upstream, with sequential fallback to others on failure.
// This avoids fan-out which would cause duplicate queries at DNS tunnel servers.
type HealthAwareUpstream struct {
	states []*UpstreamState
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

// NewHealthAwareUpstream creates a new health-aware upstream wrapper.
func NewHealthAwareUpstream(upstreams []upstream.Upstream) *HealthAwareUpstream {
	states := make([]*UpstreamState, len(upstreams))
	for i, u := range upstreams {
		states[i] = &UpstreamState{
			Upstream: u,
			Address:  u.Address(),
			Healthy:  true,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	h := &HealthAwareUpstream{
		states: states,
		ctx:    ctx,
		cancel: cancel,
	}

	h.wg.Add(1)
	go h.monitorLoop()

	return h
}

// Exchange sends the DNS request to the fastest healthy upstream. If it fails,
// it falls back to the next-fastest. This avoids parallel fan-out which would
// send duplicate data packets to DNS tunnel servers.
func (h *HealthAwareUpstream) Exchange(req *dns.Msg) (*dns.Msg, error) {
	ordered := h.orderedHealthyStates()

	// If all unhealthy, try all as fallback
	if len(ordered) == 0 {
		ordered = h.allStates()
	}

	if len(ordered) == 0 {
		return nil, fmt.Errorf("no upstreams available")
	}

	var lastErr error
	for _, s := range ordered {
		resp, err := s.Upstream.Exchange(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// Address returns a descriptive address for this composite upstream.
func (h *HealthAwareUpstream) Address() string {
	return "health-aware-proxy"
}

// Close stops the health monitor and closes all underlying upstreams.
func (h *HealthAwareUpstream) Close() error {
	h.cancel()
	h.wg.Wait()

	var firstErr error
	for _, s := range h.states {
		if err := s.Upstream.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// GetStatus returns a snapshot of all upstream states.
func (h *HealthAwareUpstream) GetStatus() []UpstreamStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]UpstreamStatus, len(h.states))
	for i, s := range h.states {
		s.mu.RLock()
		result[i] = UpstreamStatus{
			Address:    s.Address,
			Healthy:    s.Healthy,
			AvgLatency: s.AvgLatency,
			Failures:   s.Failures,
		}
		s.mu.RUnlock()
	}
	return result
}

// UpstreamStatus is a read-only snapshot of an upstream's state.
type UpstreamStatus struct {
	Address    string
	Healthy    bool
	AvgLatency time.Duration
	Failures   int
}

// orderedHealthyStates returns healthy upstreams sorted by latency (fastest first).
func (h *HealthAwareUpstream) orderedHealthyStates() []*UpstreamState {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []*UpstreamState
	for _, s := range h.states {
		s.mu.RLock()
		healthy := s.Healthy
		s.mu.RUnlock()
		if healthy {
			result = append(result, s)
		}
	}

	// Sort by measured latency (fastest first). Unmeasured (0) sorts last
	// to preserve config order at startup and avoid trying dead upstreams first.
	slices.SortStableFunc(result, func(a, b *UpstreamState) int {
		a.mu.RLock()
		la := a.AvgLatency
		a.mu.RUnlock()
		b.mu.RLock()
		lb := b.AvgLatency
		b.mu.RUnlock()
		switch {
		case la == 0 && lb == 0:
			return 0 // both unmeasured: preserve original order
		case la == 0:
			return 1 // a unmeasured, sort after b
		case lb == 0:
			return -1 // b unmeasured, sort after a
		default:
			return cmp.Compare(la, lb)
		}
	})

	return result
}

func (h *HealthAwareUpstream) allStates() []*UpstreamState {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]*UpstreamState, len(h.states))
	copy(result, h.states)
	return result
}

func (h *HealthAwareUpstream) monitorLoop() {
	defer h.wg.Done()

	healthyTicker := time.NewTicker(healthCheckInterval)
	unhealthyTicker := time.NewTicker(unhealthyRecheckInterval)
	defer healthyTicker.Stop()
	defer unhealthyTicker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-healthyTicker.C:
			h.probeUpstreams(true)
		case <-unhealthyTicker.C:
			h.probeUpstreams(false)
		}
	}
}

func (h *HealthAwareUpstream) probeUpstreams(healthyOnly bool) {
	h.mu.RLock()
	states := make([]*UpstreamState, len(h.states))
	copy(states, h.states)
	h.mu.RUnlock()

	for _, s := range states {
		s.mu.RLock()
		isHealthy := s.Healthy
		s.mu.RUnlock()

		if healthyOnly && !isHealthy {
			continue
		}
		if !healthyOnly && isHealthy {
			continue
		}

		go h.probeOne(s)
	}
}

func (h *HealthAwareUpstream) probeOne(s *UpstreamState) {
	msg := new(dns.Msg)
	msg.SetQuestion(".", dns.TypeNS)

	start := time.Now()

	ctx, cancel := context.WithTimeout(h.ctx, probeTimeout)
	defer cancel()

	// Run exchange with timeout via context
	type result struct {
		err error
	}
	ch := make(chan result, 1)
	go func() {
		_, err := s.Upstream.Exchange(msg)
		ch <- result{err: err}
	}()

	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case r := <-ch:
		err = r.err
	}

	latency := time.Since(start)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.LastCheck = time.Now()

	if err != nil {
		s.Failures++
		if s.Failures >= failureThreshold {
			s.Healthy = false
		}
		return
	}

	// Success
	if s.AvgLatency == 0 {
		s.AvgLatency = latency
	} else {
		s.AvgLatency = time.Duration(
			float64(s.AvgLatency)*(1-latencyAlpha) + float64(latency)*latencyAlpha,
		)
	}

	s.Failures = 0
	s.Healthy = true
}
