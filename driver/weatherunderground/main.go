package main

import (
	"fmt"
	"github.com/immesys/spawnpoint/spawnable"
	"github.com/satori/go.uuid"
	bw2 "gopkg.in/immesys/bw2bind.v5"
	"time"
)

var NAMESPACE_UUID uuid.UUID

func init() {
	NAMESPACE_UUID = uuid.FromStringOrNil("d8b61708-2797-11e6-836b-0cc47a0f7eea")
}

type TimeseriesReading struct {
	UUID  string
	Time  int64
	Value float64
}

func (msg TimeseriesReading) ToMsgPackBW() (po bw2.PayloadObject) {
	po, _ = bw2.CreateMsgPackPayloadObject(bw2.FromDotForm("2.0.9.1"), msg)
	return
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
	iface := svc.RegisterInterface(location, "i.weather")

	params.MergeMetadata(bw)

	fmt.Println(iface.FullURI())
	fmt.Println(iface.SignalURI("fahrenheit"))

	// generate UUIDs from location + metric name
	temp_f_uuid := uuid.NewV3(NAMESPACE_UUID, location+"fahrenheit").String()
	temp_c_uuid := uuid.NewV3(NAMESPACE_UUID, location+"celsius").String()
	relative_humidity_uuid := uuid.NewV3(NAMESPACE_UUID, location+"relative_humidity").String()

	src := NewWeatherUndergroundSource(apikey, location, read_rate)
	data := src.Start()
	for point := range data {
		fmt.Println(point)
		temp_f := TimeseriesReading{UUID: temp_f_uuid, Time: time.Now().Unix(), Value: point.F}
		iface.PublishSignal("fahrenheit", temp_f.ToMsgPackBW())

		temp_c := TimeseriesReading{UUID: temp_c_uuid, Time: time.Now().Unix(), Value: point.C}
		iface.PublishSignal("celsius", temp_c.ToMsgPackBW())

		rh := TimeseriesReading{UUID: relative_humidity_uuid, Time: time.Now().Unix(), Value: point.RH}
		iface.PublishSignal("relative_humidity", rh.ToMsgPackBW())
	}
}
