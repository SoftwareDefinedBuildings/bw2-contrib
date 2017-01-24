package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"gopkg.in/immesys/bw2bind.v5"
)

type Driver struct {
	bwc      *bw2bind.BW2Client
	r        DiscoveryRecord
	upd      chan DiscoveryRecord
	base     string
	svc      *bw2bind.Service
	iface    *bw2bind.Interface
	lastheat float64
	lastcool float64
}

func newThermostat(base string, bwc *bw2bind.BW2Client, r DiscoveryRecord) chan DiscoveryRecord {
	d := Driver{base: base, bwc: bwc, r: r, upd: make(chan DiscoveryRecord)}
	d.svc = bwc.RegisterService(base, "s.venstar")
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
func (d *Driver) Control(sm *bw2bind.SimpleMessage) {
	//Commands:
	//{"cmd":"set_away","value": 1 / 0}
	//{"cmd":"set_auto_setpoints", "heattemp": val, "cooltemp": val}
	fmt.Println("got message:")
	sm.Dump()
	cm := make(map[string]interface{})
	for _, po := range sm.POs {
		if po.IsType(bw2bind.PONumMsgPack, bw2bind.POMaskMsgPack) {
			pom, ok := po.(bw2bind.MsgPackPayloadObject)
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
	po, err := bw2bind.CreateMsgPackPayloadObject(bw2bind.PONumVenstarInfo, &inf)
	if err != nil {
		panic(err)
	}

	d.iface.PublishSignal("info", po)
	d.lastheat = inf.HeatTemp
	d.lastcool = inf.CoolTemp
}
