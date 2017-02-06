package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/immesys/spawnpoint/spawnable"
	uuid "github.com/satori/go.uuid"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

const NAMESPACE_UUID_STR = "d8b61708-2797-11e6-836b-0cc47a0f7eea"

type TimeSeriesReading struct {
	UUID  string
	Time  int64
	Value float64
}

type ActuationCommand struct {
	On bool `msgpack:"on"`
}

func (tsr *TimeSeriesReading) ToMsgPackPO() bw2.PayloadObject {
	po, err := bw2.CreateMsgPackPayloadObject(bw2.PONumTimeseriesReading, tsr)
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
	svc := bwClient.RegisterService(baseURI, "s.TPLinkPlug")
	relayIface := svc.RegisterInterface(deviceName, "i.plug")
	relayIface.SubscribeSlot("relay", func(msg *bw2.SimpleMessage) {
		po := msg.GetOnePODF(bw2.PODFMsgPack)
		if po == nil {
			fmt.Println("Received actuation message w/o proper PO type, dropping")
		}
		var cmd ActuationCommand
		msgPackPo, ok := po.(bw2.MsgPackPayloadObject)
		if !ok {
			fmt.Println("Actuation message contained invalid msgpack PO, dropping")
		}
		if err = msgPackPo.ValueInto(&cmd); err != nil {
			fmt.Println("Could not parse actuation message PO, dropping")
		}

		err = ps.SetRelayState(cmd.On)
		if err != nil {
			fmt.Println(err)
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
			plugIface := svc.RegisterInterface(deviceName, "i.meter")
			namespaceUUID := uuid.FromStringOrNil(NAMESPACE_UUID_STR)
			currentUUID := uuid.NewV3(namespaceUUID, deviceName+"current").String()
			bwClient.SetMetadata(plugIface.SignalURI("Current"), "UnitOfMeasure", "A")
			voltageUUID := uuid.NewV3(namespaceUUID, deviceName+"voltage").String()
			bwClient.SetMetadata(plugIface.SignalURI("Voltage"), "UnitOfMeasure", "V")
			powerUUID := uuid.NewV3(namespaceUUID, deviceName+"power").String()
			bwClient.SetMetadata(plugIface.SignalURI("Power"), "UnitOfMeasure", "W")

			for {
				stats, err := ps.GetPowerStats()
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				timestamp := time.Now().UnixNano()
				currentMsg := TimeSeriesReading{UUID: currentUUID, Time: timestamp, Value: stats.Current}
				plugIface.PublishSignal("Current", currentMsg.ToMsgPackPO())
				voltageMsg := TimeSeriesReading{UUID: voltageUUID, Time: timestamp, Value: stats.Voltage}
				plugIface.PublishSignal("Voltage", voltageMsg.ToMsgPackPO())
				powerMsg := TimeSeriesReading{UUID: powerUUID, Time: timestamp, Value: stats.Power}
				plugIface.PublishSignal("Power", powerMsg.ToMsgPackPO())

				time.Sleep(pollInterval)
			}
		}()
	}

	for {
		time.Sleep(10 * time.Second)
	}
}
