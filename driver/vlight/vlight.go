package main

import (
	"fmt"
)

type Vlight struct {
	state bool
	brightness int
}

type Info struct {
	State bool
	Brightness int
}

func NewVlight() *Vlight {
	return &Vlight {
		state: false,
		brightness: 0,
	}
}

func (v *Vlight) ActuateLight(state bool) {
	v.state = state
	if v.state == false {
		v.SetBrightness(0)
	}
}

func (v *Vlight) SetBrightness(brightness int) {
	if brightness < 0 || brightness > 100 {
		fmt.Println("Brightness must be between 0 and 100")
		return
	}
	v.brightness = brightness
}

func (v *Vlight) GetStatus() Info {
	return Info {
		State: v.state,
		Brightness: v.brightness,
	}
}
