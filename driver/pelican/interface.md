### Pelican Schedule Interface Outline

##### Context

The following structs define the way users interact with the Scheduling interface of the Pelican thermostats. These structs are used to define the weekly schedule and will be interpretted by the thermSchedule.go code to retrieve/view in addition to making changes to the existing schedule for individual thermostats within a particular site. These structs are public and accessible to anyone who subscribes to the allotted endpoint or publishes to the assigned signal.

##### Schedule Structs

```
// Struct mapping each day of the week to its daily schedule
type ThermostatSchedule struct {
  DaySchedules map[string]ThermostatDaySchedule `msgpack:"day_schedules"`
}

// Struct containing a series of blocks that describes a one day schedule
type ThermostatDaySchedule struct {
  Blocks []ThermostatBlockSchedule `msgpack:blocks`
}

// Struct containing data defining the settings of each schedule block
type ThermostatBlockSchedule struct {
  CoolSetting float64 `msgpack:"cool_setting"` // Cooling turns on when room temperature exceeds cool setting temp.
  HeatSetting float64 `msgpack:"heat_setting"` // Heating turns on when room temperature drops below heat setting temp.
  System      string  `msgpack:"system"`       // Indicates if system is heating, cooling, off, or set to auto
  Time        string  `msgpack:"time"`         // Indicates the time of day which the above settings are enacted.
}
```

##### Schedule Structs Explanation

Each Pelican Thermostat has three potential schedule settings.
1. Weekly: Each day of the week (Sun - Sat) has a unique daily schedule setting
2. Daily: Each day of the week has the same daily schedule
3. Weekday/Weekend: Per the name, weekdays and weekends have different schedules.

Next, it's wise if we attempt to define what a "daily schedule" actually looks like. Each day's schedule consist of a series of what we'll call "blocks". Each block details a certain number of settings that are enacted at a certain time of day. This is encapsulated by the ThermostatBlockSchedule struct. For example, one might have a series of four different blocks with time intervals at 6:00 a.m., 11:00 a.m., 4:00 p.m., and 6:00 p.m. At each of these times, the associated cool temperature, heat temperature, and system settings are all enacted.

Going one layer above, the ThermostatDaySchedule struct represents an array of blocks. The purpose of this struct is to represent the schedule of one day a.k.a a series of blocks. Last but not least, the outermost struct, "ThermostatSchedule", maps each day of the week (Sunday - Saturday) to their respective daily schedules (ThermostatDaySchedule struct). This is the struct that is delivered to the user for getting and setting purposes.

##### Thermostat Block Schedule Struct Fields Explanation

- CoolSetting: The cool setting refers to the temperature at which the system begins cooling. In other words, if the room temperature surpasses this threshold, the cooling system is activated. The unit of temperature is Fahrenheit.
- HeatSetting: The heat setting refers to the temperature at which the system begins heating. In other words, if the room temperature falls below this threshold, the heating system is activated. The unit of temperature is Fahrenheit.
- System: This indicates what the system is currently doing. There are four possible settings (heat/cool/off/auto) which are pretty self explanatory. Heat and cool mean the systems heating or cooling the room. Auto means that the system will automatically heat or cool according to the room temperature and cool/heat thresholds.
- Time: Time describes what time of day the particular block's settings are enacted (i.e. 6:00:AM). This time is in the RRule format (https://tools.ietf.org/html/rfc5545), and the rrule-go library is used to convert the given time into the designated format. The following section describes how time is formatted and defined in greater detail.

##### A Deeper Dive into Time Format (RRule)

Within "thermoSchedule.go", this particular block in the "convertTimeToRRule" function is responsible for creating the RRule format.

```
rruleSched, _ := rrule.NewRRule(rrule.ROption{
  Freq:    rrule.WEEKLY,
  Wkst:    weekRRule[dayOfWeek],
  Dtstart: time.Date(0, 0, 0, hour, minute, 0, 0, timezone),
})
```

Three fields are configured.
- Frequency indicates the interval with which this event occurs.
- Wkst tells us which day of the week (Sunday - Saturday) this event occurs.
- Dtstart is a required field that indicates the "start date" of the particular event. In Go, the Dtstart field is a time.Date object, which is initialized with the following parameters: year, month, day, hour minute, second, millisecond, timezone. For our purposes, there is no real concept of a "start date", just the time, so the year, month, and day parameters are filled with dummy values of 0. Only hour, minute, and timezone (which can be determined from the Pelican settings + schedule) are filled in. As long as an individual knows the time is in RRule format, he or she will be able to determine each field.

##### XBOS Interface Configuration

The current version of XBOS uses YAML files to define the expectations for the output of different functionalities of the driver code from the bw2-contrib repository. There are a couple limitations regarding what the YAML files are able to represent. The incumbent version of XBOS features protobuf definitions for messages. When the next release of XBOS comes, both new and existing YAML files will be created and modified to reflect the outputs' types more accurately.
