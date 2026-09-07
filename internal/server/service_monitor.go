// Package server provides service monitoring for uptime and request counting.
package server

import (
	"sync"
	"time"
)

// ServiceMonitor tracks service-level metrics like uptime and request counts.
type ServiceMonitor struct {
	mu            sync.RWMutex
	startTime     time.Time
	checksServed  int64
}

// NewServiceMonitor creates a new ServiceMonitor with the start time set to now.
func NewServiceMonitor() *ServiceMonitor {
	return &ServiceMonitor{
		startTime: time.Now(),
	}
}

// Uptime returns the duration since the service started.
func (sm *ServiceMonitor) Uptime() time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return time.Since(sm.startTime)
}

// StartTime returns the time when the service started.
func (sm *ServiceMonitor) StartTime() time.Time {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.startTime
}

// ChecksServed returns the total number of domain checks served.
func (sm *ServiceMonitor) ChecksServed() int64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.checksServed
}

// IncrementCheck increments the checks served counter by 1.
func (sm *ServiceMonitor) IncrementCheck() {
	sm.mu.Lock()
	sm.checksServed++
	sm.mu.Unlock()
}

// AddChecks adds n to the checks served counter.
func (sm *ServiceMonitor) AddChecks(n int) {
	sm.mu.Lock()
	sm.checksServed += int64(n)
	sm.mu.Unlock()
}
