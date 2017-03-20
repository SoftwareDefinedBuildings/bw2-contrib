package main

import (
	"fmt"
	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
	"time"
)

const (
	PONUM = "2.1.1.0"
)

type Info struct {
	Temperature float64
	RelativeHumidity float64
	HeatingSetpoint float64
	CoolingSetpoint float64
	Override bool
	Fan bool
	Mode int
	State int
	Time int64
}

type Setpoints struct {
	HeatingSetpoint float64
	CoolingSetpoint float64
}

type State struct {
	HeatingSetpoint float64
	CoolingSetpoint float64
	Override bool
	Fan bool
	Mode int
}

func (i *Info) ToMsgPackPO() (bo bw2.PayloadObject) {
	po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(PONUM), i)
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

	service := bwClient.RegisterService(baseuri, "s.vthermostat")
	iface := service.RegisterInterface("vthermostat", "i.xbos.thermostat")

	params.MergeMetadata(bwClient)

	v := NewVthermostat(poll_interval)

	iface.SubscribeSlot("setpoints", func(msg *bw2.SimpleMessage) {
		po := msg.GetOnePODF(PONUM)
		if po == nil {
			fmt.Println("Received actuation command without valid PO, dropping")
			return
		}

		msgpo, err := bw2.LoadMsgPackPayloadObject(po.GetPONum(), po.GetContents())
		if err != nil {
			fmt.Println(err)
			return
		}

		var data Setpoints
		err = msgpo.ValueInto(&data)
		if err != nil {
			fmt.Println(err)
			return
		}

		v.setHeatingSetpoint(data.HeatingSetpoint)
		v.setCoolingSetpoint(data.CoolingSetpoint)
	})

	iface.SubscribeSlot("state", func(msg *bw2.SimpleMessage) {
		po := msg.GetOnePODF(PONUM)
		if po == nil {
			fmt.Println("Received actuation command without valid PO, dropping")
			return
		}

		msgpo, err := bw2.LoadMsgPackPayloadObject(po.GetPONum(), po.GetContents())
		if err != nil {
			fmt.Println(err)
			return
		}

		var data State
		err = msgpo.ValueInto(&data)
		if err != nil {
			fmt.Println(err)
			return
		}

		v.setHeatingSetpoint(data.HeatingSetpoint)
		v.setCoolingSetpoint(data.CoolingSetpoint)
		v.setOverride(data.Override)
		v.setMode(data.Mode)
		v.setFan(data.Fan)
	})

	data := v.Start()
	for point := range data {
		reading := Info {
			Temperature: point.temperature,
			RelativeHumidity: point.relativeHumidity,
			HeatingSetpoint: point.heatingSetpoint,
			CoolingSetpoint: point.coolingSetpoint,
			Override: point.override,
			Fan: point.fan,
			Mode: point.mode,
			State: point.state,
			Time: time.Now().UnixNano(),
		}
		iface.PublishSignal("info", reading.ToMsgPackPO())
	}
}
