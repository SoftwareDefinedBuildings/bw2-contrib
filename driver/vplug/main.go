package main

import (
	"fmt"
	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
	"time"
)

type InfoData struct {
	Time int64
	State bool
}

func (i *InfoData) ToMsgPackPO() (bo bw2.PayloadObject) {
	po, err := bw2.CreateMsgPackPayloadObject(bw2.PONumTimeseriesReading, i)
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
	pollInt, err := time.ParseDuration(poll_interval)
	if err != nil {
		panic(err)
	}

	service := bwClient.RegisterService(baseuri, "s.vplug")
	iface := service.RegisterInterface("vplug", "i.xbos.plug")

	params.MergeMetadata(bwClient)

	v := NewVplug()

	iface.SubscribeSlot("state", func(msg *bw2.SimpleMessage) {
		po := msg.GetOnePODF(bw2.PODFBinaryActuation)
		if po == nil {
			fmt.Println("Received actuation command without valid PO, dropping")
			return
		} else if len(po.GetContents()) < 1 {
			fmt.Println("Received actuation command with invalid PO, dropping")
			return
		}

		if po.GetContents()[0] == 0 {
			v.ActuatePlug(false)
		} else if po.GetContents()[0] == 1 {
			v.ActuatePlug(true)
		} else {
			fmt.Println("Actuation command contents must be 0 or 1, dropping")
		}
	})

	for {
		status := v.GetStatus()
		timestamp := time.Now().UnixNano()
		msg := InfoData {
			Time: timestamp,
			State: status,
		}

		iface.PublishSignal("state", msg.ToMsgPackPO())
		time.Sleep(pollInt)
	}
}
