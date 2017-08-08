package main

import (
	"crypto/sha1"
	"encoding/xml"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/immesys/spawnpoint/spawnable"
	"github.com/levigross/grequests"
	"github.com/pkg/errors"
	bw2 "gopkg.in/immesys/bw2bind.v5"
)

func main() {
	client := bw2.ConnectOrExit("")
	client.SetEntityFromEnvironOrExit()

	params := spawnable.GetParamsOrExit()
	baseuri := params.MustString("svc_base_uri")
	poll_interval := params.MustString("poll_interval")
	poll_duration, err := time.ParseDuration(poll_interval)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Could not parse given poll interval"))
	}

	service := client.RegisterService(baseuri, "s.enlighted")

	//doEnlighted()
	system := &EnlightedSystem{
		IPAddress: params.MustString("ipaddress"),
		APIKey:    params.MustString("apikey"),
		UserId:    params.MustString("userid"),
		duration:  poll_duration,
		service:   service,
		floors:    []string{},
		lights:    make(map[string][]string),
		fixtures:  make(map[string]*Fixture),
	}

	// make sure the set of lights is up to date
	system.Refresh()
	for _ = range time.Tick(60 * time.Second) {
		system.Refresh()
	}
}

type EnlightedSystem struct {
	IPAddress string
	APIKey    string
	UserId    string
	service   *bw2.Service
	floors    []string
	lights    map[string][]string
	fixtures  map[string]*Fixture
	duration  time.Duration
	sync.Mutex
}

func (sys *EnlightedSystem) Refresh() {
	floors := sys.GetAllFloors()
	fixture_ids := sys.GetAllLightIds(floors)
	for _, fixture_id := range fixture_ids {
		sys.GetFixture(fixture_id)
	}
}

func (sys *EnlightedSystem) AuthenticationToken() (string, string) {
	timestamp := time.Now().UnixNano() // convert to milliseconds
	timestamp_str := strconv.FormatInt(timestamp, 10)
	str := fmt.Sprintf("%s%s%s", sys.UserId, timestamp_str, sys.APIKey)
	return fmt.Sprintf("%x", sha1.Sum([]byte(str))), timestamp_str
}

func (sys *EnlightedSystem) GetHeaders() *grequests.RequestOptions {
	token, timestamp := sys.AuthenticationToken()
	return &grequests.RequestOptions{
		InsecureSkipVerify: true,
		Headers: map[string]string{
			"UserId": sys.UserId,
			"ts":     timestamp,
			"AuthenticationToken": token,
			"Accept":              "application/json",
			"Content-type":        "application/xml",
		},
	}
}

func (sys *EnlightedSystem) GetFixture(id string) *Fixture {
	sys.Lock()
	defer sys.Unlock()
	if _, found := sys.fixtures[id]; !found {
		fmt.Println("discovered fixture:", id)
		fixture := &Fixture{
			id:  id,
			sys: sys,
		}
		state, err := fixture.GetState()
		if err != nil {
			log.Println(errors.Wrap(err, "Could not get status of fixture"))
			return nil
		}
		fixture.light_iface = sys.service.RegisterInterface(state.Name, "i.xbos.light")
		fixture.occ_iface = sys.service.RegisterInterface(state.Name, "i.xbos.occupancy_sensor")
		fixture.meter_iface = sys.service.RegisterInterface(state.Name, "i.xbos.meter")
		go fixture.PollAndReport(sys.duration)
		go fixture.ListenActuation()

		sys.fixtures[id] = fixture
	}

	return sys.fixtures[id]
}

func (sys *EnlightedSystem) GetAllFloors() []string {
	url := fmt.Sprintf("https://%s/ems/api/org/floor/list", sys.IPAddress)
	resp, err := grequests.Get(url, sys.GetHeaders())
	if err != nil {
		log.Fatal(err)
	}
	if !resp.Ok {
		fmt.Println(resp.StatusCode)
		fmt.Println(string(resp.Bytes()))
		log.Fatal(err)
	}
	var m interface{}
	if err := resp.JSON(&m); err != nil {
		log.Fatal(err)
	}
	fmt.Println(m)

	var ids []string

	floors := m.(map[string]interface{})["floor"]
	//fmt.Println(floors.(type))
	switch t := floors.(type) {
	case map[string]interface{}:
		ids = append(ids, t["id"].(string))
	case []map[string]interface{}:
		for _, f := range t {
			ids = append(ids, f["id"].(string))
		}
	default:
		fmt.Printf("%T", t)
	}

	return ids
}

func (sys *EnlightedSystem) GetAllLightIds(ids []string) []string {
	var fixture_ids []string
	for _, id := range ids {
		url := fmt.Sprintf("https://%s/ems/api/org/fixture/location/list/floor/%s/1", sys.IPAddress, id)
		resp, err := grequests.Get(url, sys.GetHeaders())
		if err != nil {
			log.Fatal(err)
		}
		if !resp.Ok {
			fmt.Println(resp.StatusCode)
			fmt.Println(string(resp.Bytes()))
			log.Fatal(err)
		}
		var m interface{}
		if err := resp.JSON(&m); err != nil {
			log.Fatal(err)
		}
		fixtures := m.(map[string]interface{})["fixture"].([]interface{})
		for _, fixture := range fixtures {
			fixture_id := fixture.(map[string]interface{})["id"].(string)
			fixture_ids = append(fixture_ids, fixture_id)
		}
	}
	return fixture_ids
}

//func (sys *EnlightedSystem) GetFixtureStatus(fixture_id string) string {
//	url := fmt.Sprintf("https://%s/ems/api/org/fixture/details/%s", sys.IPAddress, fixture_id)
//	resp, err := grequests.Get(url, sys.GetHeaders())
//	if err != nil {
//		log.Fatal(err)
//	}
//	if !resp.Ok {
//		fmt.Println(resp.StatusCode)
//		fmt.Println(string(resp.Bytes()))
//		log.Fatal(err)
//	}
//	var m interface{}
//	if err := resp.JSON(&m); err != nil {
//		log.Fatal(err)
//	}
//	state := m.(map[string]interface{})
//	fmt.Println("> wattage", state["wattage"])
//	fmt.Println("> lastocc seen", state["lastoccupancyseen"])
//	fmt.Println("> lightlevel", state["lightlevel"])
//	fmt.Println("> modelNo", state["modelNo"])
//	fmt.Println("> name", state["name"])
//	lightlevel := state["lightlevel"].(string)
//	return lightlevel
//}
//
//func (sys *EnlightedSystem) GetSensorStatus(fixture_id string) {
//	url := fmt.Sprintf("https://%s/ems/api/org/sensor/v2/details/%s", sys.IPAddress, fixture_id)
//	resp, err := grequests.Get(url, sys.GetHeaders())
//	if err != nil {
//		log.Fatal(err)
//	}
//	if !resp.Ok {
//		fmt.Println(resp.StatusCode)
//		fmt.Println(string(resp.Bytes()))
//		log.Fatal(err)
//	}
//	var m interface{}
//	if err := resp.JSON(&m); err != nil {
//		log.Fatal(err)
//	}
//	state := m.(map[string]interface{})
//	fmt.Println("> name", state["name"])
//	fmt.Println("> temperature", state["temperature"])
//	fmt.Println("> power", state["power"])
//	fmt.Println("> lightlevel", state["lightlevel"])
//}

func (sys *EnlightedSystem) SetFixtureState(fixture_id string, state int64, time int64) {
	type fixtures struct {
		XMLName xml.Name `xml:"fixtures"`
		Id      string   `xml:"fixture>id"`
	}
	req := sys.GetHeaders()
	req.XML = fixtures{Id: fixture_id}

	url := fmt.Sprintf("https://%s/ems/api/org/fixture/v1/op/dim/ABS/%d?time=%d", sys.IPAddress, state, time)
	resp, err := grequests.Post(url, req)
	if err != nil {
		log.Fatal(err)
	}
	if !resp.Ok {
		fmt.Println(resp.StatusCode)
		fmt.Println(string(resp.Bytes()))
		log.Fatal(err)
	}
	fmt.Println(url)
	fmt.Println(resp.String())
	//var m interface{}
	//if err := resp.JSON(&m); err != nil {
	//	log.Fatal(err)
	//}
	//state := m.(map[string]interface{})
	//lightlevel := state["lightlevel"].(string)
	//return lightlevel
}
