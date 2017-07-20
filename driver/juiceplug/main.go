package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

const JUICEPLUG_DF = "2.1.1.7"

type XBOS_EVSE struct {
	Current_limit      float64 `msgpack:"current_limit"`
	Current            float64 `msgpack:"current"`
	Voltage            float64 `msgpack:"voltage"`
	Charging_time_left int64   `msgpack:"charging_time_left"`
	State              bool    `msgpack:"state"`
	Time               int64   `msgpack:"time"`
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
	account_token := params.MustString("account_token")
	poll_interval, parseErr := time.ParseDuration(params.MustString("poll_interval"))
	if parseErr != nil {
		log.Fatal("Could not parse duration", parseErr)
	}

	acc := NewAccount(account_token)

	fmt.Println(acc)
	service := bwClient.RegisterService(baseURI, "s.juiceplug")

	// TODO: define slots

	// juiceplug native iface
	var jp_ifaces = make(map[string]*bw2.Interface)
	// i.xbos.evse iface
	var xbos_ifaces = make(map[string]*bw2.Interface)

	for _ = range time.Tick(poll_interval) {
		for _, device := range acc.read_devices() {
			var jpiface, xbosiface *bw2.Interface
			var found bool
			if jpiface, found = jp_ifaces[device.Unit_id]; !found {
				jpiface = service.RegisterInterface(device.Unit_id, "i.juiceplug")
				jp_ifaces[device.Unit_id] = jpiface
			}
			if xbosiface, found = xbos_ifaces[device.Unit_id]; !found {
				xbosiface = service.RegisterInterface(device.Unit_id, "i.xbos.evse")
				xbos_ifaces[device.Unit_id] = xbosiface
				acc.listenForActuation(xbosiface, device.Unit_id)
			}
			po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm("2.0.0.0"), device)
			if err != nil {
				log.Println("Could not publish", err)
				continue
			}
			jpiface.PublishSignal("info", po)

			signal := XBOS_EVSE{
				Current_limit: float64(device.Charging.AmpsLimit),
				Current:       float64(device.Charging.AmpsCurrent),
				Voltage:       float64(device.Charging.Voltage),
				State:         device.State == "charging",
				Time:          time.Now().UnixNano(),
			}
			po, err = bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(JUICEPLUG_DF), signal)
			if err != nil {
				log.Println("Could not publish", err)
				continue
			}
			xbosiface.PublishSignal("info", po)
		}
	}
}

func (acc *Account) listenForActuation(iface *bw2.Interface, unit_id string) {
	fmt.Println(iface.SlotURI("state"))
	iface.SubscribeSlot("state", func(msg *bw2.SimpleMessage) {
		msg.Dump()
		po := msg.GetOnePODF(JUICEPLUG_DF)
		if po == nil {
			fmt.Println("Received message on state slot without required PO. Dropping.")
			return
		}
		var params write_params
		if err := po.(bw2.MsgPackPayloadObject).ValueInto(&params); err != nil {
			fmt.Println("Received malformed PO on state slot. Dropping.", err)
			return
		}
		if err := acc.write_device(unit_id, params); err != nil {
			fmt.Println("Could not actuate plug", err)
			return
		}
	})
}
