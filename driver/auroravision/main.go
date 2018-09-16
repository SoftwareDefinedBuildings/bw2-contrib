package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

const PvPoDf = "2.1.1.6"

type XbosPvMeter struct {
	CurrentPower        float64 `msgpack:"current_power"`
	TotalEnergyLifetime float64 `msgpack:"total_energy_lifetime"`
	TotalEnergyToday    float64 `msgpack:"total_energy_today"`
	Time                int64   `msgpack:"time"`
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
	userID := params.MustString("user_id")
	password := params.MustString("password")
	apiKey := params.MustString("api_key")
	plantID := int32(params.MustInt("plant_id"))

	intervalStr := params.MustString("poll_interval")
	pollInterval, err := time.ParseDuration(intervalStr)
	if err != nil {
		fmt.Println("Invalid Poll Interval Length:", pollInterval)
		os.Exit(1)
	}

	svc := bwClient.RegisterService(baseURI, "s.auroravision")
	iface := svc.RegisterInterface("auroravision", "i.xbos.pv_meter")

	auroraVision := NewAuroraVision(userID, password, apiKey, plantID)
	if err != nil {
		fmt.Printf("Failed to initialize AuroraVision instance: %s\n", err)
		os.Exit(1)
	}

	summCh, errCh := auroraVision.PollSummary(context.Background(), pollInterval)
	for summary := range summCh {
		fmt.Printf("%+v\n", summary)
		msg := XbosPvMeter{
			CurrentPower:        summary.CurrentPower,
			TotalEnergyLifetime: summary.EnergyLifetime,
			TotalEnergyToday:    summary.EnergyToday,
			Time:                time.Now().UnixNano(),
		}
		po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(PvPoDf), msg)
		if err != nil {
			fmt.Printf("Failed to serialize to msgpack: %s\n", err)
		} else {
			iface.PublishSignal("info", po)
		}
	}

	select {
	case err := <-errCh:
		fmt.Printf("Failed to retrieve summary: %s\n", err)
	default:
	}
}
