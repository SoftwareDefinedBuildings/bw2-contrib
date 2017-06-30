package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

const JUICEPLUG_DF = "2.0.0.0"

func main() {
	bwClient := bw2.ConnectOrExit("")
	bwClient.OverrideAutoChainTo(true)
	bwClient.SetEntityFromEnvironOrExit()

	params := spawnable.GetParamsOrExit()
	baseURI := params.MustString("svc_base_uri")
	if !strings.HasSuffix(baseURI, "/") {
		baseURI += "/"
	}
	account_token := params.MustString("account_token")
	poll_interval, parseErr := time.ParseDuration(params.MustString("poll_interval"))
	if parseErr != nil {
		log.Fatal("Could not parse duration", parseErr)
	}

	acc := NewAccount(account_token)

	fmt.Println(acc)
	service := bwClient.RegisterService(baseURI, "s.juiceplug")

	// TODO: define slots

	var ifaces = make(map[string]*bw2.Interface)

	for _ = range time.Tick(poll_interval) {
		for _, device := range acc.read_devices() {
			var iface *bw2.Interface
			var found bool
			if iface, found = ifaces[device.Unit_id]; !found {
				iface = service.RegisterInterface(device.Unit_id, "i.juiceplug")
				ifaces[device.Unit_id] = iface
			}
			po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(JUICEPLUG_DF), device)
			if err != nil {
				log.Println("Could not publish", err)
				continue
			}
			iface.PublishSignal("info", po)
		}
	}
}
