package main

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type SmapReading struct {
	UUID     string
	Readings [][]json.Number
}

func (msg TimeseriesReading) ToSmapReading(path string) ([]byte, error) {
	floatString := strconv.FormatFloat(msg.Value, 'f', -1, 64)
	timeString := strconv.FormatInt(msg.Time, 10)
	rdg := &SmapReading{
		UUID:     msg.UUID,
		Readings: [][]json.Number{[]json.Number{json.Number(timeString), json.Number(floatString)}},
	}
	b, e := json.MarshalIndent(map[string]*SmapReading{
		path: rdg,
	}, "", " ")
	if e != nil {
		fmt.Println(e)
	}
	fmt.Println(string(b))
	return json.Marshal(map[string]*SmapReading{
		path: rdg,
	})
}
