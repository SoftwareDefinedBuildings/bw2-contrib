package main

type Vplug struct {
	status bool
}

func NewVplug() *Vplug {
	return &Vplug{
		status: false,
	}
}

func (v *Vplug) ActuatePlug(status bool) {
	v.status = status
}

func (v *Vplug) GetStatus() bool {
	return v.status
}
