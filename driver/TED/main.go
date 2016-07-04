package main

import (
	"fmt"
	//_ msgs "github.com/gtfierro/giles2/plugins/bosswave"
	"github.com/immesys/spawnpoint/spawnable"
	"github.com/satori/go.uuid"
	bw2 "gopkg.in/immesys/bw2bind.v5"
	// "time"
)

var NAMESPACE_UUID uuid.UUID

func init() {
	NAMESPACE_UUID = uuid.FromStringOrNil("d8b61708-2797-11e6-836b-0cc47a0f7eea")
}

func main() {
	bw := bw2.ConnectOrExit("")

	params := spawnable.GetParamsOrExit()
	url := params.MustString("URL")
	toExtract := params.MustStringSlice("extract")
	fmt.Println(toExtract)
	//name := params.MustString("name")
	poll_interval := params.MustString("poll_interval")

	bw.OverrideAutoChainTo(true)
	bw.SetEntityFromEnvironOrExit()
	svc := bw.RegisterService(baseuri, "s.TED")
	iface := svc.RegisterInterface(name, "i.meter")
	bw2.SetMetadata(iface.SignalURI("Voltage"), "UnitofMeasure", "V")
	bw2.SetMetadata(iface.SignalURI("RealPower"), "UnitofMeasure", "V")
	bw2.SetMetadata(iface.SignalURI("ApparentPower"), "UnitofMeasure", "VA")

	src := NewTEDSource(url, poll_interval, toExtract)
	data := src.Start()
	for d := range data {
		fmt.Println(d)
	}

}
