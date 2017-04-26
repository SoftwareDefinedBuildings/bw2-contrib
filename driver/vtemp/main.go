package main

import (
	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
	"time"
)

const (
	PONUM = "2.1.2.0"
)

func NewInfoPO(time int64, temp float64) bw2.PayloadObject {
	msg := map[string]interface{}{
		"time": time,
		"temperature": temp}
	po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(PONUM), msg)
	if err != nil {
		panic(err)
	}
	return po
}

func main() {
	bwClient := bw2.ConnectOrExit("")

	params := spawnable.GetParamsOrExit()
	bwClient.OverrideAutoChainTo(true)
	bwClient.SetEntityFromEnvironOrExit()

	baseuri := params.MustString("svc_base_uri")
	poll_interval := params.MustString("poll_interval")

	service := bwClient.RegisterService(baseuri, "s.vtemp")
	iface := service.RegisterInterface("vtemp_sensor", "i.xbos.temperature_sensor")

	params.MergeMetadata(bwClient)

	v := NewVtemp(poll_interval)
	data := v.Start()
	for point := range data {
		po := NewInfoPO(
			time.Now().UnixNano(),
			point)
		iface.PublishSignal("info", po)
	}
}
