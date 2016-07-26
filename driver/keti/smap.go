package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
)

type SmapReading struct {
	UUID     string `json:"uuid"`
	Readings [][]json.Number
}

func (rdg *SmapReading) AddReading(time int64, value float64) {
	floatString := strconv.FormatFloat(value, 'f', -1, 64)
	timeString := strconv.FormatInt(time, 10)
	rdg.Readings = append(rdg.Readings, []json.Number{json.Number(timeString), json.Number(floatString)})
}

func (msg TimeseriesReading) toSmapReading() *SmapReading {
	floatString := strconv.FormatFloat(msg.Value, 'f', -1, 64)
	timeString := strconv.FormatInt(msg.Time, 10)
	rdg := &SmapReading{
		UUID:     msg.UUID,
		Readings: [][]json.Number{[]json.Number{json.Number(timeString), json.Number(floatString)}},
	}
	return rdg
}

type BufferedSender struct {
	uri      string
	tosend   map[string]*SmapReading
	max, num int
	sync.Mutex
}

func NewBufferedSender(uri string, max int) *BufferedSender {
	return &BufferedSender{
		tosend: make(map[string]*SmapReading),
		uri:    uri,
		max:    max,
		num:    0,
	}
}

func (buf *BufferedSender) Send(path string, msg TimeseriesReading) error {
	buf.Lock()
	defer buf.Unlock()
	if _, found := buf.tosend[path]; found {
		buf.tosend[path].AddReading(msg.Time, msg.Value)
	} else {
		buf.tosend[path] = msg.toSmapReading()
	}
	buf.num += 1
	if buf.num < buf.max {
		return nil
	}
	// here, send
	sendme, err := json.Marshal(buf.tosend)
	if err != nil {
		return err
	}
	resp, err := http.Post(buf.uri, "application/json", bytes.NewBuffer(sendme))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		reason, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("Got status code %d: %s\n", resp.StatusCode, reason)
	}
	buf.tosend = make(map[string]*SmapReading)
	buf.num = 0
	return nil
}
