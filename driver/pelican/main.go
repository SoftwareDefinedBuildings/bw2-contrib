package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

const TSTAT_PO_DF = "2.1.1.0"

type setpointsMsg struct {
	HeatingSetpoint float64 `msgpack:"heating_setpoint"`
	CoolingSetpoint float64 `msgpack:"cooling_setpoint"`
}

type stateMsg struct {
	HeatingSetpoint float64 `msgpack:"heating_setpoint"`
	CoolingSetpoint float64 `msgpack:"cooling_setpoint"`
	Override        bool    `msgpack:"override"`
	Mode            int32   `msgpack:"mode"`
	Fan             bool    `msgpack:"fan"`
}

func main() {
	bwClient := bw2.ConnectOrExit("")
	bwClient.OverrideAutoChainTo(true)
	bwClient.SetEntityFromEnvironOrExit()

	params := spawnable.GetParamsOrExit()
	baseURI := params.MustString("svc_base_uri")
	if !strings.HasSuffix(baseURI, "/") {
		baseURI += "/"
	}
	username := params.MustString("username")
	password := params.MustString("password")
	sitename := params.MustString("sitename")
	name := params.MustString("name")

	pelican := NewPelican(username, password, sitename, name)
	pollIntStr := params.MustString("poll_interval")
	pollInt, err := time.ParseDuration(pollIntStr)
	if err != nil {
		fmt.Printf("Invalid poll interval specified: %v\n", err)
		os.Exit(1)
	}

	service := bwClient.RegisterService(baseURI, "s.pelican")
	tstatIface := service.RegisterInterface("thermostat", "i.xbos.thermostat")

	tstatIface.SubscribeSlot("setpoints", func(msg *bw2.SimpleMessage) {
		po := msg.GetOnePODF(TSTAT_PO_DF)
		if po == nil {
			fmt.Println("Received message on setpoints slot without required PO. Droping.")
			return
		}

		var setpoints setpointsMsg
		if err := po.(bw2.MsgPackPayloadObject).ValueInto(&setpoints); err != nil {
			fmt.Println("Received malformed PO on setpoints slot. Dropping.")
			return
		}

		if err := pelican.ModifySetpoints(setpoints.HeatingSetpoint, setpoints.CoolingSetpoint); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Set heating setpoint to %v and cooling setpoint to %v\n",
				setpoints.HeatingSetpoint, setpoints.CoolingSetpoint)
		}
	})

	tstatIface.SubscribeSlot("state", func(msg *bw2.SimpleMessage) {
		po := msg.GetOnePODF(TSTAT_PO_DF)
		if po == nil {
			fmt.Println("Received message on state slot without required PO. Dropping.")
			return
		}

		var state stateMsg
		if err := po.(bw2.MsgPackPayloadObject).ValueInto(&state); err != nil {
			fmt.Println("Received malformed PO on state slot. Dropping.")
			return
		}

		params := pelicanStateParams{
			HeatingSetpoint: state.HeatingSetpoint,
			CoolingSetpoint: state.CoolingSetpoint,
			Override:        state.Override,
			Mode:            state.Mode,
			Fan:             state.Fan,
		}
		if err := pelican.ModifyState(&params); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Set Pelican state to: %+v\n", params)
		}
	})

	for {
		status, err := pelican.GetStatus()
		if err != nil {
			fmt.Printf("Failed to retrieve Pelican status: %v\n", err)
			os.Exit(1)
		}

		po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(TSTAT_PO_DF), status)
		if err != nil {
			fmt.Printf("Failed to create msgpack PO: %v", err)
			os.Exit(1)
		}
		tstatIface.PublishSignal("info", po)
		time.Sleep(pollInt)
	}
}
