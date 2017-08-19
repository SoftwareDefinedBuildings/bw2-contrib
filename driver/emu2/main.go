// There are two approaches for the Eagle driver. We either implement a centralized service that the Eagle is
// configured to talk to, or we implement a local driver that then polls the Eagle's local REST interface.
// This file implements the former.
// We implement a server with a single URL served over HTTPS
//	- GET: returns a page of directions on how to set this up on your own Eagle
//	- POST: the Eagle will POST to this URL. The server will check if we've already seen the Eagle.
//			If it hasn't, we register the Eagle locally (instantiate another struct) and configure it for our
//		  	desired reporting interval, and extract all the necessary metadata
//			Finally, for all received messages, we process the latest reading and mirror it onto BOSSWAVE
// Configuration options:
//	- reporting interval (TODO: figure out what the bounds are)
//	- how much historical data to grab (TODO: figure out what the bounds are), which is then replayed on BOSSWAVE
//		with the appropriate timestamps
package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/immesys/spawnpoint/spawnable"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"github.com/tarm/serial"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

// logger
var log *logging.Logger

// set up logging facilities
func init() {
	log = logging.MustGetLogger("eagle")
	var format = "%{color}%{level} %{time:Jan 02 15:04:05} %{shortfile}%{color:reset} ▶ %{message}"
	var logBackend = logging.NewLogBackend(os.Stderr, "", 0)
	logBackendLeveled := logging.AddModuleLevel(logBackend)
	logging.SetBackend(logBackendLeveled)
	logging.SetFormatter(logging.MustStringFormatter(format))
}

type Eagle struct {
	multiplier  float64
	serial_port string
	baseuri     string

	// MAC ID of Eagle
	MacID string
	// MAC address of Eagle
	DeviceMAC string
	// MAC address of meter
	MeterMAC string
	// multiplier for demand readings
	Multiplier int64
	// type of meter (Electric/Gas/Water/Other)
	Type string
	// bosswave publishing interface
	iface     *bw2.Interface
	xbosiface *bw2.Interface
	svc       *bw2.Service
	// current status of Eagle
	current_demand              float64
	current_price               float64
	current_summation_delivered float64
	current_summation_received  float64
	current_tier                int64
	current_time                int64
	NetworkInfo

	// bosswave
	bwclient *bw2.BW2Client
	vk       string
}

func (eagle *Eagle) HandleMessage(resp Response) {
	// if we haven't seen this eagle before, ignore the message.
	// We only want to register off of the NetworkInfo messages

	// handle registration of a new eagle
	if resp.NetworkInfo != nil {
		log.Info("NETWORK INFO")
		info := resp.NetworkInfo

		// create new eeeaaagleeeee
		eagle.DeviceMAC = info.DeviceMacId
		eagle.InstallCode = info.InstallCode
		eagle.LinkKey = info.LinkKey
		eagle.FWVersion = info.FWVersion
		eagle.HWVersion = info.HWVersion
		eagle.ImageType = info.ImageType
		eagle.Manufacturer = info.Manufacturer
		eagle.ModelID = info.ModelID
		eagle.DateCode = info.DateCode

		if eagle.iface == nil {
			eagle.iface = eagle.svc.RegisterInterface(eagle.DeviceMacId, "i.meter")
			eagle.xbosiface = eagle.svc.RegisterInterface(eagle.DeviceMacId, "i.xbos.meter")
		}

		return
	}

	// handle meter data
	if resp.InstantaneousDemand != nil {
		info := resp.InstantaneousDemand
		log.Info("INST DEMAND")
		// update the object with the Meter MAC address
		// but only if we've seen the Eagle before; else, drop this

		// adjust the timestamp with the EAGLE Epoch and get the actual kW demand as a float
		eagle.current_time = int64(*info.TimeStamp+HexInt64(EAGLE_EPOCH)) * 1e9
		eagle.current_demand = float64(*info.Demand) * float64(*info.Multiplier) / float64(*info.Divisor)
		eagle.current_demand *= eagle.multiplier // extra multiplier
		eagle.current_demand *= 1000             // convert to Watts

		eagle.MeterMAC = info.MeterMacId
		eagle.forwardData()

		return
	}

	if resp.PriceCluster != nil {
		info := resp.PriceCluster
		log.Infof("PRICE CLUSTER %+v", info)
		// if this is MAX, then we don't get price
		if info.Price.Int64() == 0xffffffff {
			return
		}
		eagle.current_time = int64(*info.TimeStamp+HexInt64(EAGLE_EPOCH)) * 1e9
		eagle.current_price = float64(*info.Price) / math.Pow(10, float64(*info.TrailingDigits))
		eagle.current_tier = int64(*info.Tier)
		eagle.forwardData()
		return
	}

	if resp.MessageCluster != nil {
		log.Debugf("MessageCluster %+v", resp.MessageCluster)
		return
	}

	if resp.CurrentSummationDelivered != nil {
		// update the object with the Meter MAC address
		// but only if we've seen the Eagle before; else, drop this
		info := resp.CurrentSummationDelivered
		eagle.current_time = int64(*info.TimeStamp+HexInt64(EAGLE_EPOCH)) * 1e9
		eagle.current_summation_delivered = float64(*info.SummationDelivered) * float64(*info.Multiplier) / float64(*info.Divisor)
		eagle.current_summation_received = float64(*info.SummationReceived) * float64(*info.Multiplier) / float64(*info.Divisor)

		eagle.MeterMAC = info.MeterMacId
		eagle.forwardData()

		return
	}

	log.Warning("Got unrecognized message")
}

func (eagle *Eagle) Start() {
	s, err := serial.OpenPort(&serial.Config{Name: eagle.serial_port, Baud: 115200})
	if err != nil {
		log.Fatal(err)
	}
	eagle.svc = eagle.bwclient.RegisterService(eagle.baseuri, "s.eagle")

	closetag := make([]byte, 2)
	for {
		closetag[0] = closetag[1]
		_, err := s.Read(closetag[1:])
		if err != nil {
			log.Error(err)
			continue
		}
		fmt.Printf(string(closetag[1]))
		if bytes.Equal(closetag[:], []byte("/>")) {
			break
		}
	}

	var resp Response
	for {
		dec := xml.NewDecoder(s)
		if err := dec.Decode(&resp); err != nil {
			log.Error(err)
		} else {
			log.Debugf("%+v", resp)
			eagle.HandleMessage(resp)
		}
	}
}

func (eagle *Eagle) forwardData() {
	if eagle.iface == nil {
		log.Notice("cannot forward data; have not gotten networkinfo message yet")
		return
	}

	msg := map[string]interface{}{
		"current_demand":              eagle.current_demand,
		"current_price":               eagle.current_price,
		"current_tier":                eagle.current_tier,
		"current_summation_delivered": eagle.current_summation_delivered,
		"current_summation_received":  eagle.current_summation_received,
		"time": eagle.current_time,
	}
	po, _ := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm("2.0.9.1"), msg)
	err := eagle.iface.PublishSignal("meter", po)
	if err != nil {
		log.Error(errors.Wrap(err, "Could not publish i.meter"))
	}

	xbos_msg := map[string]interface{}{
		"power": eagle.current_demand,
		"time":  eagle.current_time,
	}
	po, _ = bw2.CreateMsgPackPayloadObject(bw2.FromDotForm("2.1.1.4"), xbos_msg)
	err = eagle.xbosiface.PublishSignal("info", po)
	if err != nil {
		log.Error(errors.Wrap(err, "Could not publish i.xbos.meter"))
	}
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

	eagle := &Eagle{
		serial_port: params.MustString("serial_port"),
		multiplier:  float64(params.MustInt("multiplier")),
		baseuri:     baseURI,
		bwclient:    bwClient,
	}

	eagle.Start()

}
