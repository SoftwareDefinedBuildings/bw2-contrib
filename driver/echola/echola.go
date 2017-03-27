package main

import (
	"encoding/xml"
	"errors"
	"fmt"

	"github.com/parnurzeal/gorequest"
)

type Echola struct {
	ipAddr string
	req    *gorequest.SuperAgent
}

type PlugState struct {
	Enabled int32
	Power   float64
}

type echolaResponse struct {
	PState1 int32 `xml:"pstate1"`
	PState2 int32 `xml:"pstate2"`
	PState3 int32 `xml:"pstate3"`
	PState4 int32 `xml:"pstate4"`
	PState5 int32 `xml:"pstate5"`
	PState6 int32 `xml:"pstate6"`
	PState7 int32 `xml:"pstate7"`
	PState8 int32 `xml:"pstate8"`

	Pow1 float64 `xml:"pow1"`
	Pow2 float64 `xml:"pow2"`
	Pow3 float64 `xml:"pow3"`
	Pow4 float64 `xml:"pow4"`
	Pow5 float64 `xml:"pow5"`
	Pow6 float64 `xml:"pow6"`
	Pow7 float64 `xml:"pow7"`
	Pow8 float64 `xml:"pow8"`

	PowT float64 `xml:"powt"`
}

func NewEchola(ipAddr string) *Echola {
	return &Echola{
		ipAddr: ipAddr,
		req:    gorequest.New(),
	}
}

func (echola *Echola) GetStatus() (float64, []PlugState, error) {
	var results []PlugState
	dest := fmt.Sprintf("http://%s/api.xml", echola.ipAddr)
	resp, _, errs := echola.req.Get(dest).End()
	if errs != nil {
		return 0.0, results, fmt.Errorf("Error retrieving plug status: %v", errs)
	} else if resp.StatusCode != 200 {
		return 0.0, results, fmt.Errorf("Error retrieving plug status: %s", resp.Status)
	}

	defer resp.Body.Close()
	var response echolaResponse
	dec := xml.NewDecoder(resp.Body)
	if err := dec.Decode(&response); err != nil {
		return 0.0, results, fmt.Errorf("Failed to decode XML: %v", err)
	}

	results = make([]PlugState, 8)
	results[0] = PlugState{Enabled: response.PState1, Power: response.Pow1}
	results[1] = PlugState{Enabled: response.PState2, Power: response.Pow2}
	results[2] = PlugState{Enabled: response.PState3, Power: response.Pow3}
	results[3] = PlugState{Enabled: response.PState4, Power: response.Pow4}
	results[4] = PlugState{Enabled: response.PState5, Power: response.Pow5}
	results[5] = PlugState{Enabled: response.PState6, Power: response.Pow6}
	results[6] = PlugState{Enabled: response.PState7, Power: response.Pow7}
	results[7] = PlugState{Enabled: response.PState8, Power: response.Pow8}
	return response.PowT, results, nil
}

func (echola *Echola) ActuatePlug(plugIndex int, on bool) error {
	if plugIndex < 1 || plugIndex > 8 {
		return errors.New("Plug index must be between 1 and 8")
	}

	var state int
	if on {
		state = 1
	} else {
		state = 0
	}
	dest := fmt.Sprintf("http://%s/switch.cgi?out%d=%d", echola.ipAddr, plugIndex, state)
	resp, _, errs := echola.req.Get(dest).End()
	if errs != nil {
		return fmt.Errorf("Failed to actuate plug: %s", errs)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("Failed to actuate plug: %s", resp.Status)
	}
	return nil
}
