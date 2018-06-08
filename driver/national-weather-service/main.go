package main

import (
	"fmt"
	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
	"log"
	"time"
)

type Cloud int

const (
	CLR Cloud = iota
	FEW
	SCT
	BKN
	OVC
	VV
)

func parseCloud(s string) Cloud {
	switch s {
	case "CLR":
		return CLR
	case "FEW":
		return FEW
	case "SCT":
		return SCT
	case "BKN":
		return BKN
	case "OVC":
		return OVC
	case "VV":
		return VV
	default:
		panic("unknown code: " + s)
	}
}

const WEATHERSTATION_DF = "2.1.1.8"

type XBOS_WEATHER_STATION struct {
	Temperature      *float64 `msgpack:"temperature,omitempty"`
	RelativeHumidity *float64 `msgpack:"relative_humidity,omitempty"`
	WindSpeed        *float64 `msgpack:"wind_speed,omitempty"`
	WindDirection    *float64 `msgpack:"wind_direction,omitempty"`
	CloudCover       *Cloud   `msgpack:"cloud_coverage,omitempty"`
	CloudHeight      *float64 `msgpack:"cloud_height,omitempty"`
	Time             int64    `msgpack:"time"`
}

func (s XBOS_WEATHER_STATION) Dump() {
	if s.Temperature != nil {
		fmt.Printf("Temperature %f  ", *s.Temperature)
	} else {
		fmt.Printf("Temperature NIL  ")
	}

	if s.RelativeHumidity != nil {
		fmt.Printf("RelativeHumidity %f  ", *s.RelativeHumidity)
	} else {
		fmt.Printf("RelativeHumidity NIL  ")
	}

	if s.WindSpeed != nil {
		fmt.Printf("WindSpeed %f  ", *s.WindSpeed)
	} else {
		fmt.Printf("WindSpeed NIL  ")
	}

	if s.WindDirection != nil {
		fmt.Printf("WindDirection %f  ", *s.WindDirection)
	} else {
		fmt.Printf("WindDirection NIL  ")
	}

	if s.CloudCover != nil {
		fmt.Printf("CloudCover %v  ", *s.CloudCover)
	} else {
		fmt.Printf("CloudCover NIL  ")
	}
	if s.CloudHeight != nil {
		fmt.Printf("CloudHeight %v  ", *s.CloudHeight)
	} else {
		fmt.Printf("CloudHeight NIL  ")
	}
	fmt.Printf("\n")
}

func main() {
	bw := bw2.ConnectOrExit("")

	params := spawnable.GetParamsOrExit()
	stations := params.MustStringSlice("stations")
	baseuri := params.MustString("svc_base_uri")
	contact := params.MustString("contact")
	read_rate := params.MustString("read_rate")

	bw.OverrideAutoChainTo(true)
	bw.SetEntityFromEnvironOrExit()
	svc := bw.RegisterService(baseuri, "s.national-weather-service")

	ifaces := make(map[string]*bw2.Interface)
	for _, station := range stations {
		iface := svc.RegisterInterface(station, "i.xbos.weather_station")
		ifaces[station] = iface
	}

	params.MergeMetadata(bw)

	src := NewNationalWeatherUndergroundSource(stations, contact, read_rate)
	log.Println("Starting reading", stations)
	data := src.Start()
	for point := range data {
		log.Printf("%+v", point)
		iface := ifaces[point.Station]

		var send XBOS_WEATHER_STATION
		send.Time = time.Now().UnixNano()

		t := point.Resp.Properties.Temperature.Value
		if t != nil {
			if point.Resp.Properties.Temperature.UnitCode == "unit:degC" {
				_t := 1.8*(*t) + 32
				t = &_t
			} else {
				log.Printf("BAD TEMP UNIT %+v", point)
				continue
			}
			send.Temperature = t
		}

		rh := point.Resp.Properties.RelativeHumidity.Value
		if point.Resp.Properties.RelativeHumidity.UnitCode != "unit:percent" {
			log.Printf("BAD RH UNIT %+v", point)
			continue
		}
		send.RelativeHumidity = rh

		ws := point.Resp.Properties.WindSpeed.Value
		if point.Resp.Properties.WindSpeed.UnitCode != "unit:m_s-1" {
			log.Printf("BAD WINDSPEED UNIT %+v", point)
			continue
		}
		send.WindSpeed = ws

		wd := point.Resp.Properties.WindDirection.Value
		if point.Resp.Properties.WindDirection.UnitCode != "unit:degree_(angle)" {
			log.Printf("BAD WINDDIRECTION UNIT %+v", point)
			continue
		}
		send.WindDirection = wd

		if len(point.Resp.Properties.CloudLayers) > 0 {
			cloud := point.Resp.Properties.CloudLayers[0]
			id := parseCloud(cloud.Amount)
			log.Println("id", id)
			if cloud.Base.UnitCode != "unit:m" {
				log.Printf("BAD CLOUDHEIGHT UNIT %+v", point)
				continue
			}
			send.CloudCover = &id
			send.CloudHeight = cloud.Base.Value
		}
		send.Dump()

		po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(WEATHERSTATION_DF), send)
		if err != nil {
			log.Println("Could not publish", err)
			continue
		}
		iface.PublishSignal("info", po)
	}
}
