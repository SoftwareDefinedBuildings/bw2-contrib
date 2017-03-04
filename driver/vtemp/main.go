package main

import (
	"github.com/immesys/spawnpoint/spawnable"
	uuid "github.com/satori/go.uuid"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

var NAMESPACE_UUID uuid.UUID

func init() {
	NAMESPACE_UUID = uuid.NewV1()
}

type TimeSeriesReading struct {
	UUID string
	Time int64
	Value float64
}

func (tsr *TimeSeriesReading) ToMsgPackPO() (bo bw2.PayloadObject) {
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
	baseuri := params.MustString("svc_base_uri")
	poll_interval := params.MustString("poll_interval")

	service := bwClient.RegisterService(baseuri, "s.vtemp")
	iface := service.RegisterInterface("Berkeley", "i.temperature")

	params.MergeMetadata(bwClient)

	temp_uuid := uuid.NewV3(NAMESPACE_UUID, "temperature").String()

	v := NewVtemp(poll_interval)
	data := v.Start()
	for point := range data {
		reading := TimeSeriesReading{UUID: temp_uuid, Time: time.Now().Unix(), Value: point.temperature}
		iface.PublishSignal("temperature", reading.ToMsgPackPO())
	}
}
