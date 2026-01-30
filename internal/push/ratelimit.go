package push

import "time"

type RateLimiter struct {
	ch chan struct{}
}

func NewRateLimiter(maxPerMinute int) *RateLimiter {
	if maxPerMinute <= 0 {
		return &RateLimiter{ch: nil}
	}
	ch := make(chan struct{}, maxPerMinute)
	for i := 0; i < maxPerMinute; i++ {
		ch <- struct{}{}
	}
	rl := &RateLimiter{ch: ch}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			for i := 0; i < maxPerMinute; i++ {
				select {
				case rl.ch <- struct{}{}:
				default:
					break
				}
			}
		}
	}()
	return rl
}

func (r *RateLimiter) Allow() bool {
	if r.ch == nil {
		return true
	}
	select {
	case <-r.ch:
		return true
	default:
		return false
	}
}
