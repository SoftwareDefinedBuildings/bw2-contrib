package main

import (
	"fmt"
	"github.com/immesys/spawnpoint/spawnable"
	"github.com/satori/go.uuid"
	bw2 "gopkg.in/immesys/bw2bind.v5"
	"strings"
	"time"
)

var NAMESPACE_UUID uuid.UUID
var bufsend *BufferedSender

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

func getChannel(stream string) string {
	var channel string
	switch stream {
	case "Temperature":
		channel = "temperature"
	case "Humidity":
		channel = "humidity"
	case "Lux":
		channel = "light"
	case "PIR":
		channel = "pir"
	case "CO2":
		channel = "co2"
	}
	return channel
}

func makeUUID(serial_id [6]byte, stream string) string {
	channel := getChannel(stream)
	return uuid.NewV5(NAMESPACE_UUID, fmt.Sprintf("%s-%s", serial_id, channel)).String()
}

var motes = make(map[uint16]*bw2.Interface)

func publish(svc *bw2.Service, nodeid uint16, stream string, msg TimeseriesReading) {
	iface, found := motes[nodeid]
	nodestring := fmt.Sprintf("%d", nodeid)
	if !found {
		iface = svc.RegisterInterfaceHeartbeatOnPub(nodestring, stream)
	}
	fmt.Printf("URI: %s\n", iface.SignalURI(stream))
	iface.PublishSignal(stream, msg.ToMsgPackBW())
}

func publishSmap(nodeid uint16, uri, stream, serialPort string, msg TimeseriesReading) {
	path := strings.TrimPrefix(serialPort, "/dev") + fmt.Sprintf("/%d/", nodeid) + getChannel(stream)
	if err := bufsend.Send(path, msg); err != nil {
		fmt.Println(err)
	}
	//if err := msg.SendToSmap(path, uri); err != nil {
	//	fmt.Println(err)
	//}
}

func main() {
	bw := bw2.ConnectOrExit("")

	params := spawnable.GetParamsOrExit()
	bw.OverrideAutoChainTo(true)
	bw.SetEntityFromEnvironOrExit()

	// params
	baudRate := params.MustInt("BaudRate")
	NAMESPACE_UUID = uuid.FromStringOrNil(params.MustString("Namespace"))
	baseuri := params.MustString("svc_base_uri")
	smapURI := params.MustString("smapURI")

	params.MergeMetadata(bw)

	svc := bw.RegisterService(baseuri, "s.KETIMote")
	bufsend = NewBufferedSender(smapURI, 100)

	serialPorts := params.MustStringSlice("SerialPorts")
	for _, serialPort := range serialPorts {
		ketiReceiver := NewKetiMoteReceiver(serialPort, baudRate)
		go func() {
			for tempRdg := range ketiReceiver.TempReadings {
				// construct uuid
				// for the publish calls, we keep them all Temperature so they show up
				// under the same interface
				fmt.Printf("Reading: %+v\n", tempRdg)
				msg := TimeseriesReading{
					UUID:  makeUUID(tempRdg.SerialID, "Temperature"),
					Time:  time.Now().Unix(),
					Value: tempRdg.Temperature,
				}
				publish(svc, tempRdg.NodeID, "i.keti-temperature", msg)
				publishSmap(tempRdg.NodeID, smapURI, "Temperature", serialPort, msg)

				msg2 := TimeseriesReading{
					UUID:  makeUUID(tempRdg.SerialID, "Humidity"),
					Time:  time.Now().Unix(),
					Value: tempRdg.Humidity,
				}
				publish(svc, tempRdg.NodeID, "i.keti-temperature", msg2)
				publishSmap(tempRdg.NodeID, smapURI, "Humidity", serialPort, msg2)

				msg3 := TimeseriesReading{
					UUID:  makeUUID(tempRdg.SerialID, "Lux"),
					Time:  time.Now().Unix(),
					Value: tempRdg.Lux,
				}
				publish(svc, tempRdg.NodeID, "i.keti-temperature", msg3)
				publishSmap(tempRdg.NodeID, smapURI, "Lux", serialPort, msg3)
			}
		}()
		go func() {
			for pirRdg := range ketiReceiver.PIRReadings {
				fmt.Printf("Reading: %+v\n", pirRdg)
				msg := TimeseriesReading{
					UUID:  makeUUID(pirRdg.SerialID, "PIR"),
					Time:  time.Now().Unix(),
					Value: pirRdg.PIR,
				}
				publish(svc, pirRdg.NodeID, "i.keti-pir", msg)
				publishSmap(pirRdg.NodeID, smapURI, "PIR", serialPort, msg)
			}
		}()
		go func() {
			for co2Rdg := range ketiReceiver.CO2Readings {
				fmt.Printf("Reading: %+v\n", co2Rdg)
				msg := TimeseriesReading{
					UUID:  makeUUID(co2Rdg.SerialID, "CO2"),
					Time:  time.Now().Unix(),
					Value: co2Rdg.CO2,
				}
				publish(svc, co2Rdg.NodeID, "i.keti-co2", msg)
				publishSmap(co2Rdg.NodeID, smapURI, "CO2", serialPort, msg)
			}
		}()

	}

	waitForever := make(chan bool)
	<-waitForever
}
