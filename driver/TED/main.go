package main

import (
	"fmt"
	"github.com/gtfierro/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
	"log"
	"time"
)

const PONUM = `2.1.1.4`

//func (msg TimeseriesReading) ToMsgPackBW() (po bw2.PayloadObject) {
//	po, _ = bw2.CreateMsgPackPayloadObject(bw2.FromDotForm("2.0.9.1"), msg)
//	return
//}

type XBOSMeter struct {
	Power          float64 `msgpack:"power"`
	Voltage        float64 `msgpack:"voltage"`
	Apparent_power float64 `msgpack:"apparent_power"`
	Time           int64   `msgpack:"time"`
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

	svc := bw.RegisterService(baseuri, "s.ted")
	fmt.Println(svc.FullURI())
	meters := make(map[string]*bw2.Interface)

	//for _, name := range toExtract {
	//	iface := svc.RegisterInterface(name, "i.meter")
	//	meters[name] = iface
	//	uuids[name+"voltage"] = uuid.NewV3(NAMESPACE_UUID, name+"voltage").String()
	//	uuids[name+"powernow"] = uuid.NewV3(NAMESPACE_UUID, name+"powernow").String()
	//	uuids[name+"kva"] = uuid.NewV3(NAMESPACE_UUID, name+"kva").String()
	//	fmt.Println(iface.FullURI())
	//}

	src := NewTEDSource(url, poll_interval, toExtract)
	data := src.Start()
	for d := range data {
		fmt.Printf("Values: %+v\n", d)
		iface, found := meters[d.Name]
		if !found {
			iface = svc.RegisterInterface(d.Name, "i.xbos.meter")
			meters[d.Name] = iface
		}
		msg := XBOSMeter{
			Time:           time.Now().UnixNano(),
			Voltage:        d.Voltage.VoltageNow,
			Apparent_power: d.Power.KVA,
			Power:          d.Power.PowerNow,
		}
		if po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(PONUM), msg); err != nil {
			log.Println(err)
		} else if err = iface.PublishSignal("info", po); err != nil {
			log.Println(err)
		}
	}

}
