package main

import (
	"encoding/xml"
	"github.com/parnurzeal/gorequest"
	"github.com/pkg/errors"
	"log"
	"time"
)

type MTU struct {
	XMLName    xml.Name
	Name       string
	PowerNow   float64 `xml:"PowerNow"`
	VoltageNow float64 `xml:"VoltageNow"`
	KVANow     float64 `xml:"kVANow"`
	Timestamp  int64   `xml:"Timestamp"`
	//PhaseCurrent PhaseCurrent `xml:"PhaseCurrent"`
	//PhaseVoltage PhaseVoltage `xml:"PhaseVoltage"`
}

type Result struct {
	XMLName xml.Name
	MTUs    []*MTU `xml:",any"`
}

//type Data struct {
//	Name string
//	Voltage Voltage
//	Power   Power
//}

//}

type TED struct {
	URL      string
	interval time.Duration
	req      *gorequest.SuperAgent
}

func NewTEDSource(url, poll_interval string) *TED {
	dur, err := time.ParseDuration(poll_interval)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Could not parse given poll interval"))
	}
	return &TED{
		URL:      url,
		interval: dur,
		req:      gorequest.New(),
	}
}

func (ted *TED) Start() chan *MTU {
	var c = make(chan *MTU)
	go func() {
		if datas, err := ted.Read(); err == nil {
			for _, d := range datas {
				c <- d
			}
		}
		for _ = range time.Tick(ted.interval) {
			if datas, err := ted.Read(); err == nil {
				for _, d := range datas {
					c <- d
				}
			}
		}
	}()

	return c
}

func (ted *TED) Read() (map[string]*MTU, error) {
	ret := make(map[string]*MTU)

	log.Println(ted.URL)
	resp, _, errs := ted.req.Get(ted.URL).End()
	if errs != nil {
		for _, err := range errs {
			log.Println(errors.Wrap(err, "Could not fetch URL"))
			return ret, err
		}
	}
	defer resp.Body.Close()
	var res Result
	dec := xml.NewDecoder(resp.Body)
	if err := dec.Decode(&res); err != nil {
		log.Println(errors.Wrap(err, "Could not decode XML"))
		return ret, err
	}
	res.MTUs[0].Name = "MTU1"

	for _, mtu := range res.MTUs[:1] {
		log.Printf("Name: %s, Power: %0.2f, Voltage: %0.2f, kVA: %0.2f, Timestamp: %d", mtu.Name, mtu.PowerNow, mtu.VoltageNow, mtu.KVANow, mtu.Timestamp)
		ret[mtu.Name] = mtu
	}

	return ret, nil
}
