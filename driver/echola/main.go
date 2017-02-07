package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/immesys/spawnpoint/spawnable"
	uuid "github.com/satori/go.uuid"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

const NAMESPACE_UUID_STR = "d8b61708-2797-11e6-836b-0cc47a0f7eea"
const numPlugs = 8

type TimeSeriesReading struct {
	UUID  string
	Time  int64
	Value float64
}

func (tsr *TimeSeriesReading) ToMsgPackPO() bw2.PayloadObject {
	po, err := bw2.CreateMsgPackPayloadObject(bw2.PONumTimeseriesReading, tsr)
	if err != nil {
		panic(err)
	}
	return po
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

	pollIntStr := params.MustString("poll_interval")
	pollInt, err := time.ParseDuration(pollIntStr)
	if err != nil {
		fmt.Printf("Invalid poll interval specified: %v\n", err)
		os.Exit(1)
	}

	echola := NewEchola(plugIP)
	deviceName := params.MustString("name")
	if deviceName == "" {
		os.Exit(1)
	}
	service := bwClient.RegisterService(baseURI+deviceName, "s.powerup.v0")
	interfaces := make([]*bw2.Interface, numPlugs)
	for i := 0; i < numPlugs; i++ {
		interfaces[i] = service.RegisterInterface(strconv.Itoa(i+1), "i.binact")
	}

	rootUUID := uuid.FromStringOrNil(NAMESPACE_UUID_STR)
	stateUUIDs := make([]string, numPlugs)
	powerUUIDs := make([]string, numPlugs)
	for i := 0; i < numPlugs; i++ {
		stateName := fmt.Sprintf("%sstate%d", deviceName, i+1)
		stateUUIDs[i] = uuid.NewV3(rootUUID, stateName).String()
		powerName := fmt.Sprintf("%spower%d", deviceName, i+1)
		powerUUIDs[i] = uuid.NewV3(rootUUID, powerName).String()
		bwClient.SetMetadata(interfaces[i].SignalURI("power"), "UnitOfMeasure", "W")
	}

	// Subscribe to actuation commands
	for i := 0; i < numPlugs; i++ {
		interfaces[i].SubscribeSlot("state", func(msg *bw2.SimpleMessage) {
			po := msg.GetOnePODF(bw2.PODFBinaryActuation)
			if po == nil {
				fmt.Println("Received actuation command without valid PO, dropping")
				return
			} else if len(po.GetContents()) < 1 {
				fmt.Println("Received actuation command with invalid PO, dropping")
				return
			}

			if po.GetContents()[0] == 0 {
				echola.ActuatePlug(i, false)
			} else if po.GetContents()[0] == 1 {
				echola.ActuatePlug(i, true)
			} else {
				fmt.Println("Actuation command contents must be 0 or 1, dropping")
			}
		})
	}

	// Publish status information
	for {
		plugStatuses, err := echola.GetStatus()
		if err != nil {
			fmt.Printf("Error getting Echola status: %v\n", err)
			os.Exit(1)
		} else if len(plugStatuses) != numPlugs {
			fmt.Println("Could not retrieve status info for all Echola plug points")
			os.Exit(1)
		}

		timestamp := time.Now().UnixNano()
		for i := 0; i < numPlugs; i++ {
			msg := TimeSeriesReading{
				UUID:  stateUUIDs[i],
				Time:  timestamp,
				Value: float64(plugStatuses[i].Enabled),
			}
			if err := interfaces[i].PublishSignal("state", msg.ToMsgPackPO()); err != nil {
				fmt.Printf("Failed to publish state info: %v\n", err)
			}

			msg = TimeSeriesReading{
				UUID:  powerUUIDs[i],
				Time:  timestamp,
				Value: plugStatuses[i].Power,
			}
			if err := interfaces[i].PublishSignal("power", msg.ToMsgPackPO()); err != nil {
				fmt.Printf("Failed to publish power info: %v\n", err)
			}
		}

		time.Sleep(pollInt)
	}
}
