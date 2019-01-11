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
type repeatTypeRequest struct {
	Epnum    float64           `json:"epnum"`
	Id       string            `json:"id"`
	Nodename string            `json:"nodename"` // TODO(john-b-yang) Use nodename instead of id
	Userdata repeatTypeWrapper `json:"userdata"`
}

type repeatTypeWrapper struct {
	Epnum    int64  `json:"epnum"`
	Fan      string `json:"fan"`
	Nodename string `json:"nodename"`
	Repeat   string `json:"repeat"`
}

// Thermostat Schedule By Day Decoding Structs
type scheduleRequest struct {
	ClientData scheduleSetTimes `json:"clientdata"`
}

type scheduleSetTimes struct {
	SetTimes []scheduleTimeBlock `json:"setTimes"`
}

type scheduleTimeBlock struct {
	StartValue    string `json:"startValue"`
	SystemDisplay string `json:"systemDisplay"`
	HeatSetting   int64  `json:"heatSetting"`
	CoolSetting   int64  `json:"coolSetting"`
}

// Thermostat Schedule By Day Result Structs
type ThermostatSchedule struct {
	DaySchedules map[string]ThermostatDaySchedule
}

type ThermostatDaySchedule struct {
	Blocks []ThermostatBlockSchedule
}

type ThermostatBlockSchedule struct {
	CoolSetting int64
	HeatSetting int64
	System      string
	Time        string
}

func (pel *Pelican) GetSchedule(sitename string) (map[string]ThermostatSchedule, error) {
	// Part 1: Retrieve Login Authentication Cookies
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

	// Part 2: Retrieve Thermostat IDs within this site
	respTherms, _, errsTherms := gorequest.New().Get(fmt.Sprintf("https://%s.officeclimatecontrol.net/ajaxSchedule.cgi?request=getResourcesExtended&resourceType=Thermostats", sitename)).Type("form").AddCookie(cookie).End()
	if (errsTherms != nil) || (respTherms.StatusCode != 200) {
		return nil, fmt.Errorf("Error retrieving Thermostat IDs: %v", errsTherms)
	}
	var result thermIDRequest
	decoder := json.NewDecoder(respTherms.Body)
	// fmt.Printf("Thermostat Request URL: %v\n", respTherms.Request.URL)
	// fmt.Printf("Thermostat Request: %v\n\n", respTherms.Body)
	if decodeError := decoder.Decode(&result); decodeError != nil {
		return nil, fmt.Errorf("Failed to decode thermostat ID response JSON: %v\n", decodeError)
	}
	thermostatIDs := result.Resources[0].Children
	// fmt.Printf("Thermostat IDs: %v\n\n", thermostatIDs)

	returnValue := make(map[string]ThermostatSchedule, len(thermostatIDs))
	for _, child := range thermostatIDs {
		thermostatSchedule := ThermostatSchedule{}
		thermostatSchedule.DaySchedules = make(map[string]ThermostatDaySchedule, 7)

		repeatType, repeatTypeErr := getThermostatRepeatType(sitename, child.Id, cookie)
		if repeatTypeErr != nil {
			return nil, fmt.Errorf("Failed to determine repeat type for thermostat %v: %v", child.Id, repeatTypeErr)
		}
		repeatType = strings.TrimRight(repeatType, "\n")
		child.Id = strings.Split(child.Id, ":")[0] // TODO(john-b-yang) a bit hacky

		// Build Schedule Based on Repeat Cycle Type
		if repeatType == "Daily" {
			schedule, scheduleError := getThermostatScheduleByDay("0", sitename, child.Id, cookie, pel.timezone)
			if scheduleError != nil {
				return nil, fmt.Errorf("Error retrieving schedule for thermostat %v: %v", child.Id, scheduleError)
			}
			for _, day := range []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"} {
				thermostatSchedule.DaySchedules[day] = schedule
			}
		} else if repeatType == "Weekly" {
			for index, day := range []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"} {
				schedule, scheduleError := getThermostatScheduleByDay(strconv.Itoa(index), sitename, child.Id, cookie, pel.timezone)
				if scheduleError != nil {
					return nil, fmt.Errorf("Error retrieving schedule for thermostat %v on %v (day %v): %v", child.Id, day, index, scheduleError)
				}
				thermostatSchedule.DaySchedules[day] = schedule
			}
		} else if repeatType == "Weekday/Weekend" {
			weekend, weekendError := getThermostatScheduleByDay("0", sitename, child.Id, cookie, pel.timezone)
			if weekendError != nil {
				return nil, fmt.Errorf("Error retrieving schedule for thermostat %v on weekend (day 0): %v", child.Id, weekendError)
			}
			for _, day := range []string{"Sunday", "Saturday"} {
				thermostatSchedule.DaySchedules[day] = weekend
			}
			weekday, weekdayError := getThermostatScheduleByDay("1", sitename, child.Id, cookie, pel.timezone)
			if weekdayError != nil {
				return nil, fmt.Errorf("Error retrieving schedule for thermostat %v on weekday (day 1): %v", child.Id, weekdayError)
			}
			for _, day := range []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday"} {
				thermostatSchedule.DaySchedules[day] = weekday
			}
		} else {
			return nil, fmt.Errorf("Failed to recognize repeat type of thermostat %v's schedule: %v", child.Id, repeatType)
		}

		returnValue[child.Id] = thermostatSchedule
	}
	return returnValue, nil
}

func getThermostatRepeatType(sitename, thermostatID string, cookie *http.Cookie) (string, error) {
	var requestURL bytes.Buffer
	requestURL.WriteString(fmt.Sprintf("https://%s.officeclimatecontrol.net/ajaxThermostat.cgi?id=", sitename))
	requestURL.WriteString(thermostatID)
	requestURL.WriteString(":Thermostat&request=GetSchedule")

	resp, _, errs := gorequest.New().Get(requestURL.String()).Type("form").AddCookie(cookie).End()
	if errs != nil {
		return "", fmt.Errorf("Failed to retrieve schedule settings (AJAX Request) for thermostat %v: %v", thermostatID, errs)
	}
	var result repeatTypeRequest
	decoder := json.NewDecoder(resp.Body)
	if decodeError := decoder.Decode(&result); decodeError != nil {
		return "", fmt.Errorf("Failed to decode schedule settings (AJAX Request) for thermostat %v: %v", thermostatID, decodeError)
	}
	return result.Userdata.Repeat, nil
}

func getThermostatScheduleByDay(dayOfWeek, sitename, thermostatID string, cookie *http.Cookie, timezone *time.Location) (ThermostatDaySchedule, error) {
	// Construct Request URL for Thermostat Schedule by Day of Week
	var requestURL bytes.Buffer
	requestURL.WriteString(fmt.Sprintf("https://%s.officeclimatecontrol.net/thermDayEdit.cgi?section=json&nodename=", sitename))
	requestURL.WriteString(thermostatID)
	requestURL.WriteString("&epnum=1&dayofweek=")
	requestURL.WriteString(dayOfWeek)

	// Make Request, Decode into Response Struct
	var daySchedule ThermostatDaySchedule
	resp, _, errs := gorequest.New().Get(requestURL.String()).Type("form").AddCookie(cookie).End()
	if errs != nil {
		return daySchedule, fmt.Errorf("Failed to retrieve schedule for thermostat %v on day of week %v: %v", thermostatID, dayOfWeek, errs)
	}
	var result scheduleRequest
	// fmt.Println(requestURL.String())
	// fmt.Printf("Get Schedule By Day Response: %v\n", resp.Body)
	decoder := json.NewDecoder(resp.Body)
	if decodeError := decoder.Decode(&result); decodeError != nil {
		return daySchedule, fmt.Errorf("Failed to decode schedule for thermostat %v on day of week %v: %v", thermostatID, dayOfWeek, decodeError)
	}

	// Transfer Response Struct Data into return struct
	for _, block := range result.ClientData.SetTimes {
		var returnBlock ThermostatBlockSchedule
		returnBlock.CoolSetting = block.CoolSetting
		returnBlock.HeatSetting = block.HeatSetting
		returnBlock.System = block.SystemDisplay

		if rruleTime, rruleError := convertTimeToRRule(block.StartValue, timezone); rruleError != nil {
			return daySchedule, fmt.Errorf("Failed to convert time in string format %v to rrule format: %v", block.StartValue, rruleError)
		} else {
			returnBlock.Time = rruleTime
		}

		daySchedule.Blocks = append(daySchedule.Blocks, returnBlock)
	}
	// fmt.Printf("Schedule for %v - %v\n\n", dayOfWeek, daySchedule)
	return daySchedule, nil
}

func convertTimeToRRule(blockTime string, timezone *time.Location) (string, error) {
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
		Dtstart: time.Date(0, 0, 0, hour, minute, 0, 0, timezone),
	})

	return rruleSched.String(), nil
}
