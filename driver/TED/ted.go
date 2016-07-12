package main

import (
	"encoding/xml"
	"github.com/parnurzeal/gorequest"
	"github.com/pkg/errors"
	"log"
	"time"
)

type Result struct {
	XMLName xml.Name
	Voltage VoltageList `xml:"Voltage"`
	Power   PowerList   `xml:"Power"`
}

type VoltageList struct {
	XMLName  xml.Name
	Voltages []Voltage `xml:",any"`
}

type Voltage struct {
	XMLName          xml.Name
	VoltageNow       float64 `xml:"VoltageNow"`
	LowVoltageHour   float64
	LowVoltageToday  float64
	LowVoltageMTD    float64
	HighVoltageHour  float64
	HighVoltageToday float64
	HighVoltageMTD   float64
}

type PowerList struct {
	XMLName xml.Name
	Powers  []Power `xml:",any"`
}

type Power struct {
	XMLName   xml.Name
	PowerNow  float64 `xml:"PowerNow"`
	PowerHour float64
	PowerTDY  float64
	PowerMTD  float64
	PowerProj float64
	PeakTdy   float64
	PeakMTD   float64
	MinTdy    float64
	MinMTD    float64
	KVA       float64 `xml:"KVA"`
}

type Data struct {
	Name             string
	VoltageNow       float64
	LowVoltageHour   float64
	LowVoltageToday  float64
	LowVoltageMTD    float64
	HighVoltageHour  float64
	HighVoltageToday float64
	HighVoltageMTD   float64
	PowerNow         float64
	PowerHour        float64
	PowerTDY         float64
	PowerMTD         float64
	PowerProj        float64
	PeakTdy          float64
	PeakMTD          float64
	MinTdy           float64
	MinMTD           float64
	KVA              float64
}

type TED struct {
	URL       string
	toExtract []string
	interval  time.Duration
	req       *gorequest.SuperAgent
}

func NewTEDSource(url, poll_interval string, toExtract []string) *TED {
	dur, err := time.ParseDuration(poll_interval)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Could not parse given poll interval"))
	}
	return &TED{
		URL:       url,
		interval:  dur,
		toExtract: toExtract,
		req:       gorequest.New(),
	}
}

func (ted *TED) Start() chan *Data {
	var c = make(chan *Data)
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

func (ted *TED) Read() (map[string]*Data, error) {
	ret := make(map[string]*Data)
	for _, name := range ted.toExtract {
		ret[name] = &Data{Name: name}
	}

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
	for _, v := range res.Voltage.Voltages {
		for _, name := range ted.toExtract {
			if name == v.XMLName.Local {
				ret[name].VoltageNow = v.VoltageNow
				ret[name].LowVoltageHour = v.LowVoltageHour
				ret[name].LowVoltageToday = v.LowVoltageToday
				ret[name].LowVoltageMTD = v.LowVoltageMTD
				ret[name].HighVoltageHour = v.HighVoltageHour
				ret[name].HighVoltageToday = v.HighVoltageToday
				ret[name].HighVoltageMTD = v.HighVoltageMTD
			}
		}
	}
	for _, p := range res.Power.Powers {
		for _, name := range ted.toExtract {
			if name == p.XMLName.Local {
				ret[name].PowerNow = p.PowerNow
				ret[name].PowerHour = p.PowerHour
				ret[name].PowerTDY = p.PowerTDY
				ret[name].PowerMTD = p.PowerMTD
				ret[name].PowerProj = p.PowerProj
				ret[name].PeakTdy = p.PeakTdy
				ret[name].PeakMTD = p.PeakMTD
				ret[name].MinTdy = p.MinTdy
				ret[name].MinMTD = p.MinMTD
				ret[name].KVA = p.KVA
			}
		}
	}

	return ret, nil
}
