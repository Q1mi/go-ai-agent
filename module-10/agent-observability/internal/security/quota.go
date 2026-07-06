package security

import (
	"errors"
	"sync"

	"golang.org/x/time/rate"
)

// RateLimiter 是按用户维度的令牌桶限流器。
type RateLimiter struct {
	mu      sync.Mutex
	perUser map[string]*rate.Limiter
	r       rate.Limit
	burst   int
}

// NewRateLimiter 创建限流器。
func NewRateLimiter(perSec float64, burst int) *RateLimiter {
	if perSec <= 0 {
		perSec = 1
	}
	if burst <= 0 {
		burst = 1
	}
	return &RateLimiter{perUser: map[string]*rate.Limiter{}, r: rate.Limit(perSec), burst: burst}
}

// Allow 检查用户是否允许本次请求。
func (limiter *RateLimiter) Allow(user string) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	lim, ok := limiter.perUser[user]
	if !ok {
		lim = rate.NewLimiter(limiter.r, limiter.burst)
		limiter.perUser[user] = lim
	}
	return lim.Allow()
}

// ErrQuotaExceeded 表示 token 配额不足。
var ErrQuotaExceeded = errors.New("已超出本周期 token 配额")

// TokenQuota 记录每个用户的 token 消耗。
type TokenQuota struct {
	mu   sync.Mutex
	used map[string]int
	cap  int
}

// NewTokenQuota 创建配额控制器。
func NewTokenQuota(cap int) *TokenQuota {
	if cap <= 0 {
		cap = 1
	}
	return &TokenQuota{used: map[string]int{}, cap: cap}
}

// Charge 扣减 token 配额。
func (quota *TokenQuota) Charge(user string, tokens int) error {
	quota.mu.Lock()
	defer quota.mu.Unlock()
	if quota.used[user]+tokens > quota.cap {
		return ErrQuotaExceeded
	}
	quota.used[user] += tokens
	return nil
}

// Used 返回用户已用 token。
func (quota *TokenQuota) Used(user string) int {
	quota.mu.Lock()
	defer quota.mu.Unlock()
	return quota.used[user]
}

// Reset 重置配额周期。
func (quota *TokenQuota) Reset() {
	quota.mu.Lock()
	defer quota.mu.Unlock()
	quota.used = map[string]int{}
}
