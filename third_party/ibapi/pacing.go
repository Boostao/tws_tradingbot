package ibapi

import (
	"sync"
	"time"
)

// Pacer implements a token bucket rate limiter.
type Pacer struct {
	mu           sync.Mutex
	tokens       int
	maxTokens    int
	lastRefill   time.Time
	refillRate   time.Duration // Time to add 1 token
}

func NewPacer(maxTokens int, refillRate time.Duration) *Pacer {
	return &Pacer{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		lastRefill: time.Now(),
		refillRate: refillRate,
	}
}

func (p *Pacer) Wait() {
	p.mu.Lock()
	for p.tokens <= 0 {
		p.refill()
		if p.tokens > 0 {
			break
		}
		p.mu.Unlock()
		time.Sleep(10 * time.Millisecond) // Wait a bit
		p.mu.Lock()
	}
	p.tokens--
	p.mu.Unlock()
}

func (p *Pacer) refill() {
	now := time.Now()
	elapsed := now.Sub(p.lastRefill)
	if elapsed >= p.refillRate {
		tokensToAdd := int(elapsed / p.refillRate)
		p.tokens += tokensToAdd
		if p.tokens > p.maxTokens {
			p.tokens = p.maxTokens
		}
		p.lastRefill = now
	}
}
