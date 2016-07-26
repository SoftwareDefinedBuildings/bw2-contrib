package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
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
	return json.Marshal(map[string]*SmapReading{
		path: rdg,
	})
}

func (msg TimeseriesReading) SendToSmap(msgPath, uri string) error {
	mybytes, err := msg.ToSmapReading(msgPath)
	if err != nil {
		return err
	}
	resp, err := http.Post(uri, "application/json", bytes.NewBuffer(mybytes))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Got status code %d\n", resp.StatusCode)
	}
	return nil
}
