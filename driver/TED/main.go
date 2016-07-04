package main

import (
	"fmt"
	"github.com/gtfierro/spawnpoint/spawnable"
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
	bw.OverrideAutoChainTo(true)
	//TODO: is there a standard key I should be using?
	bw.SetEntityFileOrExit(params.MustString("entityfile"))

	url := params.MustString("URL")
	toExtract := params.MustStringSlice("extract")
	fmt.Println(toExtract)
	name := params.MustString("name")
	baseuri := params.MustString("svc_base_uri")
	poll_interval := params.MustString("poll_interval")

	//TODO: needs discussion on behavior of MergeMetadat
	//params.MergeMetadata(bw)

	svc := bw.RegisterService(baseuri, "s.TED")
	iface := svc.RegisterInterface(name, "i.meter")
	bw.SetMetadata(iface.SignalURI("Voltage"), "UnitofMeasure", "V")
	bw.SetMetadata(iface.SignalURI("RealPower"), "UnitofMeasure", "V")
	bw.SetMetadata(iface.SignalURI("ApparentPower"), "UnitofMeasure", "VA")
	bw.SetMetadata(iface.FullURI(), "SourceName", params.MustString("SourceName"))

	fmt.Println(svc.FullURI())
	fmt.Println(iface.FullURI())

	voltage_uuid := uuid.NewV3(NAMESPACE_UUID, name+"voltage").String()
	realpower_uuid := uuid.NewV3(NAMESPACE_UUID, name+"realpower").String()
	apparentpower_uuid := uuid.NewV3(NAMESPACE_UUID, name+"apparentpower").String()

	src := NewTEDSource(url, poll_interval, toExtract)
	data := src.Start()
	for d := range data {
		volt_msg := TimeseriesReading{UUID: voltage_uuid, Time: time.Now().Unix(), Value: d.VoltageNow}
		iface.PublishSignal("Voltage", volt_msg.ToMsgPackBW())

		power_msg := TimeseriesReading{UUID: realpower_uuid, Time: time.Now().Unix(), Value: d.PowerNow}
		iface.PublishSignal("RealPower", power_msg.ToMsgPackBW())

		ap_msg := TimeseriesReading{UUID: apparentpower_uuid, Time: time.Now().Unix(), Value: d.KVA}
		iface.PublishSignal("ApparentPower", ap_msg.ToMsgPackBW())
	}

}
