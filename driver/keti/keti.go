package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/immesys/bw2-contrib/lib/tosserial"
)

const (
	TYPE_TH  = 0x64
	TYPE_PIR = 0x65
	TYPE_CO2 = 0x66

	//SDH : not sure these are exactly right: should be changed to
	//match the SHT11 settings based on Table 6 and Table 8 in the
	//SHT11 data sheet.
	SHT11_D1 = -40.1
	SHT11_D2 = 0.01

	SHT11_C1 = -4.0
	SHT11_C2 = 0.0405
	SHT11_C3 = -2.8e-6
)

type KetiTempReading struct {
	Temperature float64
	Humidity    float64
	Lux         float64
	NodeID      uint16
	SerialID    [6]byte
}

type KetiPIRReading struct {
	PIR      float64
	NodeID   uint16
	SerialID [6]byte
}

type KetiCO2Reading struct {
	CO2      float64
	NodeID   uint16
	SerialID [6]byte
}

type KetiMoteReceiver struct {
	serial       *tosserial.TOSSerialClient
	TempReadings chan KetiTempReading
	PIRReadings  chan KetiPIRReading
	CO2Readings  chan KetiCO2Reading
}

func NewKetiMoteReceiver(serialPort string, baudrate int) *KetiMoteReceiver {
	keti := &KetiMoteReceiver{
		serial:       tosserial.NewTOSSerialClient(serialPort, baudrate),
		TempReadings: make(chan KetiTempReading, 100),
		PIRReadings:  make(chan KetiPIRReading, 100),
		CO2Readings:  make(chan KetiCO2Reading, 100),
	}
	go func() {
		for packet := range keti.serial.Packets {
			keti.handlePacket(packet)
		}
	}()

	return keti
}

func (keti *KetiMoteReceiver) handlePacket(packet []byte) {
	if len(packet) != 29 {
		fmt.Printf("Packet length was not 29. Was %d\n", len(packet))
		for i, b := range packet {
			fmt.Printf("%02x ", b)
			if i == len(packet)-1 {
				fmt.Printf("\n")
			}
		}
		return
	}
	var (
		typ       uint16
		serial_id [6]byte
		node_id   uint16
		seq       uint16
		bat       uint16
		sensor    [6]byte
		err       error
	)
	buf := bytes.NewBuffer(packet[9:29])
	if err = binary.Read(buf, binary.BigEndian, &typ); err != nil {
		fmt.Printf("Error reading type: %v\n", err)
		return
	}
	if err = binary.Read(buf, binary.BigEndian, &serial_id); err != nil {
		fmt.Printf("Error reading serial_id: %v\n", err)
		return
	}
	if err = binary.Read(buf, binary.BigEndian, &node_id); err != nil {
		fmt.Printf("Error reading node_id: %v\n", err)
		return
	}
	if err = binary.Read(buf, binary.BigEndian, &seq); err != nil {
		fmt.Printf("Error reading seq: %v\n", err)
		return
	}
	if err = binary.Read(buf, binary.BigEndian, &bat); err != nil {
		fmt.Printf("Error reading bat: %v\n", err)
		return
	}
	if err = binary.Read(buf, binary.BigEndian, &sensor); err != nil {
		fmt.Printf("Error reading sensor: %v\n", err)
		return
	}

	//TODO: add cache for tuples of (node_id, seq) to avoid duplicates. Don't forget a timeout
	sbuf := bytes.NewBuffer(sensor[:])

	if typ == TYPE_TH {
		var (
			_temp     uint16
			_humidity uint16
			lux       uint16
		)
		if err = binary.Read(sbuf, binary.BigEndian, &_temp); err != nil {
			fmt.Printf("Error reading _temp: %v\n", err)
			return
		}
		if err = binary.Read(sbuf, binary.BigEndian, &_humidity); err != nil {
			fmt.Printf("Error reading _humidity: %v\n", err)
			return
		}
		if err = binary.Read(sbuf, binary.BigEndian, &lux); err != nil {
			fmt.Printf("Error reading lux: %v\n", err)
			return
		}
		// adjsut values
		temp := SHT11_D1 + SHT11_D2*float64(_temp)
		humidity := SHT11_C1 + SHT11_C2*float64(_humidity) + SHT11_C3*float64(_humidity*_humidity)
		keti.TempReadings <- KetiTempReading{Temperature: temp, Humidity: humidity, Lux: float64(lux), NodeID: node_id, SerialID: serial_id}
		return
	}

	if typ == TYPE_PIR {
		var pir uint16
		if err = binary.Read(sbuf, binary.BigEndian, &pir); err != nil {
			fmt.Printf("Error reading pir: %v\n", err)
			return
		}
		keti.PIRReadings <- KetiPIRReading{PIR: float64(pir), NodeID: node_id, SerialID: serial_id}
		return
	}

	if typ == TYPE_CO2 {
		var co2 uint16
		if err = binary.Read(sbuf, binary.BigEndian, &co2); err != nil {
			fmt.Printf("Error reading co2: %v\n", err)
			return
		}
		keti.CO2Readings <- KetiCO2Reading{CO2: float64(co2), NodeID: node_id, SerialID: serial_id}
		return
	}
}
