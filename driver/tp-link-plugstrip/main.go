package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

const (
	PONUM = "2.1.1.2"
)

type XBOSPlugReading struct {
	Time       int64
	Voltage    float64
	Power      float64
	Current    float64
	Cumulative float64
	State      bool
}

func (tpl *XBOSPlugReading) ToMsgPackPO() bw2.PayloadObject {
	po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(PONUM), tpl)
	if err != nil {
		panic(err)
	} else {
		return po
	}
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
	plugIP := params.MustString("plug_ip")

	ps, err := NewPlugstrip(plugIP)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("Initialized driver for %s plugstrip\n", ps.model)

	deviceName := params.MustString("name")
	if deviceName == "" {
		os.Exit(1)
	}
	svc := bwClient.RegisterService(baseURI+deviceName, "s.tplink.v0")
	// relayIface := svc.RegisterInterface("0", "i.xbos.plug")
	// relayIface.SubscribeSlot("state", func(msg *bw2.SimpleMessage) {
	// 	po := msg.GetOnePODF(bw2.PODFBinaryActuation)
	// 	if po == nil {
	// 		fmt.Println("Received actuation command w/o proper PO type, dropping")
	// 		return
	// 	} else if len(po.GetContents()) < 1 {
	// 		fmt.Println("Received actuation command with invalid PO, dropping")
	// 		return
	// 	}
	//
	// 	if po.GetContents()[0] == 0 {
	// 		err = ps.SetRelayState(false)
	// 		if err != nil {
	// 			fmt.Println(err)
	// 		}
	// 	} else if po.GetContents()[0] == 1 {
	// 		err = ps.SetRelayState(true)
	// 		if err != nil {
	// 			fmt.Println(err)
	// 		}
	// 	} else {
	// 		fmt.Println("Actuation command contents must be 0x00 or 0x01, dropping")
	// 	}
	// })

	xbosIface := svc.RegisterInterface("0", "i.xbos.plug")
	xbosIface.SubscribeSlot("state", func(msg *bw2.SimpleMessage) {
		po := msg.GetOnePODF(PONUM)
		if po != nil {

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
			state := data["state"].(bool)
			ps.SetRelayState(state)
			return
		}

		//Although this is not part of the i.xbos.plug spec, it is quite handy
		//to be able to support simple binary actuation. We do this only if there
		//is no XBOS Plug PO in the message
		po = msg.GetOnePODF(bw2.PODFBinaryActuation)
		if po == nil {
			fmt.Println("Received actuation command w/o proper PO type, dropping")
			return
		} else if len(po.GetContents()) < 1 {
			fmt.Println("Received actuation command with invalid PO, dropping")
			return
		}

		if po.GetContents()[0] == 0 {
			err = ps.SetRelayState(false)
			if err != nil {
				fmt.Println(err)
			}
		} else if po.GetContents()[0] == 1 {
			err = ps.SetRelayState(true)
			if err != nil {
				fmt.Println(err)
			}
		} else {
			fmt.Println("Actuation command contents must be 0x00 or 0x01, dropping")
		}
	})

	if ps.HasPowerStats() {
		go func() {
			intervalStr := params.MustString("poll_interval")
			pollInterval, err := time.ParseDuration(intervalStr)
			if err != nil {
				fmt.Println("Invalid Poll Interval Length:", pollInterval)
				os.Exit(1)
			}

			for {
				stats, err := ps.GetPowerStats()
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				timestamp := time.Now().UnixNano()
				reading := &XBOSPlugReading{
					Time:       timestamp,
					Voltage:    stats.Voltage,
					Current:    stats.Current,
					Cumulative: stats.Total,
					Power:      stats.Power,
					//The plug reports its internal power draw (~2W) only when the plug is on, so
					//we can use this to infer relay state without having to query separately for it
					State: stats.Power != 0,
				}
				po := reading.ToMsgPackPO()
				xbosIface.PublishSignal("info", po)

				time.Sleep(pollInterval)
			}
		}()
	}

	for {
		time.Sleep(10 * time.Second)
	}
}
