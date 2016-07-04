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
	XMLName    xml.Name
	VoltageNow float64 `xml:"VoltageNow"`
}

type PowerList struct {
	XMLName xml.Name
	Powers  []Power `xml:",any"`
}

type Power struct {
	XMLName  xml.Name
	PowerNow float64 `xml:"PowerNow"`
	KVA      float64 `xml:"KVA"`
}

type Data struct {
	Name       string
	VoltageNow float64
	PowerNow   float64
	KVA        float64
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
		log.Fatalln(errors.Wrap(err, "Could not decode XML"))
	}
	for _, v := range res.Voltage.Voltages {
		for _, name := range ted.toExtract {
			if name == v.XMLName.Local {
				ret[name].VoltageNow = v.VoltageNow
			}
		}
	}
	for _, p := range res.Power.Powers {
		for _, name := range ted.toExtract {
			if name == p.XMLName.Local {
				ret[name].PowerNow = p.PowerNow
				ret[name].KVA = p.KVA
			}
		}
	}

	return ret, nil
}
