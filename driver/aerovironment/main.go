package main

import (
	"bufio"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/immesys/ragent/ragentlib"
	"github.com/immesys/spawnpoint/spawnable"
	"github.com/tarm/serial"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

const JUICEPLUG_DF = "2.1.1.7"

type XBOS_EVSE struct {
	Current_limit      float64 `msgpack:"current_limit"`
	Current            float64 `msgpack:"current"`
	Voltage            float64 `msgpack:"voltage"`
	Charging_time_left int64   `msgpack:"charging_time_left"`
	State              bool    `msgpack:"state"`
	Time               int64   `msgpack:"time"`
}
type write_params struct {
	Current_limit *float64 `msgpack:"current_limit"`
	State         *bool    `msgpack:"state"`
}

type EnableState uint

const (
	EnableState_Disabled = iota
	EnableState_Enabled
	EnableState_Charging
)

type PilotState uint

const (
	PilotState_NoVehicle = iota
	PilotState_VehicleConnectedContactOpen
	PilotState_VehicleConnectedContactorClosed1
	PilotState_VehicleConnectedContactorClosed2
	PilotState_Fault
)

// - button_state: 0 (no buttons), 1 (start pushed), 2 (stop pushed), 3 (both pushed)

type ButtonState uint

const (
	ButtonState_NoButtons = iota
	ButtonState_StartPushed
	ButtonState_StopPushed
	ButtonState_BothPushed
)

type AerovironmentStatus struct {
	EnableState           EnableState
	PilotState            PilotState
	FaultCode             int
	MaxChargeAmps         int
	RMSVoltage            float64
	Current               float64
	Power                 float64
	Frequency             float64
	ChargeSessionEnergy   float64
	ChargeSessionDuration int64
	ButtonState           ButtonState

	Time time.Time
}

type Aerovironment struct {
	status   *AerovironmentStatus
	port     *serial.Port
	readings chan AerovironmentStatus
	sync.Mutex
}

func NewAerovironment(port string, baud int) (*Aerovironment, error) {
	c := &serial.Config{Name: "/dev/ttyAMA0", Baud: 57600}
	s, err := serial.OpenPort(c)
	if err != nil {
		return nil, err
	}

	aero := &Aerovironment{
		status:   new(AerovironmentStatus),
		port:     s,
		readings: make(chan AerovironmentStatus, 10),
	}

	// start read loop
	go func() {
		buf := bufio.NewReader(s)
		for {
			s, err := buf.ReadString('\n')
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(s)
			if err = aero.parseStatusLine(s); err != nil {
				log.Println(err)
			} else {
				aero.Lock()
				aero.readings <- *aero.status
				aero.Unlock()
			}

		}
	}()

	return aero, nil
}

func (aero *Aerovironment) parseStatusLine(line string) error {
	aero.Lock()
	defer aero.Unlock()

	parts := strings.Split(line, ",")
	if len(parts) != 11 {
		return fmt.Errorf("Line %s is incomplete", line)
	}

	// EnableState
	es, err := strconv.Atoi(parts[0])
	if err != nil {
		return err
	}
	aero.status.EnableState = EnableState(es)

	// PilotState
	ps, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}
	aero.status.PilotState = PilotState(ps)

	// FaultCode
	aero.status.FaultCode, err = strconv.Atoi(parts[2])
	if err != nil {
		return err
	}

	// MaxChargeAmps
	aero.status.MaxChargeAmps, err = strconv.Atoi(parts[3])
	if err != nil {
		return err
	}

	// RMSVoltage
	voltage, err := strconv.ParseFloat(parts[4], 64)
	if err != nil {
		return err
	}
	aero.status.RMSVoltage = float64(voltage) / 10

	// Current
	current, err := strconv.ParseFloat(parts[5], 64)
	if err != nil {
		return err
	}
	aero.status.Current = float64(current) / 10

	// power
	aero.status.Power, err = strconv.ParseFloat(parts[6], 64)
	if err != nil {
		return err
	}

	// frequency
	aero.status.Frequency, err = strconv.ParseFloat(parts[7], 64)
	if err != nil {
		return err
	}
	aero.status.Frequency *= 100 // .01Hz -> 1Hz

	// energy
	aero.status.ChargeSessionEnergy, err = strconv.ParseFloat(parts[8], 64)
	if err != nil {
		return err
	}

	// duration
	aero.status.ChargeSessionDuration, err = strconv.ParseInt(parts[9], 10, 64)
	if err != nil {
		return err
	}

	// button state
	bs, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return err
	}
	aero.status.ButtonState = ButtonState(bs)

	aero.status.Time = time.Now()

	return nil
}

func (aero *Aerovironment) Enable() error {
	log.Println("Enable Aerovironment")
	_, err := aero.port.Write([]byte("*enable"))
	return err
}

func (aero *Aerovironment) Disable() error {
	log.Println("Disable Aerovironment")
	_, err := aero.port.Write([]byte("*disable"))
	return err
}

func (aero *Aerovironment) SetCurrentLimit(c int) error {
	log.Printf("Set current limit to %d", c)
	if c < 0 || c < 6 || c > 32 {
		return fmt.Errorf("current limit %d is out of range 0, 6-32", c)
	}
	_, err := aero.port.Write([]byte("*curr_lim," + strconv.Itoa(c)))
	return err
}

func (aero *Aerovironment) SetDefaultCurrentLimit(c int) error {
	log.Printf("Set DEFAULT current limit to %d", c)
	if c < 0 || c < 6 || c > 32 {
		return fmt.Errorf("DEFAULT current limit %d is out of range 0, 6-32", c)
	}
	_, err := aero.port.Write([]byte("*curr_lim_def," + strconv.Itoa(c)))
	return err
}

func (aero *Aerovironment) SetTimeout(c int) error {
	log.Printf("Set communication timeout to %d", c)
	if c < 5 || c > 300 {
		return fmt.Errorf("Timeout %d is out of range 5-300 (seconds)", c)
	}
	_, err := aero.port.Write([]byte("*timeout," + strconv.Itoa(c)))
	return err
}

// How to parse each line: we have 3 options
// 		Totalizer 66.379 kWh
//
// 		Last Charge 14.093 kWh
//
// 		0,0,0,32,2204,1,0,5999,0,000,0 <-- this is the one we are most interested in
//
// The fields, in order, for the last line are:
// - enable_state: 0=>disabled, 1=>enabled, 2=>charging
// - pilot_state: {0: 12v (no vehicle), 1: 9v (vehicle connected, contactor opend),
//				   2: 6v (vehicle connected, contactor closed), 3: 3v (vehicle connected, contactor closed)
//				   4: -12v (fault)}
// - fault_code: 0 => no fault
// - max_amps: maximum amps
// - volt_meas: measured AC line voltage, RMS, integer. Scaled up by 10 (so divide to get avlue)
// - curr_meas: measured current in amps. integer, scaled up by 10
// - power: power in watts, integer
// - frequency: measured AC lien frequency. integer in .01Hz
// - energy: accumulated energy in Watt-seconds for charge session
// - duration of charge, integer seconds
// - button_state: 0 (no buttons), 1 (start pushed), 2 (stop pushed), 3 (both pushed)

var _agent = "127.0.0.1:28588"

func init() {
	go func() {
		defer func() {
			r := recover()
			if r != nil {
				log.Fatal(fmt.Sprintf("Failed to connect ragent (%v)", r))
			}
		}()
		fmt.Println("connecting")
		ragentlib.DoClientER([]byte(_entity), "ragent.cal-sdb.org:28590", "MT3dKUYB8cnIfsbnPrrgy8Cb_8whVKM-Gtg2qd79Xco=", _agent)
	}()
}

func main() {
	time.Sleep(5 * time.Second)
	bwClient := bw2.ConnectOrExit(_agent)
	bwClient.OverrideAutoChainTo(true)
	bwClient.SetEntityFromEnvironOrExit()

	params := spawnable.GetParamsOrExit()
	baseURI := params.MustString("svc_base_uri")
	if !strings.HasSuffix(baseURI, "/") {
		baseURI += "/"
	}
	location := params.MustString("location")
	port := params.MustString("port")
	baud := params.MustInt("baud")

	driver, err := NewAerovironment(port, baud)
	if err != nil {
		log.Fatal(err)
	}

	service := bwClient.RegisterService(baseURI, "s.aerovironment")
	iface := service.RegisterInterface(location, "i.xbos.evse")
	fmt.Println("publishing on", iface.SignalURI("info"), "subscribing on", iface.SlotURI("state"))

	// write loop
	iface.SubscribeSlot("state", func(msg *bw2.SimpleMessage) {
		msg.Dump()
		po := msg.GetOnePODF(JUICEPLUG_DF)
		if po == nil {
			fmt.Println("Received message on state slot without required PO. Dropping.")
			return
		}
		var params write_params
		if err := po.(bw2.MsgPackPayloadObject).ValueInto(&params); err != nil {
			fmt.Println("Received malformed PO on state slot. Dropping.", err)
			return
		}

		if params.State != nil && *params.State {
			if err := driver.Enable(); err != nil {
				log.Println(err)
			}
		} else if params.State != nil && !*params.State {
			if err := driver.Disable(); err != nil {
				log.Println(err)
			}
		}

		if params.Current_limit != nil {
			if err := driver.SetCurrentLimit(int(*params.Current_limit)); err != nil {
				log.Println(err)
			}
		}
	})

	// read loop
	for status := range driver.readings {
		fmt.Printf("%+v\n", status)
		signal := XBOS_EVSE{
			Current_limit: float64(status.MaxChargeAmps),
			Current:       status.Current,
			Voltage:       status.RMSVoltage,
			State:         status.EnableState == EnableState_Enabled || status.EnableState == EnableState_Charging,
			Time:          status.Time.UnixNano(),
		}
		po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(JUICEPLUG_DF), signal)
		if err != nil {
			log.Println("Could not publish", err)
			continue
		}
		if err = iface.PublishSignal("info", po); err != nil {
			log.Println("Could not publish", err)
			continue
		}
	}
}
