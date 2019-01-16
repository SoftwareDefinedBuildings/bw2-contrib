package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/parnurzeal/gorequest"
	rrule "github.com/teambition/rrule-go"
)

// Login, Authentication, Thermostat ID Retrieval Structs
type thermIDRequest struct {
	Resources []thermIDResources `json:"resources"`
}

type thermIDResources struct {
	Children    []thermIDChild `json:"children"`
	GroupId     string         `json:"groupId"`
	Permissions string         `json:"permissions"`
}

type thermIDChild struct {
	Id          string `json:"id"`
	Permissions string `json:"permissions"`
}

// Thermostat Settings Structs
type settingsRequest struct {
	Epnum    float64         `json:"epnum"`
	Id       string          `json:"id"`
	Nodename string          `json:"nodename"`
	Userdata settingsWrapper `json:"userdata"`
}

type settingsWrapper struct {
	Epnum    float64 `json:"epnum"`
	Fan      string  `json:"fan"`
	Nodename string  `json:"nodename"`
	Repeat   string  `json:"repeat"`
}

// Thermostat Schedule By Day Decoding Structs
type scheduleRequest struct {
	ClientData scheduleSetTimes `json:"clientdata"`
}

type scheduleSetTimes struct {
	SetTimes []scheduleTimeBlock `json:"setTimes"`
}

type scheduleTimeBlock struct {
	HeatSetting float64 `json:"heatSetting"`
	CoolSetting float64 `json:"coolSetting"`
	StartValue  string  `json:"startValue"`
	System      string  `json:"systemDisplay"`
}

// Thermostat Schedule By Day Result Structs
type ThermostatSchedule struct {
	DaySchedules map[string]ThermostatDaySchedule `msgpack:"day_schedules"`
}

type ThermostatDaySchedule struct {
	Blocks []ThermostatBlockSchedule `msgpack:blocks`
}

type ThermostatBlockSchedule struct {
	CoolSetting float64 `msgpack:"cool_setting"`
	HeatSetting float64 `msgpack:"heat_setting"`
	System      string  `msgpack:"system"`
	Time        string  `msgpack:"time"`
}

var week = [...]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
var weekRRule = [...]rrule.Weekday{rrule.SU, rrule.MO, rrule.TU, rrule.WE, rrule.TH, rrule.FR, rrule.SA}

func (pel *Pelican) GetSchedule(sitename string) (map[string]ThermostatSchedule, error) {
	// Retrieve Login Authentication Cookies
	loginInfo := map[string]interface{}{
		"username": pel.username,
		"password": pel.password,
		"sitename": sitename,
	}
	respLogin, _, errsLogin := gorequest.New().Post(fmt.Sprintf("https://%s.officeclimatecontrol.net/#_loginPage", sitename)).Type("form").Send(loginInfo).End()
	if (errsLogin != nil) || (respLogin.StatusCode != 200) {
		return nil, fmt.Errorf("Error logging into climate control website: %v", errsLogin)
	}
	cookies := (*http.Response)(respLogin).Cookies()
	cookie := cookies[0]

	// Retrieve Thermostat IDs within given sitename
	respTherms, _, errsTherms := gorequest.New().Get(fmt.Sprintf("https://%s.officeclimatecontrol.net/ajaxSchedule.cgi?request=getResourcesExtended&resourceType=Thermostats", sitename)).Type("form").AddCookie(cookie).End()
	if (errsTherms != nil) || (respTherms.StatusCode != 200) {
		return nil, fmt.Errorf("Error retrieving Thermostat IDs: %v", errsTherms)
	}

	var IDRequest thermIDRequest
	decoder := json.NewDecoder(respTherms.Body)
	if decodeError := decoder.Decode(&IDRequest); decodeError != nil {
		return nil, fmt.Errorf("Failed to decode Thermostat ID response JSON: %v\n", decodeError)
	}
	thermostatIDs := IDRequest.Resources[0].Children

	// Construct Weekly Schedules for each Thermostat ID
	schedules := make(map[string]ThermostatSchedule, len(thermostatIDs))
	for _, thermostatID := range thermostatIDs {
		thermSchedule := ThermostatSchedule{
			DaySchedules: make(map[string]ThermostatDaySchedule, len(week)),
		}

		// Retrieve Repeat Type (Daily, Weekly, Weekend/Weekday) and Nodename from Thermostat's Settings
		settings, settingsErr := getSettings(sitename, thermostatID.Id, cookie)
		if settingsErr != nil {
			return nil, fmt.Errorf("Failed to determine repeat type for thermostat %v: %v", thermostatID, settingsErr)
		}
		repeatType := settings.Repeat
		nodename := settings.Nodename
		epnum := settings.Epnum

		// Build Schedule by Repeat Type
		if repeatType == "Daily" {
			schedule, scheduleError := getScheduleByDay(0, epnum, sitename, nodename, cookie, pel.timezone)
			if scheduleError != nil {
				return nil, fmt.Errorf("Error retrieving schedule for thermostat %v: %v", nodename, scheduleError)
			}
			for _, day := range week {
				thermSchedule.DaySchedules[day] = *schedule
			}
		} else if repeatType == "Weekly" {
			for index, day := range week {
				schedule, scheduleError := getScheduleByDay(index, epnum, sitename, nodename, cookie, pel.timezone)
				if scheduleError != nil {
					return nil, fmt.Errorf("Error retrieving schedule for thermostat %v on %v (day %v): %v", nodename, day, index, scheduleError)
				}
				thermSchedule.DaySchedules[day] = *schedule
			}
		} else if repeatType == "Weekday/Weekend" {
			weekend, weekendError := getScheduleByDay(0, epnum, sitename, nodename, cookie, pel.timezone)
			if weekendError != nil {
				return nil, fmt.Errorf("Error retrieving schedule for thermostat %v on weekend (day 0): %v", nodename, weekendError)
			}
			for _, day := range []string{"Sunday", "Saturday"} {
				thermSchedule.DaySchedules[day] = *weekend
			}
			weekday, weekdayError := getScheduleByDay(1, epnum, sitename, nodename, cookie, pel.timezone)
			if weekdayError != nil {
				return nil, fmt.Errorf("Error retrieving schedule for thermostat %v on weekday (day 1): %v", nodename, weekdayError)
			}
			for _, day := range []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday"} {
				thermSchedule.DaySchedules[day] = *weekday
			}
		} else {
			return nil, fmt.Errorf("Failed to recognize repeat type of thermostat %v's schedule: %v", nodename, repeatType)
		}

		schedules[thermostatID.Id] = thermSchedule
	}
	return schedules, nil
}

func getSettings(sitename, thermostatID string, cookie *http.Cookie) (*settingsWrapper, error) {
	var requestURL bytes.Buffer
	requestURL.WriteString(fmt.Sprintf("https://%s.officeclimatecontrol.net/ajaxThermostat.cgi?id=", sitename))
	requestURL.WriteString(thermostatID)
	requestURL.WriteString(":Thermostat&request=GetSchedule")

	resp, _, errs := gorequest.New().Get(requestURL.String()).Type("form").AddCookie(cookie).End()
	if errs != nil {
		return nil, fmt.Errorf("Failed to retrieve schedule settings for thermostat %v: %v", thermostatID, errs)
	}
	var result settingsRequest
	decoder := json.NewDecoder(resp.Body)
	if decodeError := decoder.Decode(&result); decodeError != nil {
		return nil, fmt.Errorf("Failed to decode schedule settings for thermostat %v: %v", thermostatID, decodeError)
	}
	return &result.Userdata, nil
}

func getScheduleByDay(dayOfWeek int, epnum float64, sitename, thermostatID string, cookie *http.Cookie, timezone *time.Location) (*ThermostatDaySchedule, error) {
	// Construct Request URL for Thermostat Schedule by Day of Week
	var requestURL bytes.Buffer
	requestURL.WriteString(fmt.Sprintf("https://%s.officeclimatecontrol.net/thermDayEdit.cgi?section=json&nodename=", sitename))
	requestURL.WriteString(thermostatID)
	requestURL.WriteString("&epnum=")
	requestURL.WriteString(fmt.Sprintf("%.0f", epnum))
	requestURL.WriteString("&dayofweek=")
	requestURL.WriteString(strconv.Itoa(dayOfWeek))

	// Make Request, Decode into Response Struct
	resp, _, errs := gorequest.New().Get(requestURL.String()).Type("form").AddCookie(cookie).End()
	if errs != nil {
		return nil, fmt.Errorf("Failed to retrieve schedule for thermostat %v on day of week %v: %v", thermostatID, dayOfWeek, errs)
	}
	var result scheduleRequest
	decoder := json.NewDecoder(resp.Body)
	if decodeError := decoder.Decode(&result); decodeError != nil {
		return nil, fmt.Errorf("Failed to decode schedule for thermostat %v on day of week %v: %v", thermostatID, dayOfWeek, decodeError)
	}

	// Transfer Response Struct Data into return struct
	var daySchedule ThermostatDaySchedule
	for _, block := range result.ClientData.SetTimes {
		var returnBlock ThermostatBlockSchedule
		returnBlock.CoolSetting = block.CoolSetting
		returnBlock.HeatSetting = block.HeatSetting
		returnBlock.System = block.System

		if rruleTime, rruleError := convertTimeToRRule(dayOfWeek, block.StartValue, timezone); rruleError != nil {
			return nil, fmt.Errorf("Failed to convert time in string format %v to rrule format: %v", block.StartValue, rruleError)
		} else {
			returnBlock.Time = rruleTime
		}

		daySchedule.Blocks = append(daySchedule.Blocks, returnBlock)
	}
	return &daySchedule, nil
}

func convertTimeToRRule(dayOfWeek int, blockTime string, timezone *time.Location) (string, error) {
	timeSlice := strings.Split(blockTime, ":")
	hour, hourErr := strconv.Atoi(timeSlice[0])
	if hourErr != nil {
		return "", fmt.Errorf("Failed to convert hour value of type string to type int: %v", hourErr)
	}
	if timeSlice[2] == "PM" {
		hour += 12
		if hour == 24 {
			hour = 0
		}
	}
	minute, minuteErr := strconv.Atoi(timeSlice[1])
	if minuteErr != nil {
		return "", fmt.Errorf("Failed to convert minute value of type string to type int: %v", minuteErr)
	}

	rruleSched, _ := rrule.NewRRule(rrule.ROption{
		Freq:    rrule.WEEKLY,
		Wkst:    weekRRule[dayOfWeek],
		Dtstart: time.Date(0, 0, 0, hour, minute, 0, 0, timezone),
	})

	return rruleSched.String(), nil
}
