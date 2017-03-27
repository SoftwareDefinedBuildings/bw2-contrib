package main

type InfoResponse struct {
	Time           int64   `json:"-" msgpack:"time"`
	UUID           string  `json:"UUID" msgpack:"UUID"`
	Name           string  `json:"name" msgpack:"name"`
	Mode           int     `json:"mode" msgpack:"mode"`
	State          int     `json:"state" msgpack:"state"`
	Fan            int     `json:"fan" msgpack:"fan"`
	FanState       int     `json:"fanstate" msgpack:"fanstate"`
	TempUnits      int     `json:"tempunits" msgpack:"tempunits"`
	Schedule       int     `json:"schedule" msgpack:"schedule"`
	SchedulePart   int     `json:"schedulepart" msgpack:"schedulepart"`
	Away           int     `json:"away" msgpack:"away"`
	Holiday        int     `json:"holiday" msgpack:"holiday"`
	Override       int     `json:"override" msgpack:"override"`
	OverrideTime   int     `json:"overridetime" msgpack:"overridetime"`
	ForceUnocc     int     `json:"forceunocc" msgpack:"forceunocc"`
	SpaceTemp      float64 `json:"spacetemp" msgpack:"spacetemp"`
	HeatTemp       float64 `json:"heattemp" msgpack:"heattemp"`
	CoolTemp       float64 `json:"cooltemp" msgpack:"cooltemp"`
	CoolTempMin    float64 `json:"cooltempmin" msgpack:"cooltempmin"`
	CooltempMax    float64 `json:"cooltempmax" msgpack:"cooltempmax"`
	HeatTempMin    float64 `json:"heattempmin" msgpack:"heattempmin"`
	HeatTempMax    float64 `json:"heattempmax" msgpack:"heattempmax"`
	SetpointDelta  float64 `json:"setpointdelta" msgpack:"setpointdelta"`
	AvailableModes int     `json:"availablemodes" msgpack:"availablemodes"`
}
