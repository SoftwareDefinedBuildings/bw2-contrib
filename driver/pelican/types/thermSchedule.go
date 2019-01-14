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

// Thermostat Schedule By Day Structs
type ThermostatSchedule struct {
	DaySchedules map[string]ThermostatDaySchedule
}

type ThermostatDaySchedule struct {
	Blocks []ThermostatBlockSchedule
}

type ThermostatBlockSchedule struct {
	CoolSetting float64
	HeatSetting float64
	System      string
	Time        string
}

var week = [...]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

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

	var thermIDRequest map[string]interface{}
	decoder := json.NewDecoder(respTherms.Body)
	if decodeError := decoder.Decode(&thermIDRequest); decodeError != nil {
		return nil, fmt.Errorf("Failed to decode Thermostat ID response JSON: %v\n", decodeError)
	}
	thermIDResources := ((thermIDRequest["resources"].([]interface{}))[0]).(map[string]interface{})
	thermIDChildren := thermIDResources["children"].([]interface{})
	var thermostatIDs []string
	for _, childIFace := range thermIDChildren {
		childID := (childIFace.(map[string]interface{}))["id"]
		thermostatIDs = append(thermostatIDs, childID.(string))
	}

	// Construct Weekly Schedules for each Thermostat ID
	schedules := make(map[string]ThermostatSchedule, len(thermostatIDs))
	for _, thermostatID := range thermostatIDs {
		thermSchedule := ThermostatSchedule{
			DaySchedules: make(map[string]ThermostatDaySchedule, len(week)),
		}

		// Retrieve Repeat Type (Daily, Weekly, Weekend/Weekday) and Nodename from Thermostat's Settings
		settings, settingsErr := getSettings(sitename, thermostatID, cookie)
		if settingsErr != nil {
			return nil, fmt.Errorf("Failed to determine repeat type for thermostat %v: %v", thermostatID, settingsErr)
		}
		repeatType := settings["repeat"].(string)
		nodename := settings["nodename"].(string)
		epnum := settings["epnum"].(float64)

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

		schedules[thermostatID] = thermSchedule
	}
	return schedules, nil
}

func getSettings(sitename, thermostatID string, cookie *http.Cookie) (map[string]interface{}, error) {
	var requestURL bytes.Buffer
	requestURL.WriteString(fmt.Sprintf("https://%s.officeclimatecontrol.net/ajaxThermostat.cgi?id=", sitename))
	requestURL.WriteString(thermostatID)
	requestURL.WriteString(":Thermostat&request=GetSchedule")

	resp, _, errs := gorequest.New().Get(requestURL.String()).Type("form").AddCookie(cookie).End()
	if errs != nil {
		return nil, fmt.Errorf("Failed to retrieve schedule settings for thermostat %v: %v", thermostatID, errs)
	}
	var result map[string]interface{}
	decoder := json.NewDecoder(resp.Body)
	if decodeError := decoder.Decode(&result); decodeError != nil {
		return nil, fmt.Errorf("Failed to decode schedule settings for thermostat %v: %v", thermostatID, decodeError)
	}
	userdata := result["userdata"].(map[string]interface{})
	return userdata, nil
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
	var result map[string]interface{}
	decoder := json.NewDecoder(resp.Body)
	if decodeError := decoder.Decode(&result); decodeError != nil {
		return nil, fmt.Errorf("Failed to decode schedule for thermostat %v on day of week %v: %v", thermostatID, dayOfWeek, decodeError)
	}

	// Transfer Response Struct Data into return struct
	var daySchedule ThermostatDaySchedule
	clientdata := result["clientdata"].(map[string]interface{})
	setTimes := clientdata["setTimes"].([]interface{})
	for _, block := range setTimes {
		castBlock := block.(map[string]interface{})
		var returnBlock ThermostatBlockSchedule
		returnBlock.CoolSetting = castBlock["coolSetting"].(float64)
		returnBlock.HeatSetting = castBlock["heatSetting"].(float64)
		returnBlock.System = castBlock["systemDisplay"].(string)

		blockTime := castBlock["startValue"].(string)
		if rruleTime, rruleError := convertTimeToRRule(blockTime, timezone); rruleError != nil {
			return nil, fmt.Errorf("Failed to convert time in string format %v to rrule format: %v", blockTime, rruleError)
		} else {
			returnBlock.Time = rruleTime
		}

		daySchedule.Blocks = append(daySchedule.Blocks, returnBlock)
	}
	return &daySchedule, nil
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
