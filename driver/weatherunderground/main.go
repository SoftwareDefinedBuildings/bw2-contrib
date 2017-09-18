package main

import (
	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
	"log"
	"time"
)

const WEATHERSTATION_DF = "2.1.1.8"

type XBOS_WEATHER_STATION struct {
	Temperature      float64 `msgpack:"temperature"`
	RelativeHumidity float64 `msgpack:"relative_humidity"`
	WindSpeed        float64 `msgpack:"wind_speed"`
	Time             int64   `msgpack:"time"`
}

func main() {
	bw := bw2.ConnectOrExit("")

	params := spawnable.GetParamsOrExit()
	apikey := params.MustString("API_KEY")
	location := params.MustString("location")
	baseuri := params.MustString("svc_base_uri")
	read_rate := params.MustString("read_rate")

	bw.OverrideAutoChainTo(true)
	bw.SetEntityFromEnvironOrExit()
	svc := bw.RegisterService(baseuri, "s.weatherunderground")
	iface := svc.RegisterInterface(location, "i.xbos.weather_station")

	params.MergeMetadata(bw)

	// generate UUIDs from location + metric name
	src := NewWeatherUndergroundSource(apikey, location, read_rate)
	data := src.Start()
	for point := range data {
		signal := XBOS_WEATHER_STATION{
			Temperature:      point.Temperature,
			RelativeHumidity: point.RH,
			WindSpeed:        point.WindSpeed,
			Time:             time.Now().UnixNano(),
		}
		log.Printf("%+v", signal)
		po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(WEATHERSTATION_DF), signal)
		if err != nil {
			log.Println("Could not publish", err)
			continue
		}
		iface.PublishSignal("info", po)
	}
}
