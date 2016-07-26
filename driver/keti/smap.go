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

type BufferedSender struct {
	uri      string
	tosend   []byte
	max, num int
	sync.Mutex
}

func NewBufferedSender(uri string, max int) *BufferedSender {
	return &BufferedSender{
		tosend: []byte{},
		uri:    uri,
		max:    max,
		num:    0,
	}
}

func (buf *BufferedSender) Send(path string, msg TimeseriesReading) error {
	b, err := msg.ToSmapReading(path)
	if err != nil {
		return err
	}
	buf.Lock()
	defer buf.Unlock()
	buf.num += 1
	if buf.num < buf.max {
		buf.tosend = append(buf.tosend, b...)
		return nil
	}
	sendme := buf.tosend
	resp, err := http.Post(buf.uri, "application/json", bytes.NewBuffer(sendme))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		reason, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("Got status code %d: %s\n", resp.StatusCode, reason)
	}
	buf.tosend = []byte{}
	buf.num = 0
	return nil
}
