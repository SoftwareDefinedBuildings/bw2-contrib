package main

import (
	"fmt"
	msgs "github.com/gtfierro/giles2/plugins/bosswave"
	"github.com/immesys/spawnpoint/spawnable"
	"github.com/satori/go.uuid"
	bw2 "gopkg.in/immesys/bw2bind.v5"
	"time"
)

var NAMESPACE_UUID uuid.UUID

func init() {
	NAMESPACE_UUID = uuid.FromStringOrNil("d8b61708-2797-11e6-836b-0cc47a0f7eea")
}

func main() {
	bw := bw2.ConnectOrExit("")

	params := spawnable.GetParamsOrExit()
	apikey := params.MustString("API_KEY")
	city := params.MustString("city")
	baseuri := params.MustString("path")
	read_rate := params.MustString("read_rate")

	bw.OverrideAutoChainTo(true)
	bw.SetEntityFromEnvironOrExit()
	svc := bw.RegisterService(baseuri, "s.weatherunderground")
	iface := svc.RegisterInterface(city, "i.weather")
	iface.SetMetadata("archive", "true")

	fmt.Println(iface.FullURI())
	fmt.Println(iface.SignalURI("fahrenheit"))

	// generate UUIDs from city + metric name
	temp_f_uuid := uuid.NewV3(NAMESPACE_UUID, city+"fahrenheit").String()
	temp_c_uuid := uuid.NewV3(NAMESPACE_UUID, city+"celsius").String()
	relative_humidity_uuid := uuid.NewV3(NAMESPACE_UUID, city+"relative_humidity").String()

	bw.PublishOrExit(&bw2.PublishParams{
		URI:            iface.SignalURI("fahrenheit") + "/!meta/archive",
		PayloadObjects: []bw2.PayloadObject{bw2.CreateStringPayloadObject(iface.SignalURI("fahrenheit"))},
		Persist:        true,
	})

	bw.PublishOrExit(&bw2.PublishParams{
		URI:            iface.SignalURI("celsius") + "/!meta/archive",
		PayloadObjects: []bw2.PayloadObject{bw2.CreateStringPayloadObject(iface.SignalURI("celsius"))},
		Persist:        true,
	})

	bw.PublishOrExit(&bw2.PublishParams{
		URI:            iface.SignalURI("relative_humidity") + "/!meta/archive",
		PayloadObjects: []bw2.PayloadObject{bw2.CreateStringPayloadObject(iface.SignalURI("relative_humidity"))},
		Persist:        true,
	})

	src := NewWeatherUndergroundSource(apikey, city, read_rate)
	data := src.Start()
	for point := range data {
		fmt.Println(point)
		temp_f := msgs.Point{Time: uint64(time.Now().Unix()), Value: point.F}
		temp_f_msg := msgs.Timeseries{UUID: temp_f_uuid, Data: []msgs.Point{temp_f}}
		iface.PublishSignal("fahrenheit", temp_f_msg.ToMsgPackBW())

		temp_c := msgs.Point{Time: uint64(time.Now().Unix()), Value: point.C}
		temp_c_msg := msgs.Timeseries{UUID: temp_c_uuid, Data: []msgs.Point{temp_c}}
		iface.PublishSignal("celsius", temp_c_msg.ToMsgPackBW())

		relative_humidity := msgs.Point{Time: uint64(time.Now().Unix()), Value: point.RH}
		relative_humidity_msg := msgs.Timeseries{UUID: relative_humidity_uuid, Data: []msgs.Point{relative_humidity}}
		iface.PublishSignal("relative_humidity", relative_humidity_msg.ToMsgPackBW())
	}

}
