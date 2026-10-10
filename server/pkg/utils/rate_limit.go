package utils

import (
	"github.com/ulule/limiter/v3"
)

func NewRateLimiter(interval string) *limiter.Limiter {
	rate, err := limiter.NewRateFromFormatted(interval)
	if err != nil {
		panic(err)
	}
	return limiter.New(newMemoryLimiterStore(), rate)
}

func NewRateLimiterForRate(rate limiter.Rate) *limiter.Limiter {
	return limiter.New(newMemoryLimiterStore(), rate)
}
