package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/parnurzeal/gorequest"
	"github.com/pkg/errors"
)

// TODO: right now assuming there's just one JuicePlug and that its called "JuicePlug"

var JuiceNetURI = "http://emwjuicebox.cloudapp.net/box_pin"

// manual says https, but its actually http
var JuiceNetDeviceURI = "http://emwjuicebox.cloudapp.net/box_api_secure"

type juiceAccountUnitResponse struct {
	Success bool          `json:"success"`
	Units   []juiceDevice `json:"units"`
}

type juiceDevice struct {
	Token   string `json:"token"`
	Unit_id string `json:"unit_id"`
	// this is mentioned in the API but seems to be empty
	Name string `json:"name"`
}

type JuiceDevice struct {
	Token         string `msgpack:"-"`
	Unit_id       string `msgpack:"unit_id"`
	ID            string `msgpack:"ID"`
	InfoTimestamp uint64 `msgpack:"info_timestamp"`
	ShowOverride  bool   `msgpack:"show_override"`
	State         string `msgpack:"state"`
	Charging      struct {
		AmpsLimit        uint64 `msgpack:"amps_limit"`
		AmpsCurrent      uint64 `msgpack:"amps_current"`
		Voltage          uint64 `msgpack:"voltage"`
		WhEnergy         uint64 `msgpack:"wh_energy"`
		Savings          uint64 `msgpack:"savings"`
		WattPower        uint64 `msgpack:"watt_power"`
		SecondsCharging  uint64 `msgpack:"seconds_charging"`
		WhEnergyAtPlugin uint64 `msgpack:"wh_energy_at_plugin"`
		WhEnergyToAdd    uint64 `msgpack:"wh_energy_to_add"`
	} `msgpack:"charging"`
	Lifetime struct {
		WhEnergy uint64 `msgpack:"wh_energy"`
		Savings  uint64 `msgpack:"savings"`
	} `msgpack:"lifetime"`
	ChargingTimeLeft  uint64 `msgpack:"charging_time_left"`
	PlugUnplugTime    uint64 `msgpack:"plug_unplug_time"`
	TargetTime        uint64 `msgpack:"target_time"`
	OverrideTime      uint64 `msgpack:"override_time"`
	DefaultTargetTime uint64 `msgpack:"default_target_time"`
	TimeLastPing      uint64 `msgpack:"time_last_ping"`
	UTCTime           uint64 `msgpack:"utc_time"`
	UnitTime          uint64 `msgpack:"unit_time"`
	UpdateInterval    uint64 `msgpack:"update_interval"`
	Temperature       uint64 `msgpack:"temperature"`
	Frequency         uint64 `msgpack:"frequency"`
	CarId             uint64 `msgpack:"car_id"`
	Success           bool   `msgpack:"succes"`
}

type Account struct {
	AccountToken string
	Devices      map[string]*JuiceDevice
}

func NewAccount(account_token string) *Account {
	acc := &Account{
		AccountToken: account_token,
		Devices:      make(map[string]*JuiceDevice),
	}

	// look for devices
	req := gorequest.New()
	resp, _, err := req.Post(JuiceNetURI).Type("json").
		SendMap(map[string]string{
			"cmd":           "get_account_units",
			"device_id":     "JuicePlug",
			"account_token": acc.AccountToken,
		}).End()
	if len(err) > 0 {
		log.Fatal(errors.Wrap(err[0], "Could not fetch account units"))
	}
	defer resp.Body.Close()

	var units juiceAccountUnitResponse
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&units); err != nil {
		log.Fatal(errors.Wrap(err, "Could not decode account units"))
	}

	for _, unit := range units.Units {
		JD := &JuiceDevice{
			Token:   unit.Token,
			Unit_id: unit.Unit_id,
		}
		log.Printf("Found JuicePlug with UnitID %s and Token %s", unit.Unit_id, unit.Token)
		acc.Devices[unit.Unit_id] = JD
	}

	return acc
}

func (acc *Account) read_devices() []JuiceDevice {
	var devices []JuiceDevice
	for _, jd := range acc.Devices {
		req := gorequest.New()
		resp, _, err := req.
			TLSClientConfig(&tls.Config{InsecureSkipVerify: true}).
			Post(JuiceNetDeviceURI).Type("json").
			SendMap(map[string]string{
				"cmd":           "get_state",
				"device_id":     "JuicePlug",
				"account_token": acc.AccountToken,
				"token":         jd.Token,
			}).End()
		if len(err) > 0 {
			log.Fatal(errors.Wrap(err[0], "Could not fetch device"))
		}
		defer resp.Body.Close()

		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(&jd); err != nil {
			log.Fatal(errors.Wrap(err, "Could not decode device"))
		}
		log.Printf("%+v", jd)
		devices = append(devices, *jd)
	}
	return devices
}

type write_params struct {
	State       *bool    `msgpack:"state"`
	ChargeLimit *float64 `msgpack:"current_limit"`
}

func (acc *Account) write_device(unit_id string, params write_params) error {
	jd, found := acc.Devices[unit_id]
	if !found {
		return errors.New(fmt.Sprintf("No device found with that ID (%s)", unit_id))
	}
	var amperage int64
	if params.State != nil && !(*params.State) {
		amperage = 0
	} else if params.ChargeLimit != nil {
		amperage = int64(*params.ChargeLimit)
	}

	// clamp at 40 amps (juiceplug limit)
	if amperage > 40 {
		amperage = 40
	}
	req := gorequest.New()
	resp, _, err := req.
		TLSClientConfig(&tls.Config{InsecureSkipVerify: true}).
		Post(JuiceNetDeviceURI).Type("json").
		SendMap(map[string]string{
			"cmd":           "set_limit",
			"device_id":     "JuicePlug",
			"account_token": acc.AccountToken,
			"token":         jd.Token,
			"amperage":      strconv.FormatInt(amperage, 10),
		}).End()
	if len(err) > 0 {
		log.Fatal(errors.Wrap(err[0], "Could not fetch device"))
	}
	defer resp.Body.Close()

	var m map[string]interface{}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&m); err != nil {
		log.Fatal(errors.Wrap(err, "Could not decode device"))
	}
	fmt.Println("actuation result:", m)
	if m["success"].(bool) {
		return nil
	} else {
		return errors.New("Could not set current limit")
	}
}
