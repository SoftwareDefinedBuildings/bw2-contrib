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
	baseuri := params.MustString("svc_base_uri")
	read_rate := params.MustString("read_rate")

	bw.OverrideAutoChainTo(true)
	bw.SetEntityFromEnvironOrExit()
	svc := bw.RegisterService(baseuri, "s.caiso")
	iface := svc.RegisterInterface("_", "i.production")

	params.MergeMetadata(bw)

	fmt.Println(iface.FullURI())
	fmt.Println(iface.SignalURI("solar"))

	solar_uuid := uuid.NewV3(NAMESPACE_UUID, "solar").String()
	wind_uuid := uuid.NewV3(NAMESPACE_UUID, "wind").String()

	src := NewCaisoEnergySource(read_rate)
	data := src.Start()
	for point := range data {
		fmt.Println(point)
		temp_f := TimeseriesReading{UUID: solar_uuid, Time: time.Now().Unix(), Value: point.SolarProd}
		iface.PublishSignal("solar", temp_f.ToMsgPackBW())

		temp_c := TimeseriesReading{UUID: wind_uuid, Time: time.Now().Unix(), Value: point.WindProd}
		iface.PublishSignal("wind", temp_c.ToMsgPackBW())
	}
}