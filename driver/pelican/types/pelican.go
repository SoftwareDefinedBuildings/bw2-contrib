package types

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
	username      string
	password      string
	Name          string
	HeatingStages int32
	CoolingStages int32
	target        string
	req           *gorequest.SuperAgent
}

type PelicanStatus struct {
	Temperature       float64 `msgpack:"temperature"`
	RelHumidity       float64 `msgpack:"relative_humidity"`
	HeatingSetpoint   float64 `msgpack:"heating_setpoint"`
	CoolingSetpoint   float64 `msgpack:"cooling_setpoint"`
	Override          bool    `msgpack:"override"`
	Fan               bool    `msgpack:"fan"`
	Mode              int32   `msgpack:"mode"`
	State             int32   `msgpack:"state"`
	EnabledHeatStages int32   `msgpack:"enabled_heat_stages"`
	EnabledCoolStages int32   `msgpack:"enabled_cool_stages"`
	Time              int64   `msgpack:"time"`
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
	HeatStages      int32   `xml:"heatStages"`
	CoolStages      int32   `xml:"coolStages"`
}

type PelicanSetpointParams struct {
	HeatingSetpoint *float64
	CoolingSetpoint *float64
}

type PelicanStateParams struct {
	HeatingSetpoint *float64
	CoolingSetpoint *float64
	Override        *float64
	Mode            *float64
	Fan             *float64
}

type PelicanStageParams struct {
	HeatingStages *int32
	CoolingStages *int32
}

type thermostatInfo struct {
	Name          string `xml:"name"`
	HeatingStages int32  `xml:"heatStages"`
	CoolingStages int32  `xml:"coolStages"`
}

type discoverAPIResult struct {
	Thermostats []thermostatInfo `xml:"Thermostat"`
	Success     int32            `xml:"success"`
	Message     string           `xml:"message"`
}

type NewPelicanParams struct {
	Username      string
	Password      string
	Sitename      string
	Name          string
	HeatingStages int32
	CoolingStages int32
}

func NewPelican(params *NewPelicanParams) *Pelican {
	return &Pelican{
		username:      params.Username,
		password:      params.Password,
		target:        fmt.Sprintf("https://%s.officeclimatecontrol.net/api.cgi", params.Sitename),
		Name:          params.Name,
		HeatingStages: params.HeatingStages,
		CoolingStages: params.CoolingStages,
		req:           gorequest.New(),
	}
}

func DiscoverPelicans(username, password, sitename string) ([]*Pelican, error) {
	target := fmt.Sprintf("https://%s.officeclimatecontrol.net/api.cgi", sitename)
	resp, _, errs := gorequest.New().Get(target).
		Param("username", username).
		Param("password", password).
		Param("request", "get").
		Param("object", "Thermostat").
		Param("value", "name;heatStages;coolStages").
		End()
	if errs != nil {
		return nil, fmt.Errorf("Error retrieving thermostat name from %s: %s", resp.Request.URL, errs)
	}

	defer resp.Body.Close()
	var result discoverAPIResult
	dec := xml.NewDecoder(resp.Body)
	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("Failed to decode response XML: %v", err)
	}
	if result.Success == 0 {
		return nil, fmt.Errorf("Error retrieving thermostat info from %s: %s", resp.Request.URL, result.Message)
	}

	pelicans := make([]*Pelican, len(result.Thermostats))
	for i, thermInfo := range result.Thermostats {
		pelicans[i] = NewPelican(&NewPelicanParams{
			Username:      username,
			Password:      password,
			Sitename:      sitename,
			Name:          thermInfo.Name,
			HeatingStages: thermInfo.HeatingStages,
			CoolingStages: thermInfo.CoolingStages,
		})
	}
	return pelicans, nil
}

func (pel *Pelican) GetStatus() (*PelicanStatus, error) {
	resp, _, errs := pel.req.Get(pel.target).
		Param("username", pel.username).
		Param("password", pel.password).
		Param("request", "get").
		Param("object", "Thermostat").
		Param("selection", fmt.Sprintf("name:%s;", pel.Name)).
		Param("value", "temperature;humidity;heatSetting;coolSetting;setBy;HeatNeedsFan;system;runStatus;heatStages;coolStages").
		End()
	if errs != nil {
		return nil, fmt.Errorf("Error retrieving thermostat status from %s: %v", resp.Request.URL, errs)
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

	return &PelicanStatus{
		Temperature:       thermostat.Temperature,
		RelHumidity:       float64(thermostat.RelHumidity),
		HeatingSetpoint:   float64(thermostat.HeatingSetpoint),
		CoolingSetpoint:   float64(thermostat.CoolingSetpoint),
		Override:          thermostat.SetBy != "Schedule",
		Fan:               fanState,
		Mode:              modeNameMappings[thermostat.System],
		State:             thermState,
		EnabledHeatStages: thermostat.HeatStages,
		EnabledCoolStages: thermostat.CoolStages,
		Time:              time.Now().UnixNano(),
	}, nil
}

func (pel *Pelican) ModifySetpoints(params *PelicanSetpointParams) error {
	var value string
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
		Param("selection", fmt.Sprintf("name:%s;", pel.Name)).
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

func (pel *Pelican) ModifyState(params *PelicanStateParams) error {
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
		Param("selection", fmt.Sprintf("name:%s;", pel.Name)).
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

func (pel *Pelican) ModifyStages(params *PelicanStageParams) error {
	// Turn the thermostat off, saving its previously active mode
	status, err := pel.GetStatus()
	if err != nil {
		return fmt.Errorf("Error retrieving thermostat status: %s", err)
	}
	newMode := float64(0) // Off
	if err := pel.ModifyState(&PelicanStateParams{Mode: &newMode}); err != nil {
		return fmt.Errorf("Failed to turn thermostat off: %s", err)
	}

	// Restore the thermostat to its previous mode
	defer func() {
		oldMode := float64(status.Mode)
		if err := pel.ModifyState(&PelicanStateParams{Mode: &oldMode}); err != nil {
			fmt.Printf("Failed to restore thermostat to old mode: %s\n", err)
		}
	}()

	// Change the thermostat's stage configuration
	var value string
	if params.HeatingStages != nil {
		value += fmt.Sprintf("heatStages:%d;", *params.HeatingStages)
	}
	if params.CoolingStages != nil {
		value += fmt.Sprintf("coolStages:%d;", *params.CoolingStages)
	}

	resp, _, errs := pel.req.Get(pel.target).
		Param("username", pel.username).
		Param("password", pel.password).
		Param("request", "set").
		Param("object", "thermostat").
		Param("selection", fmt.Sprintf("name:%s;", pel.Name)).
		Param("value", value).
		End()
	if errs != nil {
		return fmt.Errorf("Error modifying thermostat stages: %v (%s)", errs, resp.Request.URL)
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
