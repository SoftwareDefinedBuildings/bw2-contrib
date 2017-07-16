package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

type XBOS_PV_Meter struct {
	Current_power         float64 `msgpack:"current_power"`
	Total_energy_lifetime float64 `msgpack:"total_energy_lifetime"`
	Total_energy_today    float64 `msgpack:"total_energy_today"`
	Time                  int64   `msgpack:"time"`
}

func main() {
	// As per the Enphase attribution requirement
	fmt.Println("Powered by Enphase Energy (http://enphase.com)")

	bwClient := bw2.ConnectOrExit("")
	bwClient.OverrideAutoChainTo(true)
	bwClient.SetEntityFromEnvironOrExit()

	params := spawnable.GetParamsOrExit()
	baseURI := params.MustString("svc_base_uri")
	if !strings.HasSuffix(baseURI, "/") {
		baseURI += "/"
	}
	userID := params.MustString("user_id")
	apiKey := params.MustString("api_key")
	sysName := params.MustString("system_name")

	intervalStr := params.MustString("poll_interval")
	pollInterval, err := time.ParseDuration(intervalStr)
	if err != nil {
		fmt.Println("Invalid Poll Interval Length:", pollInterval)
		os.Exit(1)
	}

	svc := bwClient.RegisterService(baseURI, "s.enphase")
	iface := svc.RegisterInterface("enphase1", "i.xbos.pv_meter")

	enphase, err := NewEnphase(apiKey, userID, sysName)
	if err != nil {
		fmt.Println("Failed to initialize Enphase instance:", err.Error())
		os.Exit(1)
	}
	summCh := enphase.PollSummary(pollInterval)
	for summary := range summCh {
		fmt.Println(summary)
		msg := XBOS_PV_Meter{
			Current_power:         float64(summary.CurrentPower),
			Total_energy_lifetime: float64(summary.EnergyLifetime),
			Total_energy_today:    float64(summary.EnergyToday),
			Time:                  time.Now().UnixNano(),
		}

		po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(PV_PO_DF), msg)
		if err != nil {
			log.Println(err)
			continue
		}

		iface.PublishSignal("info", po)
	}
}
