package main

import (
	"github.com/pkg/errors"
)

func (srv *EagleServer) forwardDemandData(demand *InstantaneousDemand) {
	demand.Dump()

	// get the eagle object, which we know to exist
	var eagle *Eagle
	srv.eagleLock.RLock()
	eagle = srv.eagles[demand.DeviceMacId]
	srv.eagleLock.RUnlock()

	// send bosswave messages
	msg := meterMessage{
		Current_demand: demand.ActualDemand,
		Time:           demand.ActualTimestamp * 1e9,
	}
	log.Debug(eagle.iface)
	err := eagle.iface.PublishSignal("meter", msg.ToMsgPackBW())
	if err != nil {
		log.Error(errors.Wrap(err, "Could not publish i.meter"))
	}
	log.Debug(eagle.iface.SignalURI("meter"))
}
