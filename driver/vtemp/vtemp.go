package main

import (
	"math"
	"math/rand"
	"time"
)

const (
	BASE = 75
	FLUX = 5
	JITTER = 0.5
	SECONDS_IN_DAY = 86400
)

type Vtemp struct {
	random *rand.Rand
	rate time.Duration
	data chan float64
}

func NewVtemp(rate string) *Vtemp {
	dur, err := time.ParseDuration(rate)
	if err != nil {
		panic(err)
	}

	s1 := rand.NewSource(time.Now().UnixNano())
	return &Vtemp{
		random: rand.New(s1),
		rate: dur,
		data: make(chan float64),
	}
}

func (v *Vtemp) Start() chan float64 {
	go func() {
		for _ = range time.Tick(v.rate) {
			v.data <- v.getTemp()
		}
	}()
	return v.data
}

func (v *Vtemp) getTemp() float64 {
	return v.generateTemp()
}

func (v *Vtemp) generateTemp() float64 {
	currentTime := time.Now()
	currentTimeSeconds := float64((currentTime.Hour() * 3600) + (currentTime.Minute() * 60) + currentTime.Second())
	timeFlux := (FLUX * math.Sin(float64(currentTimeSeconds / SECONDS_IN_DAY) * 2.0 * math.Pi))
	jitterDifference := (JITTER * (v.random.Float64() * 2 - 1))
	temp := BASE + timeFlux + jitterDifference
	return temp
}
