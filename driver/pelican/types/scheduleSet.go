package types

import (
	"fmt"

	rrule "github.com/teambition/rrule-go"
)

func (pel *Pelican) SetSchedule(newSchedule *ThermostatSchedule) error {
	for day, blocks := range newSchedule.DaySchedules {

		// Delete Day's Existing Schedule
		respDelete, _, errsDelete := pel.scheduleReq.Get(pel.target).
			Param("username", pel.username).
			Param("password", pel.password).
			Param("request", "set").
			Param("object", "thermostatSchedule").
			Param("selection", fmt.Sprintf("name:%s;dayOfWeek:%s;", pel.Name, day)).
			Param("value", "deleteAll").
			End()

		if errsDelete != nil {
			return fmt.Errorf("Error deleting thermostat schedule settings on day %v: %v\n", day, errsDelete)
		}

		defer respDelete.Body.Close()

		for index, block := range blocks {
			var value string = ""
			value += fmt.Sprintf("coolSetting:%.0f;", block.CoolSetting)
			value += fmt.Sprintf("heatSetting:%.0f", block.HeatSetting)
			value += fmt.Sprintf("system:%s", block.System)

			time, timeErr := rrule.StrToRRule(block.Time)
			if timeErr != nil {
				return fmt.Errorf("Error converting time string %v to RRule format: %v\n", block.Time, timeErr)
			}

			hour := time.OrigOptions.Dtstart.Hour()
			minute := time.OrigOptions.Dtstart.Minute()
			value += fmt.Sprintf("startTime:%s;", fmt.Sprintf("%v:%v", hour, minute))

			// Set (Day, Time)'s Schedule
			respSet, _, errsSet := pel.scheduleReq.Get(pel.target).
				Param("username", pel.username).
				Param("password", pel.password).
				Param("request", "set").
				Param("object", "thermostatSchedule").
				Param("selection", fmt.Sprintf("name:%s;dayOfWeek:%s;setTime:%v;", pel.Name, day, index)).
				Param("value", value).
				End()

			if errsSet != nil {
				return fmt.Errorf("Error setting schedule for thermostat on day %v at index %v: %v", day, index, errsSet)
			}

			defer respSet.Body.Close()
		}
	}

	return nil
}
