package main

import (
	"encoding/json"
	"fmt"
	"github.com/parnurzeal/gorequest"
	"github.com/pkg/errors"
	"log"
	"strconv"
	"time"
)

type WeatherResponse struct {
	Current WeatherData `json:"current_observation"`
}

type WeatherData struct {
	Temperature float64 `json:"temp_f"`
	RH_s        string  `json:"relative_humidity"`
	RH          float64
	WindSpeed   float64 `json:"wind_mph"`
}

type WeatherUndergroundSource struct {
	key      string
	location string
	URL      string
	rate     time.Duration
	data     chan WeatherData
	req      *gorequest.SuperAgent
}

func NewWeatherUndergroundSource(key, location string, rate string) *WeatherUndergroundSource {
	dur, err := time.ParseDuration(rate)
	if err != nil {
		panic(err)
	}
	return &WeatherUndergroundSource{
		key:      key,
		location: location,
		URL:      fmt.Sprintf("http://api.wunderground.com/api/%s/conditions/q/%s.json", key, location),
		rate:     dur,
		data:     make(chan WeatherData),
		req:      gorequest.New(),
	}
}

func (src *WeatherUndergroundSource) Start() chan WeatherData {
	go func() {
		if point, err := src.Read(); err == nil {
			src.data <- point
		}
		for _ = range time.Tick(src.rate) {
			if point, err := src.Read(); err == nil {
				src.data <- point
			}
		}
	}()
	return src.data
}

func (src *WeatherUndergroundSource) Read() (WeatherData, error) {
	var res WeatherResponse
	log.Println("Reading")
	resp, _, errs := src.req.Get(src.URL).End()
	if errs != nil {
		for _, err := range errs {
			log.Println(errors.Wrap(err, "Could not fetch URL"))
			return WeatherData{}, err
		}
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&res); err != nil {
		log.Println(errors.Wrap(err, "Could not decode API response"))
		return WeatherData{}, err
	}
	var f float64
	var err error
	rh_s := res.Current.RH_s
	if len(rh_s) > 1 {
		tmp := rh_s[:len(rh_s)-1]
		f, err = strconv.ParseFloat(tmp, 64)
	} else {
		return res.Current, errors.New(fmt.Sprintf("Could not parse RH value %v", rh_s))
	}
	res.Current.RH = f
	return res.Current, err
}
