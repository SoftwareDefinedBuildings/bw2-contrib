package main

import (
	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

type InfoData struct {
	state bool
}

func (i *InfoData) ToMsgPackPO() (bo bw2.PayloadObject) {
	po, err := bw2.CreateMsgPackPayloadObject(bw2.PONumTimeseriesReading, i)
	if err != nil {
		panic(err)
	}
	return po
}

func main() {

}