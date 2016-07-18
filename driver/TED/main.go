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
	bw.SetEntityFromEnvironOrExit()

	url := params.MustString("URL")
	toExtract := params.MustStringSlice("extract")
	fmt.Println(toExtract)
	baseuri := params.MustString("svc_base_uri")
	poll_interval := params.MustString("poll_interval")

	params.MergeMetadata(bw)

	svc := bw.RegisterService(baseuri, "s.TED")
	fmt.Println(svc.FullURI())
	meters := make(map[string]*bw2.Interface)
	uuids := make(map[string]string)

	for _, name := range toExtract {
		iface := svc.RegisterInterface(name, "i.meter")
		meters[name] = iface
		uuids[name+"voltage"] = uuid.NewV3(NAMESPACE_UUID, name+"voltage").String()
		uuids[name+"powernow"] = uuid.NewV3(NAMESPACE_UUID, name+"powernow").String()
		uuids[name+"kva"] = uuid.NewV3(NAMESPACE_UUID, name+"kva").String()
		fmt.Println(iface.FullURI())
	}

	src := NewTEDSource(url, poll_interval, toExtract)
	data := src.Start()
	for d := range data {
		fmt.Printf("Values: %+v\n", d)
		volt_msg := TimeseriesReading{UUID: uuids[d.Name+"voltage"], Time: time.Now().Unix(), Value: d.VoltageNow}
		meters[d.Name].PublishSignal("Voltage", volt_msg.ToMsgPackBW())

		power_msg := TimeseriesReading{UUID: uuids[d.Name+"powernow"], Time: time.Now().Unix(), Value: d.PowerNow}
		meters[d.Name].PublishSignal("PowerNow", power_msg.ToMsgPackBW())

		ap_msg := TimeseriesReading{UUID: uuids[d.Name+"kva"], Time: time.Now().Unix(), Value: d.KVA}
		meters[d.Name].PublishSignal("KVA", ap_msg.ToMsgPackBW())
	}

}
