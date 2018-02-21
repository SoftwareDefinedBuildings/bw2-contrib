package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"time"

	"github.com/levigross/grequests"
	"github.com/pkg/errors"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

const LIGHT_PONUM = `2.1.1.1`
const METER_PONUM = `2.1.1.4`
const OCC_PONUM = `2.1.2.1`

type Fixture struct {
	id          string
	sys         *EnlightedSystem
	light_iface *bw2.Interface
	occ_iface   *bw2.Interface
	meter_iface *bw2.Interface
}

type signal struct {
	State      bool  `msgpack:"state"`
	Brightness int64 `msgpack:"brightness"`
	Ambient    int64 `msgpack:"ambient"`
	Time       int64 `msgpack:"time"`
}

type enlightedState struct {
	Wattage             int64 `json:"wattage,string"`
	Last_occupancy_seen int64 `json:"lastoccupancyseen,string"`
	Light_level         int64 `json:"lightlevel,string"`
	Ambient_light_level int64 `json:"ambientLight,string"`
	Lumens              int64
	Temperature         float64 `json:"temperature,string"`
	Power               float64 `json:"power,string"`
	Name                string  `json:"name"`
}

func (f *Fixture) GetState() (state enlightedState, err error) {
	url := fmt.Sprintf("https://%s/ems/api/org/fixture/details/%s", f.sys.IPAddress, f.id)
	resp, err := grequests.Get(url, f.sys.GetHeaders())
	if err != nil {
		return state, err
	}
	if !resp.Ok {
		fmt.Println(resp.StatusCode)
		fmt.Println(string(resp.Bytes()))
		return state, errors.New(string(resp.Bytes()))
	}
	if err := resp.JSON(&state); err != nil {
		return state, err
	}
	fixture_light_level := state.Light_level

	url = fmt.Sprintf("https://%s/ems/api/org/sensor/v2/details/%s", f.sys.IPAddress, f.id)
	resp, err = grequests.Get(url, f.sys.GetHeaders())
	if err != nil {
		return state, err
	}
	if !resp.Ok {
		fmt.Println(resp.StatusCode)
		fmt.Println(string(resp.Bytes()))
		return state, errors.New(string(resp.Bytes()))
	}
	if err := resp.JSON(&state); err != nil {
		return state, err
	}
	// TODO: the sensor details also give us light level, but I think its lumens?
	// So, we put that in a separate field and make sure to preserve the original
	// brightness level of the light
	state.Lumens = state.Light_level
	state.Light_level = fixture_light_level

	return state, nil
}

func (f *Fixture) SetState(brightness int64, time int64) {
	type fixtures struct {
		XMLName xml.Name `xml:"fixtures"`
		Id      string   `xml:"fixture>id"`
	}
	req := f.sys.GetHeaders()
	req.XML = fixtures{Id: f.id}

	url := fmt.Sprintf("https://%s/ems/api/org/fixture/v1/op/dim/ABS/%d?time=%d", f.sys.IPAddress, brightness, time)
	resp, err := grequests.Post(url, req)
	if err != nil {
		log.Fatal(err)
	}
	if !resp.Ok {
		fmt.Println(resp.StatusCode)
		fmt.Println(string(resp.Bytes()))
		log.Fatal(err)
	}
	fmt.Println(url)
	fmt.Println(resp.String())
}

func (f *Fixture) ListenActuation() {
	f.light_iface.SubscribeSlot("state", func(msg *bw2.SimpleMessage) {
		type actuation struct {
			State      bool  `msgpack:"state"`
			Brightness int64 `msgpack:"brightness"`
		}

		po := msg.GetOnePODF(LIGHT_PONUM)
		pom, ok := po.(bw2.MsgPackPayloadObject)
		if !ok {
			log.Println("Invalid payload")
			msg.Dump()
			return
		}
		var act actuation
		if err := pom.ValueInto(&act); err != nil {
			log.Println(errors.Wrap(err, "Could not unmarshal actuation request"))
		}
		log.Printf("ACTUATION %+v", act)

		if act.Brightness > 100 {
			act.Brightness = 100
		}
		if act.Brightness > 0 {
			f.SetState(act.Brightness, 60) // set for 1 hour
		} else if !act.State {
			f.SetState(0, 60)
		} else if act.State {
			f.SetState(80, 60) // set to 80% for 1 hour
		}

	})
}

func (f *Fixture) PollAndReport(dur time.Duration) {
	for _ = range time.Tick(dur) {
		state, err := f.GetState()
		if err != nil {
			log.Println(errors.Wrapf(err, "Could not read fixture %s", f.id))
			continue
		}

		ts := time.Now().UnixNano()
		msg := &signal{
			State:      state.Power > 0,
			Brightness: state.Light_level,
			Ambient:    state.Ambient_light_level,
			Time:       ts,
		}
		fmt.Printf("%s %+v\n", f.id, state)
		if po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(LIGHT_PONUM), msg); err != nil {
			log.Println(err)
		} else if err = f.light_iface.PublishSignal("info", po); err != nil {
			log.Println(err)
		}

		type occupancy struct {
			Occupied bool  `msgpack:"occupancy"`
			Time     int64 `msgpack:"time"`
		}
		occmsg := &occupancy{
			Occupied: state.Last_occupancy_seen < 30,
			Time:     ts,
		}
		if po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(OCC_PONUM), occmsg); err != nil {
			log.Println(err)
		} else if err = f.occ_iface.PublishSignal("info", po); err != nil {
			log.Println(err)
		}

		type meter struct {
			Power float64 `msgpack:"power"`
			Time  int64   `msgpack:"time"`
		}
		metermsg := &meter{
			Power: state.Power / 1000, // needs to be kW
			Time:  ts,
		}
		if po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(METER_PONUM), metermsg); err != nil {
			log.Println(err)
		} else if err = f.meter_iface.PublishSignal("info", po); err != nil {
			log.Println(err)
		}
	}
}
