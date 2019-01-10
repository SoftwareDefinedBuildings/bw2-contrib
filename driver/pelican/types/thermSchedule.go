// time: convert to iso 8601 standard (use pelican built in time zone value)
// replace middlemen structs with json unmarshaling into free form struct + hard coded key

package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/parnurzeal/gorequest"
)

// Login, Authentication, Thermostat ID Retrieval Structs
type ThermostatRequest struct {
	Request      string
	Resources    []ThermostatResources
	ResourceType string
}

type ThermostatResources struct {
	Children     []ThermostatChild
	GroupId      string
	Id           string
	Permissions  string
	Schedule     string
	ScheduleType string
	Title        string
}

type ThermostatChild struct {
	Id           string
	Permissions  string
	Schedule     string
	ScheduleType string
	Title        string
}

// Thermostat Settings Structs
type ThermostatScheduleByIDRequest struct {
	Epnum    float64
	Id       string
	Nodename string
	Request  string
	Status   int64
	Userdata ThermostatScheduleByID
}

type ThermostatScheduleByID struct {
	AllowDisableKeypad bool
	Epnum              int64
	Fan                string
	Keypad             string
	MultipleFan        bool
	MultipleSystem     bool
	Nodename           string
	Repeat             string
	RepeatDisplay      string
	Reply              string
	State              string
	System             string
	Status             int64
}

// Thermostat Schedule By Day Decoding Structs
type ThermostatScheduleRequest struct {
	ClientData ThermostatScheduleSetTimes `msgpack:"clientdata"`
}

type ThermostatScheduleSetTimes struct {
	SetTimes []ThermostatScheduleSetTimesBlock `msgpack:"setTimes"`
}

type ThermostatScheduleSetTimesBlock struct {
	Label         string `msgpack:"label"`
	EntryIndex    int64  `msgpack:"entryIndex"`
	StartValue    string `msgpack:"startValue"`
	SystemDisplay string `msgpack:"systemDisplay"`
	HeatSetting   int64  `msgpack:"heatSetting"`
	CoolSetting   int64  `msgpack:"coolSetting"`
	FanDisplay    string `msgpack:"fanDisplay"`
}

// Thermostat Schedule By Day Result Structs
type ThermostatSchedule struct {
	DaySchedules map[string]ThermostatDaySchedule
}

type ThermostatDaySchedule struct {
	Blocks []ThermostatBlockSchedule
}

type ThermostatBlockSchedule struct {
	Label         string
	StartTime     string
	HeatSetting   int64
	CoolSetting   int64
	SystemDisplay string
}

// Note: sitename parameter = 410soda
func (pel *Pelican) GetSchedule(sitename string) (map[string]ThermostatSchedule, error) {
	// Part 1: Retrieve Login Authentication Cookies
	loginInfo := map[string]interface{}{
		"username": pel.username,
		"password": pel.password,
		"sitename": sitename,
	}
	respLogin, _, errsLogin := gorequest.New().Post("https://410soda.officeclimatecontrol.net/#_loginPage").Type("form").Send(loginInfo).End()
	if (errsLogin != nil) || (respLogin.StatusCode != 200) {
		return nil, fmt.Errorf("Error logging into pelican website: %v", errsLogin)
	}
	cookies := (*http.Response)(respLogin).Cookies()
	cookie := cookies[0]

	// Part 2: Retrieve Thermostat IDs within this site
	respTherms, _, errsTherms := gorequest.New().Get("https://410soda.officeclimatecontrol.net/ajaxSchedule.cgi?request=getResourcesExtended&resourceType=Thermostats").Type("form").AddCookie(cookie).End()
	if (errsTherms != nil) || (respTherms.StatusCode != 200) {
		return nil, fmt.Errorf("Error retrieving Thermostat IDs (AJAX Request): %v", errsTherms)
	}
	var result ThermostatRequest
	decoder := json.NewDecoder(respTherms.Body)
	if decodeError := decoder.Decode(&result); decodeError != nil {
		return nil, fmt.Errorf("Failed to decode thermostat ID response JSON: %v\n", decodeError)
	}
	thermostatIDs := result.Resources[0].Children

	var returnValue map[string]ThermostatSchedule
	for _, child := range thermostatIDs {
		var thermostatSchedule ThermostatSchedule

		// Determine Repeat Cycle (Weekly, Daily, Weekday/Weekend)
		repeatType, repeatTypeErr := getThermostatRepeatType(child.Id, cookie)
		if repeatTypeErr != nil {
			return nil, fmt.Errorf("Failed to determine repeat type for thermostat %v: %v", child.Id, repeatTypeErr)
		}
		repeatType = strings.TrimRight(repeatType, "\n")

		// Build Schedule Based on Repeat Cycle Type
		if repeatType == "Daily" {
			schedule, scheduleError := getThermostatScheduleByDay(child.Id, cookie, "0")
			if scheduleError != nil {
				return nil, fmt.Errorf("Error retrieving schedule for thermostat %v: %v", child.Id, scheduleError)
			}
			for _, day := range []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"} {
				thermostatSchedule.DaySchedules[day] = schedule
			}
		} else if repeatType == "Weekly" {
			for index, day := range []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"} {
				schedule, scheduleError := getThermostatScheduleByDay(child.Id, cookie, strconv.Itoa(index))
				if scheduleError != nil {
					return nil, fmt.Errorf("Error retrieving schedule for thermostat %v on %v (day %v): %v", child.Id, day, index, scheduleError)
				}
				thermostatSchedule.DaySchedules[day] = schedule
			}
		} else if repeatType == "Weekday/Weekend" {
			weekend, weekendError := getThermostatScheduleByDay(child.Id, cookie, "0")
			if weekendError != nil {
				return nil, fmt.Errorf("Error retrieving schedule for thermostat %v on weekend (day 0): %v", child.Id, weekendError)
			}
			for _, day := range []string{"Sunday", "Saturday"} {
				thermostatSchedule.DaySchedules[day] = weekend
			}
			weekday, weekdayError := getThermostatScheduleByDay(child.Id, cookie, "1")
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

func getThermostatRepeatType(thermostatID string, cookie *http.Cookie) (string, error) {
	var requestURL bytes.Buffer
	requestURL.WriteString("https://410soda.officeclimatecontrol.net/ajaxThermostat.cgi?id=")
	requestURL.WriteString(thermostatID)
	requestURL.WriteString(":Thermostat&request=GetSchedule")

	resp, _, errs := gorequest.New().Get(requestURL.String()).Type("form").AddCookie(cookie).End()
	if errs != nil {
		return "", fmt.Errorf("Failed to retrieve schedule settings (AJAX Request) for thermostat %v: %v", thermostatID, errs)
	}
	var result ThermostatScheduleByIDRequest
	decoder := json.NewDecoder(resp.Body)
	if decodeError := decoder.Decode(&result); decodeError != nil {
		return "", fmt.Errorf("Failed to decode schedule settings (AJAX Request) for thermostat %v: %v", thermostatID, decodeError)
	}
	return result.Userdata.RepeatDisplay, nil
}

func getThermostatScheduleByDay(thermostatID string, cookie *http.Cookie, dayOfWeek string) (ThermostatDaySchedule, error) {
	// Construct Request URL for Thermostat Schedule by Day of Week
	var requestURL bytes.Buffer
	requestURL.WriteString("https://410soda.officeclimatecontrol.net/thermDayEdit.cgi?section=json&nodename=")
	requestURL.WriteString(thermostatID)
	requestURL.WriteString("&epnum=1&dayofweek=")
	requestURL.WriteString(dayOfWeek)

	// Make Request, Decode into Response Struct
	var daySchedule ThermostatDaySchedule
	resp, _, errs := gorequest.New().Get(requestURL.String()).Type("form").AddCookie(cookie).End()
	if errs != nil {
		return daySchedule, fmt.Errorf("Failed to retrieve schedule for thermostat %v on day of week %v: %v", thermostatID, dayOfWeek, errs)
	}
	var result ThermostatScheduleRequest
	decoder := json.NewDecoder(resp.Body)
	if decodeError := decoder.Decode(&result); decodeError != nil {
		return daySchedule, fmt.Errorf("Failed to decode schedule for thermostat %v on day of week %v: %v", thermostatID, dayOfWeek, decodeError)
	}

	// Transfer Response Struct Data into return struct
	for _, block := range result.ClientData.SetTimes {
		var returnBlock ThermostatBlockSchedule
		returnBlock.Label = block.Label
		returnBlock.CoolSetting = block.CoolSetting
		returnBlock.HeatSetting = block.HeatSetting
		returnBlock.StartTime = block.StartValue
		returnBlock.SystemDisplay = block.SystemDisplay
		daySchedule.Blocks = append(daySchedule.Blocks, returnBlock)
	}
	return daySchedule, nil
}
