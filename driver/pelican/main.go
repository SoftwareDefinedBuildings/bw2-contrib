package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/immesys/spawnpoint/spawnable"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

const TSTAT_PO_NUM = "2.1.1.0"

func main() {
	bwClient := bw2.ConnectOrExit("")
	bwClient.OverrideAutoChainTo(true)
	bwClient.SetEntityFromEnvironOrExit()

	params := spawnable.GetParamsOrExit()
	baseURI := params.MustString("svc_base_uri")
	if !strings.HasSuffix(baseURI, "/") {
		baseURI += "/"
	}
	username := params.MustString("username")
	password := params.MustString("password")
	sitename := params.MustString("sitename")
	name := params.MustString("name")

	pollIntStr := params.MustString("poll_interval")
	pollInt, err := time.ParseDuration(pollIntStr)
	if err != nil {
		fmt.Printf("Invalid poll interval specified: %v\n", err)
		os.Exit(1)
	}

	service := bwClient.RegisterService(baseURI, "s.pelican")
	tstat_iface := service.RegisterInterface("thermostat", "i.xbos.thermostat")

	pelican := NewPelican(username, password, sitename, name)
	for {
		status, err := pelican.GetStatus()
		if err != nil {
			fmt.Printf("Failed to retrieve Pelican status: %v\n", err)
			os.Exit(1)
		}

		po, err := bw2.CreateMsgPackPayloadObject(bw2.FromDotForm(TSTAT_PO_NUM), status)
		if err != nil {
			fmt.Printf("Failed to create msgpack PO: %v", err)
			os.Exit(1)
		}
		tstat_iface.PublishSignal("info", po)
		time.Sleep(pollInt)
	}
}
