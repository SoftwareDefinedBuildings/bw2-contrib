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
const DR_PO_DF = "2.1.1.9"

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

	pelicans, err := DiscoverPelicans(username, password, sitename)
	if err != nil {
		fmt.Printf("Failed to discover thermostats: %v\n", err)
		os.Exit(1)
	}

	pollIntStr := params.MustString("poll_interval")
	pollInt, err := time.ParseDuration(pollIntStr)
	if err != nil {
		fmt.Printf("Invalid poll interval specified: %v\n", err)
		os.Exit(1)
	}

	pollDrStr := params.MustString("poll_interval_dr")
	pollDr, drErr := time.ParseDuration(pollDrStr)
	if drErr != nil {
		fmt.Printf("Invalid demand response poll interval specified: %v\n", drErr)
		os.Exit(1)
	}

	service := bwClient.RegisterService(baseURI, "s.pelican")
	tstatIfaces := make([]*bw2.Interface, len(pelicans))
	drstatIfaces := make([]*bw2.Interface, len(pelicans))
	for i, pelican := range pelicans {
		name := strings.Replace(pelican.name, " ", "_", -1)
		name = strings.Replace(name, "&", "_and_", -1)
		name = strings.Replace(name, "'", "", -1)
		fmt.Println("Transforming", pelican.name, "=>", name)
		tstatIfaces[i] = service.RegisterInterface(name, "i.xbos.thermostat")
		drstatIfaces[i] = service.RegisterInterface(name, "i.xbos.demand_response")

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

			if err := pelican.ModifySetpoints(&setpoints); err != nil {
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

			params := pelicanStateParams{
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
	}

	done := make(chan bool)
	for i, pelican := range pelicans {
		currentPelican := pelican
		currentIface := tstatIfaces[i]
		currentDRIface := drstatIfaces[i]
		go func() {
			for {
				if status, err := currentPelican.GetStatus(); err != nil {
					fmt.Printf("Failed to retrieve Pelican status: %v\n", err)
					done <- true
				} else if status != nil {
					fmt.Printf("%s %+v\n", currentPelican.name, status)

					po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(TSTAT_PO_DF), status)
					if err != nil {
						fmt.Printf("Failed to create msgpack PO: %v", err)
						done <- true
					}
					currentIface.PublishSignal("info", po)
				}
				time.Sleep(pollInt)
			}
		}()

		go func() {
			for {
				if drStatus, drErr := currentPelican.TrackDREvent(); drErr != nil {
					fmt.Printf("Failed to retrieve Pelican's DR status: %v\n", drErr)
				} else if drStatus != nil {
					fmt.Printf("%s DR Status: %+v\n", currentPelican.name, drStatus)
					po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(DR_PO_DF), drStatus)
					if err != nil {
						fmt.Printf("Failed to create DR msgpack PO: %v", err)
					}
					currentDRIface.PublishSignal("info", po)
				}
				time.Sleep(pollDr)
			}
		}()
	}
	<-done
}
