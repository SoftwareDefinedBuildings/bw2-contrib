### Pelican Schedule Interface Outline

##### Context

The following structs define the way users interact with the Scheduling interface of the Pelican thermostats. These structs are used to define the weekly schedule and will be interpretted by the thermSchedule.go code to retrieve/view in addition to making changes to the existing schedule for individual thermostats within a particular site. These structs are public and accessible to anyone who subscribes to the allotted endpoint or publishes to the assigned signal.

##### Schedule Structs

// Struct mapping each day of the week to its daily schedule <br>
type ThermostatSchedule struct {<br>
&nbsp;&nbsp;&nbsp;DaySchedules map[string]ThermostatDaySchedule `msgpack:"day_schedules"`<br>
}<br>

// Struct containing a series of blocks that describes a one day schedule <br>
type ThermostatDaySchedule struct { <br>
&nbsp;&nbsp;&nbsp;Blocks []ThermostatBlockSchedule `msgpack:blocks` <br>
} <br>

// Struct containing data defining the settings of each schedule block <br>
type ThermostatBlockSchedule struct { <br>
&nbsp;&nbsp;&nbsp;CoolSetting float64 `msgpack:"cool_setting"` <br>
&nbsp;&nbsp;&nbsp;HeatSetting float64 `msgpack:"heat_setting"` <br>
&nbsp;&nbsp;&nbsp;System      string  `msgpack:"system"` <br>
&nbsp;&nbsp;&nbsp;Time        string  `msgpack:"time"` <br>
}

##### Schedule Structs Explanation

Each Pelican Thermostat has three potential schedule settings.
1. Weekly: Each day of the week (Sun - Sat) has a unique daily schedule setting
2. Daily: Each day of the week has the same daily schedule
3. Weekday/Weekend: Per the name, weekdays and weekends have different schedules.

Next, it's wise if we attempt to define what a "daily schedule" actually looks like. Each day's schedule consist of a series of what we'll call "blocks". Each block details a certain number of settings that are enacted at a certain time of day. This is encapsulated by the ThermostatBlockSchedule struct. For example, one might have a series of four different blocks with time intervals at 6:00 a.m., 11:00 a.m., 4:00 p.m., and 6:00 p.m. At each of these times, the associated cool temperature, heat temperature, and system settings are all enacted.

Going one layer above, the ThermostatDaySchedule struct represents an array of blocks. The purpose of this struct is to represent the schedule of one day a.k.a a series of blocks. Last but not least, the outermost struct, "ThermostatSchedule", maps each day of the week (Sunday - Saturday) to their respective daily schedules (ThermostatDaySchedule struct). This is the struct that is delivered to the user for getting and setting purposes.

##### XBOS Interface Configuration

The current version of XBOS uses YAML files to define the expectations for the output of different functionalities of the driver code from the bw2-contrib repository. There are a couple limitations regarding what the YAML files are able to represent. The incumbent version of XBOS features protobuf definitions for messages. When the next release of XBOS comes, both new and existing YAML files will be created and modified to reflect the outputs' types more accurately.
