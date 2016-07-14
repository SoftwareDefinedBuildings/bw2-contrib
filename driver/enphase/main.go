package main

import (
	"fmt"
	"os"
	"time"

	"github.com/immesys/spawnpoint/spawnable"
	"github.com/satori/go.uuid"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

// Not sure why this isn't in bw2_pid
const PODFTimeSeriesReading = `2.0.9.1`

type TimeseriesReading struct {
	UUID  string
	Time  int64
	Value uint64
}

func (tsr *TimeseriesReading) ToMsgPack() bw2.PayloadObject {
	po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(PODFTimeSeriesReading), tsr)
	if err != nil {
		panic(err)
	} else {
		return po
	}
}

func main() {
	// As per the Enphase attribution requirement
	fmt.Println("Powered by Enphase Energy (http://enphase.com)")

	bwClient := bw2.ConnectOrExit("")
	bwClient.OverrideAutoChainTo(true)
	bwClient.SetEntityFromEnvironOrExit()

	params := spawnable.GetParamsOrExit()
	name := params.MustString("name")
	baseURI := params.MustString("svc_base_uri")
	userID := params.MustString("user_id")
	apiKey := params.MustString("api_key")
	sysName := params.MustString("system_name")

	intervalStr := params.MustString("poll_interval")
	pollInterval, err := time.ParseDuration(intervalStr)
	if err != nil {
		fmt.Println("Invalid Poll Interval Length:", pollInterval)
		os.Exit(1)
	}

	svc := bwClient.RegisterService(baseURI, "s.Enphase")
	iface := svc.RegisterInterface(name, "i.meter")
	bwClient.SetMetadata(iface.SignalURI("CurrentPower"), "UnitofMeasure", "W")
	// TODO More intelligent way to derive UUIDs, e.g. from V3?
	currentPowerUUID := uuid.NewV4()
	bwClient.SetMetadata(iface.SignalURI("EnergyLifetime"), "UnitofMeasure", "Wh")
	energyLifetimeUUID := uuid.NewV4()
	bwClient.SetMetadata(iface.SignalURI("EnergyToday"), "UnitofMeasure", "Wh")
	energyTodayUUID := uuid.NewV4()

	enphase, err := NewEnphase(apiKey, userID, sysName)
	if err != nil {
		fmt.Println("Failed to initialize Enphase instance:", err.Error())
		os.Exit(1)
	}
	summCh := enphase.PollSummary(pollInterval)
	for summary := range summCh {
		fmt.Printf("Summary: %+v\n", summary)

		currentPowerReading := TimeseriesReading{
			UUID:  currentPowerUUID.String(),
			Time:  time.Now().UnixNano(),
			Value: summary.CurrentPower,
		}
		iface.PublishSignal("CurrentPower", currentPowerReading.ToMsgPack())

		energyLifetimeReading := TimeseriesReading{
			UUID:  energyLifetimeUUID.String(),
			Time:  time.Now().UnixNano(),
			Value: summary.EnergyLifetime,
		}
		iface.PublishSignal("EnergyLifetime", energyLifetimeReading.ToMsgPack())

		energyTodayReading := TimeseriesReading{
			UUID:  energyTodayUUID.String(),
			Time:  time.Now().UnixNano(),
			Value: summary.EnergyToday,
		}
		iface.PublishSignal("EnergyToday", energyTodayReading.ToMsgPack())
	}
}
