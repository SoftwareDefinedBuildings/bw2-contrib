package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"strconv"
	"strings"	
	"time"
)

// Object that continuously scrapes for solar and wind energy production
type CaisoEnergySource struct {
	URL string
	rate time.Duration
	data chan EnergyProductionData
}

// Tuple of solar and wind energy production data
type EnergyProductionData struct {
	SolarProd float64
	WindProd float64
}

// Create a new CaisoEnergySource that polls at given rate
func NewCaisoEnergySource(rate string) *CaisoEnergySource {
	dur, err := time.ParseDuration(rate)
	if err != nil {
		panic(err)
	}

	return &CaisoEnergySource {
		URL: "http://content.caiso.com/outlook/SP/renewables.html",
		rate: dur,
		data: make(chan EnergyProductionData),
	}
}

// Starts polling for a given CaisoEnergySource instance.
func (src *CaisoEnergySource) Start() chan EnergyProductionData {
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

// Scrape the Caiso website once for wind and solar production data.
func (src *CaisoEnergySource) Read() (EnergyProductionData, error) {
	doc, err := goquery.NewDocument(src.URL)
	if err != nil {
		return EnergyProductionData{}, err
	}

	var currentSolar string
	var currentWind string

	doc.Find("tr").Each(func(i int, s *goquery.Selection) {
	    currentSolar = s.Find("#currentsolar").Text()
	    currentWind = s.Find("#currentwind").Text()
  	})
	
	currentSolarInt, err := strconv.ParseFloat(strings.Split(currentSolar, " MW")[0], 64)
	if err != nil {
		fmt.Println(err)
	}

	currentWindInt, err := strconv.ParseFloat(strings.Split(currentWind, " MW")[0], 64)
	if err != nil {
		fmt.Println(err)
	}
	return EnergyProductionData{ SolarProd : currentSolarInt, WindProd : currentWindInt }, err
}
