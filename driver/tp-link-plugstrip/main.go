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
	if deviceName == "" {
		os.Exit(1)
	}
	svc := bwClient.RegisterService(baseURI+deviceName, "s.powerup.v0")
	relayIface := svc.RegisterInterface("relay", "i.binact")
	relayIface.SubscribeSlot("state", func(msg *bw2.SimpleMessage) {
		po := msg.GetOnePODF(bw2.PODFBinaryActuation)
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
			namespaceUUID := uuid.FromStringOrNil(NAMESPACE_UUID_STR)
			currentUUID := uuid.NewV3(namespaceUUID, deviceName+"current").String()
			bwClient.SetMetadata(relayIface.SignalURI("Current"), "UnitOfMeasure", "A")
			voltageUUID := uuid.NewV3(namespaceUUID, deviceName+"voltage").String()
			bwClient.SetMetadata(relayIface.SignalURI("Voltage"), "UnitOfMeasure", "V")
			powerUUID := uuid.NewV3(namespaceUUID, deviceName+"power").String()
			bwClient.SetMetadata(relayIface.SignalURI("Power"), "UnitOfMeasure", "W")

			for {
				stats, err := ps.GetPowerStats()
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				timestamp := time.Now().UnixNano()
				currentMsg := TimeSeriesReading{UUID: currentUUID, Time: timestamp, Value: stats.Current}
				relayIface.PublishSignal("current", currentMsg.ToMsgPackPO())
				voltageMsg := TimeSeriesReading{UUID: voltageUUID, Time: timestamp, Value: stats.Voltage}
				relayIface.PublishSignal("voltage", voltageMsg.ToMsgPackPO())
				powerMsg := TimeSeriesReading{UUID: powerUUID, Time: timestamp, Value: stats.Power}
				relayIface.PublishSignal("power", powerMsg.ToMsgPackPO())

				time.Sleep(pollInterval)
			}
		}()
	}

	for {
		time.Sleep(10 * time.Second)
	}
}
