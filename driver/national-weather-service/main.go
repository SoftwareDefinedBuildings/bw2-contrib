package main

import (
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
	Temperature      float64 `msgpack:"temperature"`
	RelativeHumidity float64 `msgpack:"relative_humidity"`
	WindSpeed        float64 `msgpack:"wind_speed"`
	WindDirection    float64 `msgpack:"wind_direction"`
	CloudCover       Cloud   `msgpack:"cloud_coverage"`
	CloudHeight      float64 `msgpack:"cloud_height"`
	Time             int64   `msgpack:"time"`
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

		t := point.Resp.Properties.Temperature.Value
		if point.Resp.Properties.Temperature.UnitCode == "unit:degC" {
			t = 1.8*t + 32
		} else {
			log.Printf("BAD TEMP UNIT %+v", point)
			continue
		}

		if point.Resp.Properties.RelativeHumidity.UnitCode != "unit:percent" {
			log.Printf("BAD RH UNIT %+v", point)
			continue
		}

		if point.Resp.Properties.WindSpeed.UnitCode != "unit:m_s-1" {
			log.Printf("BAD WINDSPEED UNIT %+v", point)
			continue
		}

		if point.Resp.Properties.WindDirection.UnitCode != "unit:degree_(angle)" {
			log.Printf("BAD WINDDIRECTION UNIT %+v", point)
			continue
		}

		var send XBOS_WEATHER_STATION
		if len(point.Resp.Properties.CloudLayers) > 0 {
			cloud := point.Resp.Properties.CloudLayers[0]
			id := parseCloud(cloud.Amount)
			log.Println("id", id)
			if cloud.Base.UnitCode != "unit:m" {
				log.Printf("BAD CLOUDHEIGHT UNIT %+v", point)
				continue
			}
			send = XBOS_WEATHER_STATION{
				Temperature:      t,
				RelativeHumidity: point.Resp.Properties.RelativeHumidity.Value,
				WindSpeed:        point.Resp.Properties.WindSpeed.Value,
				WindDirection:    point.Resp.Properties.WindDirection.Value,
				CloudCover:       id,
				CloudHeight:      cloud.Base.Value,
				Time:             time.Now().UnixNano(),
			}
		} else {
			send = XBOS_WEATHER_STATION{
				Temperature:      t,
				RelativeHumidity: point.Resp.Properties.RelativeHumidity.Value,
				WindSpeed:        point.Resp.Properties.WindSpeed.Value,
				WindDirection:    point.Resp.Properties.WindDirection.Value,
				Time:             time.Now().UnixNano(),
			}
		}

		po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(WEATHERSTATION_DF), send)
		if err != nil {
			log.Println("Could not publish", err)
			continue
		}
		iface.PublishSignal("info", po)
	}
}
