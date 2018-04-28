package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

type ADREventWrapperAPI struct {
	XMLName xml.Name    `xml:"result"`
	Success int         `xml:"success"`
	Message string      `xml:"message"`
	Event   ADREventAPI `xml:"attribute"`
}

type ADREventAPI struct {
	End    string `xml:"OpenADREventEnd"`
	Start  string `xml:"OpenADREventStart"`
	Status string `xml:"OpenADRStatus"`
	Type   string `xml:"OpenADREventType"`
}

type ADREvent struct {
	Event_End   int64
	Event_Start int64
	Event_Type  int64
	DR_Status   int64
	Time        int64
}

func (pel *Pelican) TrackDREvent() (*ADREvent, error) {
	resp, _, errs := pel.req.Get(pel.target).
		Param("username", pel.username).
		Param("password", pel.password).
		Param("request", "get").
		Param("object", "Site").
		Param("value", "OpenADREventEnd;OpenADREventStart;OpenADRStatus;OpenADREventType").
		End()

	if errs != nil {
		return nil, fmt.Errorf("Error retrieving thermostat status from %s: %v", pel.target, errs)
	}

	defer resp.Body.Close()
	var result ADREventWrapperAPI
	dec := xml.NewDecoder(resp.Body)
	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("Failed to decode response XML: %v", err)
	}
	if result.Success == 0 {
		return nil, fmt.Errorf("Error retrieving thermostat status from %s: %s", resp.Request.URL, result.Message)
	}

	event := &result.Event
	var output ADREvent

	// Convert ADR Start Time from String to Int
	if startTime, startErr := DRTimeToUnix(event.Start, pel.timezone); startErr != nil {
		return nil, fmt.Errorf("String to Unix Time Conversion Error: %v", startErr)
	} else {
		output.Event_Start = startTime
	}

	// Convert ADR End Time from String to Int
	if endTime, endErr := DRTimeToUnix(event.End, pel.timezone); endErr != nil {
		return nil, fmt.Errorf("String to Unix Time Conversion Error: %v", endErr)
	} else {
		output.Event_Start = endTime
	}

	// Map Status to Corresponding Integer Value
	statusMode := event.Status
	if statusMode == "Not Configured" {
		output.DR_Status = 0
	} else if statusMode == "Unusable" {
		output.DR_Status = 1
	} else if statusMode == "Inactive" {
		output.DR_Status = 2
	} else if statusMode == "Active" {
		output.DR_Status = 3
	} else {
		// Unrecognized Status
		output.DR_Status = -1
	}

	// Map Event Type to Corresponding Integer Value
	typeMode := event.Type
	if typeMode == "Normal" {
		output.Event_Type = 0
	} else if typeMode == "Moderate" {
		output.Event_Type = 1
	} else if typeMode == "High" {
		output.Event_Type = 2
	} else if typeMode == "Special" {
		output.Event_Type = 3
	} else {
		// Unrecognized Event Type (includes 'None')
		output.Event_Type = -1
	}

	output.Time = time.Now().UnixNano()

	return &output, nil
}

func DRTimeToUnix(DRTime string, timezone *time.Location) (int64, error) {
	// Time field is empty or nil
	if DRTime == "" || len(DRTime) == 0 {
		return 0, nil
	}

	// Using Parse in Location to cast time string into correct int64 value
	outputTime, timeErr := time.ParseInLocation("2006-01-02T15:04", DRTime, timezone)
	if timeErr != nil {
		return 0, fmt.Errorf("Error parsing %v into Time struct: %v\n", DRTime, timeErr)
	}

	return outputTime.UnixNano(), nil
}

// Testing function to check whether TrackDREvent function works properly
func testTrackDREvent() {
	bwClient := bw2.ConnectOrExit("")
	bwClient.OverrideAutoChainTo(true)
	bwClient.SetEntityFromEnvironOrExit()

	params := spawnable.GetParamsOrExit()
	baseURI := params.MustString("svc_base_uri")
	if !strings.HasSuffix(baseURI, "/") {
		baseURI += "/"
	}
	username := params.MustString("username")
	password := params.MustString("password")
	sitename := params.MustString("sitename")

	pelicans, err := DiscoverPelicans(username, password, sitename)
	if err != nil {
		fmt.Printf("Failed to discover thermostats: %v\n", err)
		os.Exit(1)
	}

	eventDR, eventErr := pelicans[0].TrackDREvent()
	if eventErr != nil {
		fmt.Printf("Error: %v\n", eventErr)
	}
	fmt.Printf("Event Struct: %+v\n", eventDR)
}
