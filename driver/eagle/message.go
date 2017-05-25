package main

import (
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

type meterMessage struct {
	Current_demand float64
	Time           int64
}

func (msg meterMessage) ToMsgPackBW() (po bw2.PayloadObject) {
	po, _ = bw2.CreateMsgPackPayloadObject(bw2.FromDotForm("2.0.9.1"), map[string]interface{}{"current_demand": msg.Current_demand, "time": msg.Time})
	return
}
