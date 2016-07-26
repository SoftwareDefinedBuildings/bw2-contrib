package tosserial

// port from https://github.com/SoftwareDefinedBuildings/smap/blob/master/python/smap/iface/tinyos.py

import (
	"fmt"
	"github.com/tarm/serial"
	"log"
	"sync"
)

const (
	HDLC_FLAG_BYTE   = 0x7e
	HDLC_CTLESC_BYTE = 0x7d
)

type TOSSerialClient struct {
	packet []byte
	sync.Mutex
	Packets chan []byte
}

func NewTOSSerialClient(port string, baudrate int) *TOSSerialClient {
	c := &serial.Config{Name: port, Baud: baudrate}
	tos := &TOSSerialClient{
		packet:  []byte{},
		Packets: make(chan []byte, 100),
	}
	go func() {
		for {
			port, err := serial.OpenPort(c)
			if err != nil {
				log.Fatal(err)
			}
			for {
				buf := make([]byte, 128)
				n, err := port.Read(buf)
				if err != nil {
					fmt.Println(err)
				}
				tos.dataReceived(buf[:n])
			}

		}
	}()
	return tos
}

//Developer notes:
//
//Packet data read from Serial is in this format:
//[HDLC_FLAG_BYTE][Escaped data][HDLC_FLAG_BYTE]
//
//[Escaped data] is encoded so that [HDLC_FLAG_BYTE] byte
//values cannot occur within it. When [Escaped data] has been
//unescaped, the last 2 bytes are a 16-bit CRC of the earlier
//part of the packet (excluding the initial HDLC_FLAG_BYTE
//byte)
//
//It's also possible that the serial device was half-way
//through transmitting a packet when this function was called
//(app was just started). So we also neeed to handle this case:
//
//[Incomplete escaped data][HDLC_FLAG_BYTE][HDLC_FLAG_BYTE][Escaped data][HDLC_FLAG_BYTE]
//
//In this case we skip over the first (incomplete) packet.
//

//Read bytes until we get to a HDLC_FLAG_BYTE value
//(either the end of a packet, or the start of a new one)
func (tos *TOSSerialClient) dataReceived(data []byte) {
	for _, d := range data {
		if d == HDLC_FLAG_BYTE {
			tos.deliver()
		} else {
			tos.Lock()
			tos.packet = append(tos.packet, d)
			tos.Unlock()
		}
	}
}

func (tos *TOSSerialClient) deliver() {
	// decode packet and check CRC
	if len(tos.packet) <= 2 {
		fmt.Println("called deliver with <= 2 bytes")
		tos.Lock()
		tos.packet = []byte{}
		tos.Unlock()
		return
	}
	tos.Lock()
	packet := tos.unescape(tos.packet)
	tos.Unlock()
	tos.packet = []byte{}
	crc := tos.crc16(0, packet[:len(packet)-2])
	packet_crc := tos.decode(packet[len(packet)-2:])
	if crc != packet_crc {
		fmt.Printf("Wrong CRC: %x != %x %v\n", crc, packet_crc, packet)
	}
	if len(packet) > 0 {
		tos.Packets <- packet[:len(packet)-2]
	}
}

func (tos *TOSSerialClient) unescape(packet []byte) []byte {
	var ret []byte
	esc := false
	for _, b := range packet {
		if esc {
			ret = append(ret, b^0x20)
			esc = false
		} else if b == HDLC_CTLESC_BYTE {
			esc = true
		} else {
			ret = append(ret, b)
		}
	}
	return ret
}

func (tos *TOSSerialClient) decode(v []byte) uint16 {
	r := uint16(0)
	for i := len(v) - 1; i >= 0; i-- {
		r = (r << 8) + uint16(v[i])
	}
	return r
}

func (tos *TOSSerialClient) crc16(base_crc uint16, frame_data []byte) uint16 {
	crc := base_crc
	for _, b := range frame_data {
		crc = crc ^ (uint16(b) << 8)
		for i := 0; i < 8; i++ {
			if crc&0x8000 == 0x8000 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc = (crc << 1)
			}
			crc = crc & 0xffff
		}
	}
	return crc
}
