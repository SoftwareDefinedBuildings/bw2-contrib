package main

import (
	"fmt"
	"time"
)

type Vplug struct {
	status bool
	rate time.Duration
	data chan PlugStats
}

type PlugStats struct {
	info bool
}

func NewVplug(rate string) *Vplug {
	dur, err := time.ParseDuration(rate)
	if err != nil {
		panic(err)
	}

	return &Vplug{
		stat
		rate: dur,
		data: make(chan PlugStats),
	}
}

func (v *Vplug) ActuatePlug(status bool) {
	v.status = status
}

func (v *Vplug) GetStatus() PlugStats {
	return PlugStats {
		info: v.status,
	}
}
