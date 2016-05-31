package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/immesys/spawnpoint/spawnable"
	bw "gopkg.in/immesys/bw2bind.v5"
)

var btok string
var lid string

const minInterval = 1 * time.Second
const maxInterval = 30 * time.Minute
const adjustTime = 10 * time.Second

var resetTime time.Time
var interval time.Duration
var preempt chan struct{}
var ifc *bw.Interface

func main() {
	cl := bw.ConnectOrExit("")
	cl.SetEntityFromEnvironOrExit()
	params := spawnable.GetParamsOrExit()
	params.MergeMetadata(cl)

	interval = minInterval
	resetTime = time.Now()
	preempt = make(chan struct{}, 1)
	uri := params.MustString("svc_base_uri")
	btok = params.MustString("bearer_token")
	lid = params.MustString("light_id")

	svc := cl.RegisterService(uri, "s.lifx")
	ifc = svc.RegisterInterface("0", "i.hsb-light")

	ifc.SubscribeSlot("hsb", dispatch)
	for {
		select {
		case <-preempt:
		case <-time.After(interval):
		}
		updateColor()
		if time.Now().Sub(resetTime) > adjustTime {
			interval *= 2
			if interval > maxInterval {
				interval = maxInterval
			}
			resetTime = time.Now()
		}
	}
}

type State struct {
	Color struct {
		Hue        float64
		Saturation float64
	}
	Brightness float64
	Power      string
}

var lastHue float64
var lastSat float64
var lastBrightness float64

func updateColor() {
	client := &http.Client{}
	uri := fmt.Sprintf("https://api.lifx.com/v1/lights/%s", lid)
	req, _ := http.NewRequest("GET", uri, nil)
	req.Header.Add("Content-Type", `application/json`)
	req.Header.Add("Authorization", "Bearer "+btok)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Got HTTP GET error: ", err)
		return
	}
	contents, _ := ioutil.ReadAll(resp.Body)
	cs := []State{}
	err = json.Unmarshal(contents, &cs)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(cs)
	fmt.Println(string(contents))
	type hsbcmd struct {
		Hue        float64 `msgpack:"hue"`
		Saturation float64 `msgpack:"saturation"`
		Brightness float64 `msgpack:"brightness"`
		State      bool    `msgpack:"state"`
	}
	c := hsbcmd{
		Hue:        cs[0].Color.Hue / 360.0,
		Saturation: cs[0].Color.Saturation,
		Brightness: cs[0].Brightness,
		State:      cs[0].Power == "on",
	}
	po, _ := bw.CreateMsgPackPayloadObject(bw.PONumHSBLightMessage, &c)
	err = ifc.PublishSignal("hsb", po)
	if err != nil {
		panic(err)
	}
	if lastHue != cs[0].Color.Hue ||
		lastSat != cs[0].Color.Saturation ||
		lastBrightness != cs[0].Brightness {
		interval = minInterval
		resetTime = time.Now()
	}
	lastHue = cs[0].Color.Hue
	lastSat = cs[0].Color.Saturation
	lastBrightness = cs[0].Brightness
}
func dispatch(m *bw.SimpleMessage) {
	m.Dump()
	po := m.GetOnePODF(bw.PODFHSBLightMessage)
	if po == nil {
		return
	}
	interval = minInterval
	resetTime = time.Now()
	var v map[string]interface{}
	po.(bw.MsgPackPayloadObject).ValueInto(&v)
	hue, hashue := v["hue"].(float64)
	sat, hassat := v["saturation"].(float64)
	bri, hasbri := v["brightness"].(float64)
	sta, hassta := v["state"].(bool)

	colorstr := ""
	pstr := "on"
	if hashue {
		clamp(&hue)
		colorstr += fmt.Sprintf("hue:%.3f ", hue*360)
	}
	if hassat {
		clamp(&sat)
		colorstr += fmt.Sprintf("saturation:%.3f ", sat)
	}
	if hasbri {
		clamp(&bri)
		colorstr += fmt.Sprintf("brightness:%.3f ", bri)
	}
	if hassta && !sta {
		pstr = "off"
	}

	if colorstr != "" {
		colorstr = ",\"color\":\"" + colorstr + "\""
	}
	msg := fmt.Sprintf("{\"power\":\"%s\",\"duration\":0.1%s}", pstr, colorstr)

	fmt.Println(spawnable.DoHttpPutStr(fmt.Sprintf("https://api.lifx.com/v1/lights/%s/state", lid),
		msg, []string{"Content-Type", `application/json`,
			"Authorization", "Bearer " + btok}))
	time.Sleep(50 * time.Millisecond)
	preempt <- struct{}{}
}

func clamp(f *float64) {
	if *f > 1.0 {
		*f = 1.0
	}
	if *f < 0 {
		*f = 0
	}
}
