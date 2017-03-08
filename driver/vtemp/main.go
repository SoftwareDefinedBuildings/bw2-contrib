package main

import (
	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

type Reading struct {
	Time int64
	Temperature float64
}

func (r *Reading) ToMsgPackPO(ponum int) (bo bw2.PayloadObject) {
	po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(PONUM), r)
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
		reading := Info {
			Time: time.Now().UnixNano(),
			Temperature: point,
		}
		iface.PublishSignal("info", reading.ToMsgPackPO())
	}
}
