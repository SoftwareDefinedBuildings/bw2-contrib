package types

import (
	"fmt"

	rrule "github.com/teambition/rrule-go"
)

func (pel *Pelican) SetSchedule(newSchedule *ThermostatSchedule) error {
	// Iterate and create schedules day by day
	for day, blocks := range newSchedule.DaySchedules {
		// Build HTTP Request's value by day
		var value string
		for _, block := range blocks {
			value += fmt.Sprintf("coolSetting:%.0f;", block.CoolSetting)
			value += fmt.Sprintf("heatSetting:%.0f", block.HeatSetting)
			value += fmt.Sprintf("system:%s", block.System)

			time, timeErr := rrule.StrToRRule(block.Time)
			if timeErr != nil {
				return fmt.Errorf("Error converting time string %v to RRule format: %v\n", block.Time, timeErr)
			}
			// TODO(john-b-yang) Might be start (24 hr), not set time?
			hour := time.OrigOptions.Dtstart.Hour()
			minute := time.OrigOptions.Dtstart.Minute()
			meridiem := "AM"
			if hour > 12 {
				hour -= 12
				meridiem = "PM"
			}
			value += fmt.Sprintf("setTime:%s;", fmt.Sprintf("%v:%v:%v", hour, minute, meridiem))

			// Remove existing schedule
			// TODO(john-b-yang) Should this be a separate go request before creating the new schedule?
			value += fmt.Sprintf("deleteAll")
		}

		// Constructing HTTP Request
		// TODO(john-b-yang) Replace w/ "schedReq"
		resp, _, errs := pel.req.Get(pel.target).
			Param("username", pel.username).
			Param("password", pel.password).
			Param("request", "set").
			Param("object", "thermostatSchedule").
			Param("selection", fmt.Sprintf("name:%s", pel.Name)).
			Param("dayOfWeek", day). // TODO(john-b-yang) ":", not "="
			Param("value", value).
			End()

		if errs != nil {
			return fmt.Errorf("Error modifying thermostat schedule settings: %v\n", errs)
		}

		defer resp.Body.Close()
		fmt.Printf("%v\n", resp.Request.URL)
	}

	return nil
}
