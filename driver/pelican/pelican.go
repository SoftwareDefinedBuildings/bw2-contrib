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
	location string
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
	Time            string  `msgpack:"time"`
}

// Thermostat Object API Result Structs

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

// Thermostat History Object API Result Structs

type apiResultHistory struct {
	XMLName xml.Name   `xml:"result"`
	Success int        `xml:"success"`
	Message string     `xml:"message"`
	Records apiRecords `xml:"ThermostatHistory"`
}

type apiRecords struct {
	Name    string       `xml:"name"`
	History []apiHistory `xml:"History"`
}

type apiHistory struct {
	TimeStamp string `xml:"timestamp"`
}

// Miscellaneous Structs

type pelicanStateParams struct {
	HeatingSetpoint *float64
	CoolingSetpoint *float64
	Override        *float64
	Mode            *float64
	Fan             *float64
}

type thermostatInfo struct {
	Name        string `xml:"name"`
	Description string `xml:"description"`
}

type discoverApiResult struct {
	Thermostats []thermostatInfo `xml:"Thermostat"`
	Success     int32            `xml:"success"`
	Message     string           `xml:"message"`
}

func NewPelican(username, password, sitename, name, location string) *Pelican {
	return &Pelican{
		username: username,
		password: password,
		target:   fmt.Sprintf("https://%s.officeclimatecontrol.net/api.cgi", sitename),
		name:     name,
		req:      gorequest.New(),
		location: location,
	}
}

func DiscoverPelicans(username, password, sitename, location string) ([]*Pelican, error) {
	target := fmt.Sprintf("https://%s.officeclimatecontrol.net/api.cgi", sitename)
	resp, _, errs := gorequest.New().Get(target).
		Param("username", username).
		Param("password", password).
		Param("request", "get").
		Param("object", "Thermostat").
		Param("value", "name;description").
		End()
	if errs != nil {
		return nil, fmt.Errorf("Error retrieving thermostat name from %s: %s", target, errs)
	}

	defer resp.Body.Close()
	var result discoverApiResult
	dec := xml.NewDecoder(resp.Body)
	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("Failed to decode response XML: %v", err)
	}
	if result.Success == 0 {
		return nil, fmt.Errorf("Error retrieving thermostat status from %s: %s", resp.Request.URL, result.Message)
	}

	var pelicans []*Pelican
	for _, thermInfo := range result.Thermostats {
		if thermInfo.Name != "" {
			pelicans = append(pelicans, NewPelican(username, password, sitename, thermInfo.Name, location))
		}
	}
	return pelicans, nil
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
		return nil, fmt.Errorf("Error retrieving thermostat status from %s: %v", pel.target, errs)
	}

	defer resp.Body.Close()
	var result apiResult
	dec := xml.NewDecoder(resp.Body)
	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("Failed to decode response XML: %v", err)
	}
	if result.Success == 0 {
		return nil, fmt.Errorf("Error retrieving thermostat status from %s: %s", resp.Request.URL, result.Message)
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
			// Thermostat is not heating or cooling, but fan is still running
			// Report this as off
			thermState = 0 //Off
		}
	}

	// Thermostat History Object Request to retrieve time stamp
	// Retrieves all time stamps from the past 4 hours
	location, locErr := time.LoadLocation(pel.location)
	if locErr != nil {
		return nil, fmt.Errorf("Location definition error in get request: %v\n", locErr)
	}
	diff := time.Now().Add(-4 * time.Hour)
	endTime := time.Now().In(location).Format(time.RFC3339)
	startTime := diff.In(location).Format(time.RFC3339)

	respHist, _, errsHist := pel.req.Get(pel.target).
		Param("username", pel.username).
		Param("password", pel.password).
		Param("request", "get").
		Param("object", "ThermostatHistory").
		Param("selection", fmt.Sprintf("startDateTime:%s;endDateTime:%s;", startTime, endTime)).
		Param("value", "timestamp").
		End()
	defer respHist.Body.Close()

	if errsHist != nil {
		return nil, fmt.Errorf("Error retrieving thermostat status from %s: %v", pel.target, errsHist)
	}

	var histResult apiResultHistory
	histDec := xml.NewDecoder(respHist.Body)
	if histErr := histDec.Decode(&histResult); histErr != nil {
		return nil, fmt.Errorf("Failed to decode response XML: %v", histErr)
	}
	if histResult.Success == 0 {
		return nil, fmt.Errorf("Error retrieving thermostat status from %s: %s", respHist.Request.URL, histResult.Message)
	}
	if len(histResult.Records.History) <= 0 {
		return nil, fmt.Errorf("Error: No timestamp records found in past %v from %s", diff, pel.target)
	}
	match := histResult.Records.History[len(histResult.Records.History)-1]

	return &PelicanStatus{
		Temperature:     thermostat.Temperature,
		RelHumidity:     float64(thermostat.RelHumidity),
		HeatingSetpoint: float64(thermostat.HeatingSetpoint),
		CoolingSetpoint: float64(thermostat.CoolingSetpoint),
		Override:        thermostat.SetBy != "Schedule",
		Fan:             fanState,
		Mode:            modeNameMappings[thermostat.System],
		State:           thermState,
		Time:            match.TimeStamp,
	}, nil
}

func (pel *Pelican) ModifySetpoints(setpoints *setpointsMsg) error {
	var value string
	// heating setpoint
	if setpoints.HeatingSetpoint != nil {
		value += fmt.Sprintf("heatSetting:%d;", int(*setpoints.HeatingSetpoint))
	}
	// cooling setpoint
	if setpoints.CoolingSetpoint != nil {
		value += fmt.Sprintf("coolSetting:%d;", int(*setpoints.CoolingSetpoint))
	}
	resp, _, errs := pel.req.Get(pel.target).
		Param("username", pel.username).
		Param("password", pel.password).
		Param("request", "set").
		Param("object", "thermostat").
		Param("selection", fmt.Sprintf("name:%s;", pel.name)).
		Param("value", value).
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
	var value string

	// mode
	if params.Mode != nil {
		mode := int(*params.Mode)
		if mode < 0 || mode > 3 {
			return fmt.Errorf("Specified thermostat mode %d is invalid", mode)
		}
		systemVal := modeValMappings[mode]
		value += fmt.Sprintf("system:%s;", systemVal)
	}

	// override
	if params.Override != nil {
		var scheduleVal string
		if *params.Override == 1 {
			scheduleVal = "Off"
		} else {
			scheduleVal = "On"
		}
		value += fmt.Sprintf("schedule:%s;", scheduleVal)
	}

	// fan
	if params.Fan != nil {
		var fanVal string
		if *params.Fan == 1 {
			fanVal = "On"
		} else {
			fanVal = "Auto"
		}
		value += fmt.Sprintf("fan:%s;", fanVal)
	}

	// heating setpoint
	if params.HeatingSetpoint != nil {
		value += fmt.Sprintf("heatSetting:%d;", int(*params.HeatingSetpoint))
	}
	// cooling setpoint
	if params.CoolingSetpoint != nil {
		value += fmt.Sprintf("coolSetting:%d;", int(*params.CoolingSetpoint))
	}

	resp, _, errs := pel.req.Get(pel.target).
		Param("username", pel.username).
		Param("password", pel.password).
		Param("request", "set").
		Param("object", "thermostat").
		Param("selection", fmt.Sprintf("name:%s;", pel.name)).
		Param("value", value).
		End()
	if errs != nil {
		return fmt.Errorf("Error modifying thermostat state: %v (%s)", errs, resp.Request.URL)
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
