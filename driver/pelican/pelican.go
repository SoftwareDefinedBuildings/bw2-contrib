package main

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/parnurzeal/gorequest"
)

var modeNameMappings = map[string]int32{
	"Off":  0,
	"Heat": 1,
	"Cool": 2,
	"Auto": 3,
}
var modeValMappings = []string{"Off", "Heat", "Cool", "Auto"}

var stateMappings = map[string]int32{
	"Off":         0,
	"Heat-Stage1": 1,
	"Heat-Stage2": 1,
	"Cool-Stage1": 2,
	"Cool-Stage2": 2,
}

// TODO Support case where the thermostat is configured to use Celsius

type Pelican struct {
	username string
	password string
	name     string
	target   string
	req      *gorequest.SuperAgent
}

type PelicanStatus struct {
	Temperature     float64 `msgpack:"temperature"`
	RelHumidity     float64 `msgpack:"relative_humidity"`
	HeatingSetpoint float64 `msgpack:"heating_setpoint"`
	CoolingSetpoint float64 `msgpack:"cooling_setpoint"`
	Override        bool    `msgpack:"override"`
	Fan             bool    `msgpack:"fan"`
	Mode            int32   `msgpack:"mode"`
	State           int32   `msgpack:"state"`
	Time            int64   `msgpack:"time"`
}

type apiResult struct {
	Thermostat apiThermostat `xml:"Thermostat"`
	Success    int32         `xml:"success"`
	Message    string        `xml:"message"`
}

type apiThermostat struct {
	Temperature     float64 `xml:"temperature"`
	RelHumidity     int32   `xml:"humidity"`
	HeatingSetpoint int32   `xml:"heatSetting"`
	CoolingSetpoint int32   `xml:"coolSetting"`
	SetBy           string  `xml:"setBy"`
	HeatNeedsFan    string  `xml:"HeatNeedsFan"`
	System          string  `xml:"system"`
	RunStatus       string  `xml:"runStatus"`
}

type pelicanStateParams struct {
	HeatingSetpoint float64
	CoolingSetpoint float64
	Override        bool
	Mode            int32
	Fan             bool
}

func NewPelican(username, password, sitename, name string) *Pelican {
	return &Pelican{
		username: username,
		password: password,
		target:   fmt.Sprintf("https://%s.officeclimatecontrol.net/api.cgi", sitename),
		name:     name,
		req:      gorequest.New(),
	}
}

func (pel *Pelican) GetStatus() (*PelicanStatus, error) {
	resp, _, errs := pel.req.Get(pel.target).
		Param("username", pel.username).
		Param("password", pel.password).
		Param("request", "get").
		Param("object", "Thermostat").
		Param("selection", fmt.Sprintf("name:%s;", pel.name)).
		Param("value", "temperature;humidity;heatSetting;coolSetting;setBy;HeatNeedsFan;system;runStatus").
		End()
	if errs != nil {
		return nil, fmt.Errorf("Error retrieving thermostat status: %v", errs)
	}

	defer resp.Body.Close()
	var result apiResult
	dec := xml.NewDecoder(resp.Body)
	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("Failed to decode response XML: %v", err)
	}
	if result.Success == 0 {
		return nil, fmt.Errorf("Error retrieving thermostat status: %s", result.Message)
	}

	thermostat := result.Thermostat
	var fanState bool
	if strings.HasPrefix(thermostat.RunStatus, "Heat") {
		fanState = thermostat.HeatNeedsFan == "Yes"
	} else if thermostat.RunStatus != "Off" {
		fanState = true
	} else {
		fanState = false
	}
	thermState, ok := stateMappings[thermostat.RunStatus]
	if !ok {
		// Thermostat is not calling for heating or cooling
		if thermostat.System == "Off" {
			thermState = 0 // Off
		} else {
			thermState = 3 // Auto
		}
	}

	return &PelicanStatus{
		Temperature:     thermostat.Temperature,
		RelHumidity:     float64(thermostat.RelHumidity),
		HeatingSetpoint: float64(thermostat.HeatingSetpoint),
		CoolingSetpoint: float64(thermostat.CoolingSetpoint),
		Override:        thermostat.SetBy != "Schedule",
		Fan:             fanState,
		Mode:            modeNameMappings[thermostat.System],
		State:           thermState,
		Time:            time.Now().UnixNano(),
	}, nil
}

func (pel *Pelican) ModifySetpoints(heatingSetpoint, coolingSetpoint float64) error {
	resp, _, errs := pel.req.Get(pel.target).
		Param("username", pel.username).
		Param("password", pel.password).
		Param("request", "set").
		Param("object", "thermostat").
		Param("selection", fmt.Sprintf("name:%s;", pel.name)).
		Param("value", fmt.Sprintf("heatSetting:%d;coolSetting:%d;", int(heatingSetpoint), int(coolingSetpoint))).
		End()
	if errs != nil {
		return fmt.Errorf("Error modifying thermostat temp settings: %v", errs)
	}

	defer resp.Body.Close()
	var result apiResult
	dec := xml.NewDecoder(resp.Body)
	if err := dec.Decode(&result); err != nil {
		return fmt.Errorf("Failed to decode response XML: %v", err)
	}
	if result.Success == 0 {
		return fmt.Errorf("Error modifying thermostat temp settings: %v", result.Message)
	}

	return nil
}

func (pel *Pelican) ModifyState(params *pelicanStateParams) error {
	if params.Mode < 0 || params.Mode > 3 {
		return fmt.Errorf("Specified thermostat mode %d is invalid", params.Mode)
	}

	var scheduleVal string
	if params.Override {
		scheduleVal = "Off"
	} else {
		scheduleVal = "On"
	}
	systemVal := modeValMappings[params.Mode]
	var fanVal string
	if params.Fan {
		fanVal = "On"
	} else {
		fanVal = "Auto"
	}

	resp, _, errs := pel.req.Get(pel.target).
		Param("username", pel.username).
		Param("password", pel.password).
		Param("request", "set").
		Param("object", "thermostat").
		Param("selection", fmt.Sprintf("name:%s;", pel.name)).
		Param("value", fmt.Sprintf("heatSetting:%d;coolSetting:%d;schedule:%s;system:%s;fan:%s;",
			int(params.HeatingSetpoint), int(params.CoolingSetpoint), scheduleVal, systemVal, fanVal)).
		End()
	if errs != nil {
		return fmt.Errorf("Error modifying thermostat state: %v", errs)
	}

	defer resp.Body.Close()
	var result apiResult
	dec := xml.NewDecoder(resp.Body)
	if err := dec.Decode(&result); err != nil {
		return fmt.Errorf("Failed to decode response XML: %v", err)
	}
	if result.Success == 0 {
		return fmt.Errorf("Error modifying thermostat state: %s", result.Message)
	}

	return nil
}
