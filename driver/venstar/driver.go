package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/satori/go.uuid"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

const (
	NAMESPACE_UUID = `d8b61708-2797-11e6-836b-0cc47a0f7eea`
	PONUM          = "2.1.1.0"
)

func (ir *InfoResponse) ToMsgPackPO() bw2.PayloadObject {
	po, err := bw2.CreateMsgPackPayloadObject(bw2.PONumTimeseriesReading, ir)
	if err != nil {
		panic(err)
	}
	return po
}

func NewXbosInfoPO(time int64, temp float64, relHumidity float64, heatingSetpoint float64, coolingSetpoint float64, override bool, fan bool, mode int, state int) bw2.PayloadObject {
	msg := map[string]interface{}{
		"temperature":       temp,
		"relative_humidity": relHumidity,
		"heating_setpoint":  heatingSetpoint,
		"cooling_setpoint":  coolingSetpoint,
		"override":          override,
		"fan":               fan,
		"mode":              mode,
		"state":             state,
		"time":              time}
	po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(PONUM), msg)
	if err != nil {
		panic(err)
	}
	return po
}

type Driver struct {
	bwc            *bw2.BW2Client
	r              DiscoveryRecord
	upd            chan DiscoveryRecord
	base           string
	svc            *bw2.Service
	iface          *bw2.Interface
	xbos_iface     *bw2.Interface
	lastheat       float64
	lastcool       float64
	override       bool
	fan            bool
	timeseriesUUID string
}

func newThermostat(base string, bwc *bw2.BW2Client, r DiscoveryRecord) chan DiscoveryRecord {
	d := Driver{base: base, bwc: bwc, r: r, upd: make(chan DiscoveryRecord)}
	d.svc = bwc.RegisterService(base, "s.venstar")

	rootUUID := uuid.FromStringOrNil(NAMESPACE_UUID)
	d.timeseriesUUID = uuid.NewV3(rootUUID, "info").String()

	go d.Start()
	return d.upd
}

func (d *Driver) Start() {
	go func() {
		//We do not use this at the moment
		for _ = range d.upd {
		}
	}()
	d.iface = d.svc.RegisterInterface(d.r.Name, "i.venstar")
	d.iface.SubscribeSlot("control", d.Control)

	d.xbos_iface = d.svc.RegisterInterface(d.r.Name, "i.xbos.thermostat")
	d.xbos_iface.SubscribeSlot("setpoints", func(msg *bw2.SimpleMessage) {
		fmt.Println("got message from slot setpoints:")
		msg.Dump()

		po := msg.GetOnePODF(PONUM)
		if po == nil {
			fmt.Println("Received actuation command without valid PO, dropping")
			return
		}

		msgpo, err := bw2.LoadMsgPackPayloadObject(po.GetPONum(), po.GetContents())
		if err != nil {
			fmt.Println(err)
			return
		}

		var data map[string]interface{}
		err = msgpo.ValueInto(&data)
		if err != nil {
			fmt.Println(err)
			return
		}

		heat := data["heating_setpoint"].(float64)
		cool := data["cooling_setpoint"].(float64)
		d.SetSetpoints(
			nil,
			&heat,
			&cool)
	})

	d.xbos_iface.SubscribeSlot("state", func(msg *bw2.SimpleMessage) {
		fmt.Println("got message from slot state:")
		msg.Dump()
		cm := make(map[string]interface{})

		po := msg.GetOnePODF(PONUM)
		if po == nil {
			fmt.Println("Received actuation command without valid PO, dropping")
			return
		}

		msgpo, err := bw2.LoadMsgPackPayloadObject(po.GetPONum(), po.GetContents())
		if err != nil {
			fmt.Println(err)
			return
		}

		var data map[string]interface{}
		err = msgpo.ValueInto(&data)
		if err != nil {
			fmt.Println(err)
			return
		}

		mode := int(data["mode"].(uint64))
		heat := data["heating_setpoint"].(float64)
		cool := data["cooling_setpoint"].(float64)
		d.SetSetpoints(
			&mode,
			&heat,
			&cool)
		d.override = data["override"].(bool)
		d.fan = data["fan"].(bool)
	})

	for {

		d.Scrape()
		time.Sleep(10 * time.Second)
	}
}

func (d *Driver) SetSetpoints(mode *int, heat *float64, cool *float64) {
	if heat == nil {
		heat = &d.lastheat
	}
	if cool == nil {
		cool = &d.lastcool
	}
	if mode == nil {
		auto := 3
		mode = &auto
	}
	resp, err := http.PostForm("http://"+d.r.IP+"/control", url.Values{
		"mode":     {fmt.Sprintf("%d", *mode)},
		"heattemp": {fmt.Sprintf("%d", int(*heat))},
		"cooltemp": {fmt.Sprintf("%d", int(*cool))},
	})
	if err != nil {
		fmt.Println("SET FAILURE: ", err)
	}
	contents, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("set response: ", string(contents))
	resp.Body.Close()
}
func (d *Driver) SetAway(val int) {
	resp, err := http.PostForm("http://"+d.r.IP+"/settings", url.Values{
		"away": {fmt.Sprintf("%d", val)},
	})
	if err != nil {
		fmt.Println("SET FAILURE: ", err)
	}
	contents, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("set response: ", string(contents))
	resp.Body.Close()
}
func (d *Driver) Control(sm *bw2.SimpleMessage) {
	//Commands:
	//{"cmd":"set_away","value": 1 / 0}
	//{"cmd":"set_auto_setpoints", "heattemp": val, "cooltemp": val}
	fmt.Println("got message:")
	sm.Dump()
	cm := make(map[string]interface{})
	for _, po := range sm.POs {
		if po.IsType(bw2.PONumMsgPack, bw2.POMaskMsgPack) {
			pom, ok := po.(bw2.MsgPackPayloadObject)
			if !ok {
				fmt.Println("skipping invalid command")
				continue
			}
			pom.ValueInto(&cm)
			fmt.Println("got PO:", cm)
			if ok {
				cmd, ok := cm["cmd"]
				if ok {
					switch cmd {
					case "set_away":
						val, ok := cm["value"].(float64)
						if !ok {
							fmt.Println("DROPPING COMMAND set_away - invalid 'value'")
							continue
						}
						d.SetAway(int(val))
					case "set_auto_setpoints":
						heattemp, hok := cm["heattemp"].(float64)
						cooltemp, cok := cm["cooltemp"].(float64)
						var ht *float64 = nil
						var ct *float64 = nil
						if hok {
							ht = &heattemp
						}
						if cok {
							ct = &cooltemp
						}
						d.SetSetpoints(nil, ht, ct)
					}
				}
			}
		}
	}
}

func (d *Driver) Scrape() {
	resp, err := http.Get("http://" + d.r.IP + "/query/info")
	contents, _ := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	resp.Body.Close()
	inf := InfoResponse{}
	json.Unmarshal(contents, &inf)
	inf.Time = time.Now().UnixNano()
	inf.UUID = d.timeseriesUUID
	po := inf.ToMsgPackPO()

	d.iface.PublishSignal("info", po)
	xbosPO := NewXbosInfoPO(
		inf.Time,
		inf.SpaceTemp,
		0.0,
		inf.HeatTemp,
		inf.CoolTemp,
		d.override,
		d.fan,
		inf.Mode,
		inf.State)
	d.xbos_iface.PublishSignal("info", xbosPO)
	d.lastheat = inf.HeatTemp
	d.lastcool = inf.CoolTemp
}
