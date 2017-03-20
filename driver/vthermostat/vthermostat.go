package main

import (
	"math"
	"time"
)

const (
	BASE_TEMP = 78
	BASE_HSETPOINT = 60
	BASE_CSETPOINT = 90
	E = 0.1
	FLUX = 10
	THERMAL_RESISTANCE = 0.1

	DEW_POINT = 50.0
	REL_HUMIDITY_A = 5.656854249
	REL_HUMIDITY_B = 0.9659363289
)

type Vthermostat struct {
	data chan Point
	rate time.Duration
	temperature float64
	relativeHumidity float64
	heatingSetpoint float64
	coolingSetpoint float64
	override bool
	fan bool
	mode int
	state int
}

type Point struct {
	temperature float64
	relativeHumidity float64
	heatingSetpoint float64
	coolingSetpoint float64
	override bool
	fan bool
	mode int
	state int
}

func NewVthermostat(rate string) *Vthermostat {
	dur, err := time.ParseDuration(rate)
	if err != nil {
		panic(err)
	}

	return &Vthermostat{
		data: make(chan Point),
		rate: dur,
		temperature: BASE_TEMP,
		relativeHumidity: 0.0,
		heatingSetpoint: BASE_HSETPOINT,
		coolingSetpoint: BASE_CSETPOINT,
		override: false,
		fan: false,
		mode: 0,
		state: 0,
	}
}

func (v *Vthermostat) setHeatingSetpoint(point float64) {
	v.heatingSetpoint = point
}

func (v *Vthermostat) setCoolingSetpoint(point float64) {
	v.coolingSetpoint = point
}

func (v *Vthermostat) setOverride(override bool) {
	v.override = override
}

func (v *Vthermostat) setFan(fan bool) {
	v.fan = fan
}

func (v *Vthermostat) setMode(mode int) {
	v.mode = mode
}

func (v *Vthermostat) setState(state int) { //state is determined by schedule
	v.state = state
}

func (v *Vthermostat) generateOutsideAirTemp() float64 {
	currentTime := time.Now()
	currentTimeSeconds := float64((currentTime.Hour() * 3600) + (currentTime.Minute() * 60) + currentTime.Second())
	temp := BASE_TEMP + (FLUX * math.Sin(0.01 * currentTimeSeconds))
	return temp
}

func (v *Vthermostat) generateRelativeHumidity(temp float64) float64 {
	if temp < DEW_POINT {
		temp = DEW_POINT
	}
	return REL_HUMIDITY_A * math.Pow(REL_HUMIDITY_B, temp)
}

func (v *Vthermostat) generateRoomTemp(outsideAirTemp float64) float64 {
	cooling, heating := 0, 0
	if v.temperature < v.heatingSetpoint {
		heating = 1
	} else if v.temperature > v.coolingSetpoint {
		cooling = 1
	}

	switch v.mode {
		case 0:
			cooling, heating = 0, 0
		case 1:
			cooling = 0
		case 2:
			heating = 0
		case 3:
			break
	}

	deltaT := outsideAirTemp - v.temperature
	temp := v.temperature + (THERMAL_RESISTANCE * deltaT) - (float64(cooling) * E * deltaT) + (float64(heating) * E * deltaT) //T + (thermal_resistance * delta_t) - (cooling * e * delta_t) + (heat * e * delta_t)
	return temp
}

func (v *Vthermostat) Start() chan Point {
	go func() {
		for _ = range time.Tick(v.rate) {
			v.data <- v.run()
		}
	}()
	return v.data
}

func (v *Vthermostat) run() Point {
	oat := v.generateOutsideAirTemp()
	v.temperature = v.generateRoomTemp(oat)
	v.relativeHumidity = v.generateRelativeHumidity(v.temperature)
	return v.getPoint()
}

func (v *Vthermostat) getPoint() Point {
	return Point {
		temperature: v.temperature,
		relativeHumidity: v.relativeHumidity,
		heatingSetpoint: v.heatingSetpoint,
		coolingSetpoint: v.coolingSetpoint,
		override: v.override,
		fan: v.fan,
		mode: v.mode,
		state: v.state,
	}
}
