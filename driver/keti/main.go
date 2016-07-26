package main

import (
	"fmt"
	"github.com/immesys/spawnpoint/spawnable"
	"github.com/satori/go.uuid"
	bw2 "gopkg.in/immesys/bw2bind.v5"
	"time"
)

var NAMESPACE_UUID uuid.UUID

//func init() {
//NAMESPACE_UUID = uuid.FromStringOrNil("d8b61708-2797-11e6-836b-0cc47a0f7eea")
//}

type TimeseriesReading struct {
	UUID  string
	Time  int64
	Value float64
}

func (msg TimeseriesReading) ToMsgPackBW() (po bw2.PayloadObject) {
	po, _ = bw2.CreateMsgPackPayloadObject(bw2.FromDotForm("2.0.9.1"), msg)
	return
}

func makeUUID(nodeid uint16, stream string) string {
	return uuid.NewV3(NAMESPACE_UUID, fmt.Sprintf("%s", nodeid)+stream).String()
}

var motes = make(map[uint16]*bw2.Interface)

func publish(svc *bw2.Service, nodeid uint16, stream string, msg TimeseriesReading) {
	iface, found := motes[nodeid]
	if !found {
		iface = svc.RegisterInterface(fmt.Sprintf("%s", nodeid), stream)
	}
	iface.PublishSignal(stream, msg.ToMsgPackBW())
}

func main() {
	bw := bw2.ConnectOrExit("")

	params := spawnable.GetParamsOrExit()
	bw.OverrideAutoChainTo(true)
	bw.SetEntityFromEnvironOrExit()

	// params
	serialPort := params.MustString("SerialPort")
	baudRate := params.MustInt("BaudRate")
	NAMESPACE_UUID = uuid.FromStringOrNil(params.MustString("Namespace"))
	baseuri := params.MustString("svc_base_uri")

	params.MergeMetadata(bw)

	svc := bw.RegisterService(baseuri, "s.KETIMote")

	ketiReceiver := NewKetiMoteReceiver(serialPort, baudRate)

	go func() {
		for tempRdg := range ketiReceiver.TempReadings {
			// construct uuid
			msg := TimeseriesReading{
				UUID:  makeUUID(tempRdg.NodeID, "Temperature"),
				Time:  time.Now().Unix(),
				Value: tempRdg.Temperature,
			}
			publish(svc, tempRdg.NodeID, "Temperature", msg)

			msg2 := TimeseriesReading{
				UUID:  makeUUID(tempRdg.NodeID, "Humidity"),
				Time:  time.Now().Unix(),
				Value: tempRdg.Humidity,
			}
			publish(svc, tempRdg.NodeID, "Humidity", msg2)

			msg3 := TimeseriesReading{
				UUID:  makeUUID(tempRdg.NodeID, "Lux"),
				Time:  time.Now().Unix(),
				Value: tempRdg.Lux,
			}
			publish(svc, tempRdg.NodeID, "Lux", msg3)
		}
	}()
	go func() {
		for pirRdg := range ketiReceiver.PIRReadings {
			msg := TimeseriesReading{
				UUID:  makeUUID(pirRdg.NodeID, "PIR"),
				Time:  time.Now().Unix(),
				Value: pirRdg.PIR,
			}
			publish(svc, pirRdg.NodeID, "PIR", msg)
		}
	}()
	go func() {
		for co2Rdg := range ketiReceiver.CO2Readings {
			msg := TimeseriesReading{
				UUID:  makeUUID(co2Rdg.NodeID, "CO2"),
				Time:  time.Now().Unix(),
				Value: co2Rdg.CO2,
			}
			publish(svc, co2Rdg.NodeID, "CO2", msg)
		}
	}()

	waitForever := make(chan bool)
	<-waitForever
}
