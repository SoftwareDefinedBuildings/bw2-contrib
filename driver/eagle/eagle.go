// This package implements the UPLOADER API described in https://rainforestautomation.com/wp-content/uploads/2014/07/EAGLE-Uploader-API_06.pdf
// The EAGLE sends data in a POST request, e.g.
//   POST /rfaeagle.php HTTP/1.0
//   Host: 192.168.11.3:8888
//   Accept: */*
//   From: nobody@rainforestautomation.com
//   User-Agent: Raven Uploader/v1
//   Content-Length: 483
//   Content-Type: application/x-www-form-urlencoded
//   <?xml version="1.0"?>
//   <rainforest macId="0xf0ad4e00ce69" timestamp="1355292588s">
//   <InstantaneousDemand>
//   <DeviceMacId>0x00158d0000000004</DeviceMacId>
//   <MeterMacId>0x00178d0000000004</MeterMacId>
//   <TimeStamp>0x185adc1d</TimeStamp>
//   <Demand>0x001738</Demand>
//   <Multiplier>0x00000001</Multiplier>
//   <Divisor>0x000003e8</Divisor>
//   <DigitsRight>0x03</DigitsRight>
//   <DigitsLeft>0x00</DigitsLeft>
//   <SuppressLeadingZero>Y</SuppressLeadingZero>
//   </InstantaneousDemand>
//   </rainForest>
//
// Tags are case-insensitive. We need to make sure to extract the MAC address from the header (attribute of the rainforest tag)
// Eagle does expect a response to each POST request. At the very least its 200 OK response, but we can also send commands back
// to the eagle in this reply (only one command per reply)
//
// XML "Fragments" to look for:
//	- Instantaneous Demand
//  - Message (might require a reply?)
//	- CurrentSummation
//  - FastPollStatus
//	- HistoryData
//
// Commands to send:
//  - set_schedule (changes poll rate)
package main

import (
	"encoding/xml"
	"fmt"
	"time"

	"github.com/pkg/errors"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

var EAGLE_EPOCH int64

func init() {
	t, _ := time.Parse("15:04:05 02 Jan 2006", "00:00:00 01 Jan 2000")
	EAGLE_EPOCH = t.Unix()
}

// Represents an instance of an Eagle
type Eagle struct {
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
	iface *bw2.Interface
	// current status of Eagle
	current_demand              float64
	current_price               float64
	current_summation_delivered float64
	current_summation_received  float64
	current_time                int64
	NetworkInfo
}

type Response struct {
	XMLName                   xml.Name
	MacID                     string `xml:"macId,attr"`
	Timestamp                 string `xml:"timestamp,attr"`
	InstantaneousDemand       *InstantaneousDemand
	NetworkInfo               *NetworkInfo
	PriceCluster              *PriceCluster
	MessageCluster            *MessageCluster
	CurrentSummationDelivered *CurrentSummation
}

type InstantaneousDemand struct {
	XMLName             xml.Name
	DeviceMacId         string
	MeterMacId          string
	ActualTimestamp     int64
	ActualDemand        float64
	TimeStamp           *HexInt64
	Demand              *HexInt64
	Multiplier          *HexInt64
	Divisor             *HexInt64
	DigitsRight         *HexInt64
	DigitsLeft          *HexInt64
	SuppressLeadingZero string
}

func (demand *InstantaneousDemand) Dump() {
	fmt.Println("Data Reading: ")
	fmt.Printf("  Device: %s, Meter: %s\n", demand.DeviceMacId, demand.MeterMacId)
	fmt.Printf("  Demand: %f, Timestamp: %d\n", demand.ActualDemand, demand.ActualTimestamp)
	fmt.Printf("  Divisor: %d, Multiplier: %d\n", demand.Divisor.Int64(), demand.Multiplier.Int64())
	fmt.Printf("  Dig Left: %d, Dig Right: %d\n", demand.DigitsLeft.Int64(), demand.DigitsRight.Int64())
}

type NetworkInfo struct {
	XMLName      xml.Name
	DeviceMacId  string
	InstallCode  string
	LinkKey      string
	FWVersion    string
	HWVersion    string
	ImageType    string
	Manufacturer string
	ModelID      string
	DateCode     string
}

type PriceCluster struct {
	XMLName         xml.Name
	DeviceMacId     string
	MeterMacId      string
	TimeStamp       *HexInt64
	Price           *HexInt64
	ActualPrice     float64
	ActualTimestamp int64
	Currency        string
	TrailingDigits  *HexInt64
	Tier            string
	TierLabel       string
	RateLabel       string
}

type MessageCluster struct {
	XMLName              xml.Name
	DeviceMacId          string
	MeterMacId           string
	TimeStamp            string
	Id                   string
	Text                 string
	Priority             string
	ConfirmationRequired string
	Confirmed            string
	Queue                string
}

type CurrentSummation struct {
	XMLName                  xml.Name
	DeviceMacId              string
	MeterMacId               string
	ActualTimestamp          int64
	ActualSummationDelivered float64
	ActualSummationReceived  float64
	TimeStamp                *HexInt64
	SummationDelivered       *HexInt64
	SummationReceived        *HexInt64
	Multiplier               *HexInt64
	Divisor                  *HexInt64
	DigitsRight              *HexInt64
	DigitsLeft               *HexInt64
	SuppressLeadingZero      string
}

func (summ *CurrentSummation) Dump() {
	fmt.Println("Current Summation: ")
	fmt.Printf("  Device: %s, Meter: %s\n", summ.DeviceMacId, summ.MeterMacId)
	fmt.Printf("  Delivered: %f, Received: %f, Timestamp: %d\n", summ.ActualSummationDelivered, summ.ActualSummationReceived, summ.ActualTimestamp)
	fmt.Printf("  Divisor: %d, Multiplier: %d\n", summ.Divisor.Int64(), summ.Multiplier.Int64())
	fmt.Printf("  Dig Left: %d, Dig Right: %d\n", summ.DigitsLeft.Int64(), summ.DigitsRight.Int64())
}

func (srv *EagleServer) forwardData(eagle *Eagle) {
	msg := map[string]interface{}{
		"current_demand":              eagle.current_demand,
		"current_price":               eagle.current_price,
		"current_summation_delivered": eagle.current_summation_delivered,
		"current_summation_received":  eagle.current_summation_received,
		"time": eagle.current_time,
	}
	po, _ := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm("2.0.9.1"), msg)
	err := eagle.iface.PublishSignal("meter", po)
	if err != nil {
		log.Error(errors.Wrap(err, "Could not publish i.meter"))
	}
}
