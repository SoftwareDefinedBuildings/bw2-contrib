package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/SoftwareDefinedBuildings/bw2-contrib/driver/pelican/storage"
	"github.com/SoftwareDefinedBuildings/bw2-contrib/driver/pelican/types"
	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

const TSTAT_PO_DF = "2.1.1.0"
const DR_PO_DF = "2.1.1.9"
const OCCUPANCY_PO_DF = "2.1.2.1"
const SCHED_PO_DF = "2.1.2.2"

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
	HeatingStages *int32 `msgpack:"enabled_heat_stages"`
	CoolingStages *int32 `msgpack:"enabled_cool_stages"`
}

type occupancyMsg struct {
	Occupancy bool  `msgpack:"occupancy"`
	Time      int64 `msgpack:"time"`
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

	pollDrStr := params.MustString("poll_interval_dr")
	pollDr, drErr := time.ParseDuration(pollDrStr)
	if drErr != nil {
		fmt.Printf("Invalid demand response poll interval specified: %v\n", drErr)
		os.Exit(1)
	}

	pollSchedStr := params.MustString("poll_interval_sched")
	pollSched, schedErr := time.ParseDuration(pollSchedStr)
	if schedErr != nil {
		fmt.Printf("Invalid schedule poll interval specified: %v\n", schedErr)
		os.Exit(1)
	}

	service := bwClient.RegisterService(baseURI, "s.pelican")
	tstatIfaces := make([]*bw2.Interface, len(pelicans))
	drstatIfaces := make([]*bw2.Interface, len(pelicans))
	schedstatIfaces := make([]*bw2.Interface, len(pelicans))
	occupancyIfaces := make([]*bw2.Interface, len(pelicans))
	for i, pelican := range pelicans {
		pelican := pelican
		name := strings.Replace(pelican.Name, " ", "_", -1)
		name = strings.Replace(name, "&", "_and_", -1)
		name = strings.Replace(name, "'", "", -1)
		fmt.Println("Transforming", pelican.Name, "=>", name)
		tstatIfaces[i] = service.RegisterInterface(name, "i.xbos.thermostat")
		drstatIfaces[i] = service.RegisterInterface(name, "i.xbos.demand_response")
		schedstatIfaces[i] = service.RegisterInterface(name, "i.xbos.thermostat_schedule")
		occupancyIfaces[i] = service.RegisterInterface(name, "i.xbos.occupancy")

		// Ensure thermostat is running with correct number of stages
		if err := pelican.ModifyStages(&types.PelicanStageParams{
			HeatingStages: &pelican.HeatingStages,
			CoolingStages: &pelican.CoolingStages,
		}); err != nil {
			fmt.Printf("Failed to configure heating/cooling stages for pelican %s: %s\n",
				pelican.Name, err)
			//os.Exit(1)
		}

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
				fmt.Println("Received message on stage slot without required PO. Dropping.")
				return
			}

			var stages stageMsg
			if err := po.(bw2.MsgPackPayloadObject).ValueInto(&stages); err != nil {
				fmt.Println("Received malformed PO on stage slot. Dropping.", err)
				return
			}
			if stages.HeatingStages == nil && stages.CoolingStages == nil {
				fmt.Println("Received message on stage slot with no content. Dropping.")
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

		schedstatIfaces[i].SubscribeSlot("schedule", func(msg *bw2.SimpleMessage) {
			po := msg.GetOnePODF(SCHED_PO_DF)
			if po == nil {
				fmt.Println("Received message on stage slot without required PO. Dropping.")
				return
			}

			var schedule types.ThermostatSchedule
			if err := po.(bw2.MsgPackPayloadObject).ValueInto(&schedule); err != nil {
				fmt.Println("Received malformed PO on stage slot. Dropping.", err)
				return
			}
			if schedule.DaySchedules == nil {
				fmt.Println("Received message on stage slot with no content. Dropping.")
				return
			}

			if err := pelican.SetSchedule(&schedule); err != nil {
				fmt.Println(err)
			} else {
				fmt.Printf("Set pelican schedule to: %v", schedule)
			}
		})
	}

	done := make(chan bool)
	for i, pelican := range pelicans {
		currentPelican := pelican
		currentIface := tstatIfaces[i]
		currentDRIface := drstatIfaces[i]
		currentSchedIface := schedstatIfaces[i]
		currentOccupancyIface := occupancyIfaces[i]

		go func() {
			for {
				if status, err := currentPelican.GetStatus(); err != nil {
					fmt.Printf("Failed to retrieve Pelican status: %v\n", err)
					done <- true
				} else if status != nil {
					fmt.Printf("%s %+v\n", currentPelican.Name, status)

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
					fmt.Printf("%s DR Status: %+v\n", currentPelican.Name, drStatus)
					po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(DR_PO_DF), drStatus)
					if err != nil {
						fmt.Printf("Failed to create DR msgpack PO: %v", err)
					}
					currentDRIface.PublishSignal("info", po)
				}
				time.Sleep(pollDr)
			}
		}()

		go func() {
			for {
				if schedStatus, schedErr := currentPelican.GetSchedule(); schedErr != nil {
					fmt.Printf("Failed to retrieve Pelican's Schedule: %v\n", schedErr)
				} else {
					fmt.Printf("%s Schedule: %+v\n", currentPelican.Name, schedStatus)
					po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(SCHED_PO_DF), schedStatus)
					if err != nil {
						fmt.Printf("Failed to create Schedule msgpack PO: %v", err)
					}
					currentSchedIface.PublishSignal("info", po)
				}
				time.Sleep(pollSched)
			}
		}()

		occupancy, err := currentPelican.GetOccupancy()
		if err != nil {
			fmt.Printf("Failed to retrieve initial occupancy reading: %s\n", err)
			return
		}
		// Start occupancy tracking loop only if thermostat has the necessary sensor
		if occupancy != types.OCCUPANCY_UNKNOWN {
			go func() {
				for {
					occupancy, err := currentPelican.GetOccupancy()
					if err != nil {
						fmt.Printf("Failed to read thermostat occupancy: %s\n", err)
					} else {
						occupancyStatus := occupancyMsg{
							Occupancy: (occupancy == types.OCCUPANCY_OCCUPIED),
							Time:      time.Now().UnixNano(),
						}
						po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(OCCUPANCY_PO_DF), occupancyStatus)
						if err != nil {
							fmt.Printf("Failed to create occupancy msgpack PO: %s\n", err)
						} else {
							currentOccupancyIface.PublishSignal("info", po)
						}
					}
					time.Sleep(pollInt)
				}
			}()
		}
	}
	<-done
}
