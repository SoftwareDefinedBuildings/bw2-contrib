package main

import (
	"fmt"
	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
	"time"
)

const (
	PONUM = "2.1.1.1"
)

func NewInfoPO(time int64, state bool, brightness int) bw2.PayloadObject {
	msg := map[string]interface{}{
		"time": time,
		"state": state,
		"brightness": brightness}
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
	pollInt, err := time.ParseDuration(poll_interval)
	if err != nil {
		panic(err)
	}

	service := bwClient.RegisterService(baseuri, "s.vlight")
	iface := service.RegisterInterface("vlight", "i.xbos.light")

	params.MergeMetadata(bwClient)

	v := NewVlight()

	iface.SubscribeSlot("state", func(msg *bw2.SimpleMessage) {
		po := msg.GetOnePODF(PONUM)
		if po == nil {
			fmt.Println("Received actuation command without valid PO, dropping")
			return
		} else if len(po.GetContents()) < 1 {
			fmt.Println("Received actuation command with invalid PO, dropping")
			return
		}

		msgpo, err := bw2.LoadMsgPackPayloadObject(po.GetPONum(), po.GetContents())
		if err != nil {
			fmt.Println(err)
			return
		}

		var data map[string]interface{}
		err = msgpo.ValueInto(&data)
		if err != nil {
			fmt.Println(err)
			return
		}

		v.ActuateLight(data["state"].(bool))
		v.SetBrightness(int(data["brightness"].(uint64)))
	})

	for {
		info := v.GetStatus()
		timestamp := time.Now().UnixNano()
		msg := NewInfoPO(
			timestamp,
			info.State,
			info.Brightness)

		iface.PublishSignal("info", msg)
		time.Sleep(pollInt)
	}
}
