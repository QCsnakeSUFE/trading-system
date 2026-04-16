package models

import (
	"math"
	"math/rand"
	"time"
)

const (
	tradingDaysPerYear = 252
	tradingHoursPerDay = 24
	secondsPerYear     = tradingDaysPerYear * tradingHoursPerDay * 60 * 60
	callsPerSecond     = 50
)

type PriceSimulator struct {
	currentPrice float64
	mu           float64
	sigma        float64
	dt           float64
	r            *rand.Rand
}

func NewPriceSimulator(startPrice float64) *PriceSimulator {
	return &PriceSimulator{
		currentPrice: startPrice,
		mu:           0.2,
		sigma:        50,
		dt:           1.0 / callsPerSecond / secondsPerYear,
		r:            rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (ps *PriceSimulator) NextPrice() float64 {
	z := ps.r.NormFloat64()

	drift := (ps.mu - 0.5*math.Pow(ps.sigma, 2)) * ps.dt
	diffusion := ps.sigma * math.Sqrt(ps.dt) * z

	ps.currentPrice = ps.currentPrice * math.Exp(drift+diffusion)

	return ps.currentPrice
}
