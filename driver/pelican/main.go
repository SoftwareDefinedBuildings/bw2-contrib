package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/SoftwareDefinedBuildings/bw2-contrib/driver/pelican/storage"
	"github.com/SoftwareDefinedBuildings/bw2-contrib/driver/pelican/types"
	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

const TSTAT_PO_DF = "2.1.1.0"

type setpointsMsg struct {
	HeatingSetpoint *float64 `msgpack:"heating_setpoint"`
	CoolingSetpoint *float64 `msgpack:"cooling_setpoint"`
}

type stateMsg struct {
	HeatingSetpoint *float64 `msgpack:"heating_setpoint"`
	CoolingSetpoint *float64 `msgpack:"cooling_setpoint"`
	Override        *bool    `msgpack:"override"`
	Mode            *int     `msgpack:"mode"`
	Fan             *bool    `msgpack:"fan"`
}

type stageMsg struct {
	HeatingStages *int32 `msgpack:"enabled_heating_stages"`
	CoolingStages *int32 `msgpack:"enabled_cooling_stages"`
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

	pelicans, err := storage.ReadPelicans(username, password, sitename)
	if err != nil {
		fmt.Printf("Failed to read thermostat info: %v\n", err)
		os.Exit(1)
	}

	pollIntStr := params.MustString("poll_interval")
	pollInt, err := time.ParseDuration(pollIntStr)
	if err != nil {
		fmt.Printf("Invalid poll interval specified: %v\n", err)
		os.Exit(1)
	}

	service := bwClient.RegisterService(baseURI, "s.pelican")
	tstatIfaces := make([]*bw2.Interface, len(pelicans))
	for i, pelican := range pelicans {
		// Ensure thermostat is running with correct number of stages
		if err := pelican.ModifyStages(&types.PelicanStageParams{
			HeatingStages: &pelican.HeatingStages,
			CoolingStages: &pelican.CoolingStages,
		}); err != nil {
			fmt.Printf("Failed to configure heating/cooling stages for pelican %s: %s\n",
				pelican.Name, err)
			os.Exit(1)
		}

		tstatIfaces[i] = service.RegisterInterface(pelican.Name, "i.xbos.thermostat")

		tstatIfaces[i].SubscribeSlot("setpoints", func(msg *bw2.SimpleMessage) {
			po := msg.GetOnePODF(TSTAT_PO_DF)
			if po == nil {
				fmt.Println("Received message on setpoints slot without required PO. Droping.")
				return
			}

			var setpoints setpointsMsg
			if err := po.(bw2.MsgPackPayloadObject).ValueInto(&setpoints); err != nil {
				fmt.Println("Received malformed PO on setpoints slot. Dropping.", err)
				return
			}

			params := types.PelicanSetpointParams{
				HeatingSetpoint: setpoints.HeatingSetpoint,
				CoolingSetpoint: setpoints.CoolingSetpoint,
			}
			if err := pelican.ModifySetpoints(&params); err != nil {
				fmt.Println(err)
			} else {
				fmt.Printf("Set heating setpoint to %v and cooling setpoint to %v\n",
					setpoints.HeatingSetpoint, setpoints.CoolingSetpoint)
			}
		})

		tstatIfaces[i].SubscribeSlot("state", func(msg *bw2.SimpleMessage) {
			po := msg.GetOnePODF(TSTAT_PO_DF)
			if po == nil {
				fmt.Println("Received message on state slot without required PO. Dropping.")
				return
			}

			var state stateMsg
			if err := po.(bw2.MsgPackPayloadObject).ValueInto(&state); err != nil {
				fmt.Println("Received malformed PO on state slot. Dropping.", err)
				return
			}

			params := types.PelicanStateParams{
				HeatingSetpoint: state.HeatingSetpoint,
				CoolingSetpoint: state.CoolingSetpoint,
			}
			fmt.Printf("%+v", state)
			if state.Mode != nil {
				m := float64(*state.Mode)
				params.Mode = &m
			}

			if state.Override != nil && *state.Override {
				f := float64(1)
				params.Override = &f
			} else {
				f := float64(0)
				params.Override = &f
			}

			if state.Fan != nil && *state.Fan {
				f := float64(1)
				params.Fan = &f
			} else {
				f := float64(0)
				params.Fan = &f
			}

			if err := pelican.ModifyState(&params); err != nil {
				fmt.Println(err)
			} else {
				fmt.Printf("Set Pelican state to: %+v\n", params)
			}
		})

		tstatIfaces[i].SubscribeSlot("stages", func(msg *bw2.SimpleMessage) {
			po := msg.GetOnePODF(TSTAT_PO_DF)
			if po == nil {
				fmt.Println("Received message on state slot without required PO. Dropping.")
				return
			}

			var stages stageMsg
			if err := po.(bw2.MsgPackPayloadObject).ValueInto(&stages); err != nil {
				fmt.Println("Received malformed PO on stage slot. Dropping.", err)
				return
			}

			params := types.PelicanStageParams{
				HeatingStages: stages.HeatingStages,
				CoolingStages: stages.CoolingStages,
			}
			if err := pelican.ModifyStages(&params); err != nil {
				fmt.Println(err)
			} else {
				if stages.HeatingStages != nil {
					fmt.Printf("Set pelican heating stages to: %d\n", *stages.HeatingStages)
				}
				if stages.CoolingStages != nil {
					fmt.Printf("Set pelican cooling stages to: %d\n", *stages.CoolingStages)
				}
			}
		})
	}

	wg := sync.WaitGroup{}
	for i, pelican := range pelicans {
		wg.Add(1)
		currentPelican := pelican
		currentIface := tstatIfaces[i]
		go func() {
			defer wg.Done()
			for {
				status, err := currentPelican.GetStatus()
				if err != nil {
					fmt.Printf("Failed to retrieve Pelican status: %v\n", err)
					return
				}

				po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(TSTAT_PO_DF), status)
				if err != nil {
					fmt.Printf("Failed to create msgpack PO: %v", err)
					return
				}
				currentIface.PublishSignal("info", po)
				time.Sleep(pollInt)
			}
		}()
	}

	wg.Wait()
}
