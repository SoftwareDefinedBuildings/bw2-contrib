package main

import (
	"encoding/xml"
	"fmt"
	"time"
)

type DR_EVENT_STATUS int

const (
	DR_EVENT_UNKNOWN        DR_EVENT_STATUS = -1
	DR_EVENT_NOT_CONFIGURED DR_EVENT_STATUS = iota
	DR_EVENT_UNUSABLE
	DR_EVENT_INACTIVE
	DR_EVENT_ACTIVE
)

type DR_EVENT_TYPE int

const (
	DR_EVENT_UNRECOGNIZED DR_EVENT_TYPE = -1
	DR_EVENT_NORMAL       DR_EVENT_TYPE = iota
	DR_EVENT_MODERATE
	DR_EVENT_HIGH
	DR_EVENT_SPECIAL
)

type ADREventWrapperAPI struct {
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
	EventEnd   int64           `msgpack:"event_end"`
	EventStart int64           `msgpack:"event_start"`
	EventType  DR_EVENT_TYPE   `msgpack:"event_type"`
	DRStatus   DR_EVENT_STATUS `msgpack:"dr_status"`
	Time       int64           `msgpack:"time"`
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
		return nil, fmt.Errorf("Error retrieving thermostat demand-response status from %s: %v", pel.target, errs)
	}

	defer resp.Body.Close()
	var result ADREventWrapperAPI
	dec := xml.NewDecoder(resp.Body)
	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("Failed to decode demand-response XML: %v", err)
	}
	if result.Success == 0 {
		return nil, fmt.Errorf("Error retrieving thermostat demand-response status from %s: %s", resp.Request.URL, result.Message)
	}

	event := result.Event
	var output ADREvent

	// Convert ADR Start Time from String to Int
	if startTime, startErr := DRTimeToUnix(event.Start, pel.timezone); startErr != nil {
		return nil, fmt.Errorf("String to Unix Time Conversion Error: %v", startErr)
	} else {
		output.EventStart = startTime
	}

	// Convert ADR End Time from String to Int
	if endTime, endErr := DRTimeToUnix(event.End, pel.timezone); endErr != nil {
		return nil, fmt.Errorf("String to Unix Time Conversion Error: %v", endErr)
	} else {
		output.EventEnd = endTime
	}

	output.DRStatus = GetEventStatus(event.Status)
	output.EventType = GetEventType(event.Type)
	output.Time = time.Now().UnixNano()

	return &output, nil
}

func DRTimeToUnix(DRTime string, timezone *time.Location) (int64, error) {
	// Time field is empty or nil
	if len(DRTime) == 0 {
		return 0, nil
	}

	// Using Parse in Location to convert time string into correct time.Time value
	outputTime, timeErr := time.ParseInLocation("2006-01-02T15:04", DRTime, timezone)
	if timeErr != nil {
		return 0, fmt.Errorf("Error parsing %v into Time struct: %v\n", DRTime, timeErr)
	}

	return outputTime.UnixNano(), nil
}

// Map Status to Corresponding Integer Value
func GetEventStatus(eventStatus string) DR_EVENT_STATUS {
	switch eventStatus {
	case "Not Configured":
		return DR_EVENT_NOT_CONFIGURED
	case "Unusable":
		return DR_EVENT_UNUSABLE
	case "Inactive":
		return DR_EVENT_INACTIVE
	case "Active":
		return DR_EVENT_ACTIVE
	default:
		return DR_EVENT_UNKNOWN
	}
}

// Map Event Type to Corresponding Integer Value
func GetEventType(eventType string) DR_EVENT_TYPE {
	switch eventType {
	case "Normal":
		return DR_EVENT_NORMAL
	case "Moderate":
		return DR_EVENT_MODERATE
	case "High":
		return DR_EVENT_HIGH
	case "Special":
		return DR_EVENT_SPECIAL
	default:
		return DR_EVENT_UNRECOGNIZED
	}
}
