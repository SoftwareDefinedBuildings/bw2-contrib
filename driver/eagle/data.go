package main

func (srv *EagleServer) forwardDemandData(demand *InstantaneousDemand) {
	demand.Dump()
}
