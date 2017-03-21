package main

import (
	"fmt"
	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
	"os"
	"time"
)

const (
	PONUM = "2.1.1.0"
)

type CSVPoint struct {
	time int64
	temperature float64
}

func NewInfoPO(time int64, temp float64, relHumidity float64, heatingSetpoint float64, coolingSetpoint float64, override bool, fan bool, mode int, state int) (bw2.PayloadObject) {
	msg := map[string]interface{}{
		"temperature": temp, 
		"relative_humidity": relHumidity, 
		"heating_setpoint": heatingSetpoint, 
		"cooling_setpoint": coolingSetpoint, 
		"override": override, 
		"fan": fan, 
		"mode": mode, 
		"state": state, 
		"time": time}
	po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(PONUM), msg)
	if err != nil {
		panic(err)
	}
	return po
}

func main() {
	bwClient := bw2.ConnectOrExit("")

	params := spawnable.GetParamsOrExit()
	bwClient.OverrideAutoChainTo(true)
	bwClient.SetEntityFromEnvironOrExit()

	baseuri := params.MustString("svc_base_uri")
	poll_interval := params.MustString("poll_interval")

	service := bwClient.RegisterService(baseuri, "s.vthermostat")
	iface := service.RegisterInterface("vthermostat", "i.xbos.thermostat")

	params.MergeMetadata(bwClient)

	v := NewVthermostat(poll_interval)

	csvChan := make(chan CSVPoint, 10)
	csvFile, err := os.Create("vthermostat.csv")
	if err == nil {
		go func() {
			fmt.Println("receiving")
			for point := range csvChan {
				line := fmt.Sprint(point.time, ",", point.temperature, ",")
				csvFile.Write([]byte(line))
			}
		}()
	} else {
		fmt.Println(err)
	}

	iface.SubscribeSlot("setpoints", func(msg *bw2.SimpleMessage) {
		po := msg.GetOnePODF(PONUM)
		if po == nil {
			fmt.Println("Received actuation command without valid PO, dropping")
			return
		}

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

		v.setHeatingSetpoint(data["heating_setpoint"].(float64))
		v.setCoolingSetpoint(data["cooling_setpoint"].(float64))
	})

	iface.SubscribeSlot("state", func(msg *bw2.SimpleMessage) {
		po := msg.GetOnePODF(PONUM)
		if po == nil {
			fmt.Println("Received actuation command without valid PO, dropping")
			return
		}

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

		v.setHeatingSetpoint(data["heating_setpoint"].(float64))
		v.setCoolingSetpoint(data["cooling_setpoint"].(float64))
		v.setOverride(data["override"].(bool))
		v.setMode(int(data["mode"].(uint64)))
		v.setFan(data["fan"].(bool))
	})

	data := v.Start()
	for point := range data {
		timestamp := time.Now().UnixNano()
		po := NewInfoPO(
			timestamp,
			point.temperature,
			point.relativeHumidity,
			point.heatingSetpoint,
			point.coolingSetpoint,
			point.override,
			point.fan,
			point.mode,
			point.state)
		iface.PublishSignal("info", po)
		
		csvData := CSVPoint {
			time: timestamp,
			temperature: point.temperature,
		}

		csvChan <- csvData
	}
}
