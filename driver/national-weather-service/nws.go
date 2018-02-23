package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/parnurzeal/gorequest"
	"github.com/pkg/errors"
)

type NWS struct {
	stations []string
	contact  string
	rate     time.Duration
	URL      string
	req      *gorequest.SuperAgent
}

func NewNationalWeatherUndergroundSource(stations []string, contact, read_rate string) *NWS {
	dur, err := time.ParseDuration(read_rate)
	if err != nil {
		panic(err)
	}

	nws := &NWS{
		stations: stations,
		rate:     dur,
		contact:  contact,
		URL:      "https://api.weather.gov/stations/%s/observations/current",
		req:      gorequest.New(),
	}

	return nws
}

type Response struct {
	Properties struct {
		Temperature struct {
			UnitCode string  `json:"unitCode"`
			Value    float64 `json:"value"`
		} `json:"temperature"`
		RelativeHumidity struct {
			UnitCode string  `json:"unitCode"`
			Value    float64 `json:"value"`
		} `json:"relativeHumidity"`
		WindSpeed struct {
			UnitCode string  `json:"unitCode"`
			Value    float64 `json:"value"`
		} `json:"windSpeed"`
		WindDirection struct {
			UnitCode string  `json:"unitCode"`
			Value    float64 `json:"value"`
		} `json:"windDirection"`
		CloudLayers []struct {
			Base struct {
				Value    float64 `json:"value"`
				UnitCode string  `json:"unitCode"`
			} `json:"base"`
			Amount string `json:"amount"`
		} `json:"cloudLayers"`
	} `json:"properties"`
}

type Datum struct {
	Resp    Response
	Station string
}

func (nws *NWS) Start() chan Datum {
	ret := make(chan Datum, 10)
	log.Println(nws.contact)

	go func() {
		for _, station := range nws.stations {
			fmt.Println("Read station", station)
			datum, err := nws.Read(station)
			if err != nil {
				log.Printf("Error reading station %s: %s", station, err)
			} else {
				ret <- datum
			}
		}
		for _ = range time.Tick(nws.rate) {
			for _, station := range nws.stations {
				fmt.Println("Read station", station)
				datum, err := nws.Read(station)
				if err != nil {
					log.Printf("Error reading station %s: %s", station, err)
				} else {
					ret <- datum
				}
			}
		}
	}()
	return ret
}

func (nws *NWS) Read(station string) (Datum, error) {
	var d = Datum{
		Station: station,
	}
	log.Println(fmt.Sprintf(nws.URL, station))
	resp, _, errs := nws.req.Get(fmt.Sprintf(nws.URL, station)).
		Set("User-Agent", nws.contact).
		Set("Accept-Encoding", "gzip, deflate").
		Set("Accept", "*/*").
		End()
	if errs != nil {
		return d, errors.Wrap(errs[0], "Could not fetch URL")
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&d.Resp); err != nil {
		return d, errors.Wrap(err, "Could not decode response")
	}
	return d, nil

}
