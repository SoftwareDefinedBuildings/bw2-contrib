// There are two approaches for the Eagle driver. We either implement a centralized service that the Eagle is
// configured to talk to, or we implement a local driver that then polls the Eagle's local REST interface.
// This file implements the former.
// We implement a server with a single URL served over HTTPS
//	- GET: returns a page of directions on how to set this up on your own Eagle
//	- POST: the Eagle will POST to this URL. The server will check if we've already seen the Eagle.
//			If it hasn't, we register the Eagle locally (instantiate another struct) and configure it for our
//		  	desired reporting interval, and extract all the necessary metadata
//			Finally, for all received messages, we process the latest reading and mirror it onto BOSSWAVE
// Configuration options:
//	- reporting interval (TODO: figure out what the bounds are)
//	- how much historical data to grab (TODO: figure out what the bounds are), which is then replayed on BOSSWAVE
//		with the appropriate timestamps
package main

import (
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/op/go-logging"
	"golang.org/x/crypto/acme/autocert"
)

// logger
var log *logging.Logger

// set up logging facilities
func init() {
	log = logging.MustGetLogger("eagle")
	var format = "%{color}%{level} %{time:Jan 02 15:04:05} %{shortfile}%{color:reset} ▶ %{message}"
	var logBackend = logging.NewLogBackend(os.Stderr, "", 0)
	logBackendLeveled := logging.AddModuleLevel(logBackend)
	logging.SetBackend(logBackendLeveled)
	logging.SetFormatter(logging.MustStringFormatter(format))
}

type EagleConfig struct {
	PollRate time.Duration
}

// server config
type Config struct {
	Port          string
	ListenAddress string
	TLSHost       string
}

type EagleServer struct {
	// server configuration
	cfg *Config
	// MAC address -> eagle instance
	eagles    map[string]*Eagle
	eagleLock sync.RWMutex

	// HTTPS server
	address string
	router  *httprouter.Router
}

func StartEagleServer(cfg *Config) {
	server := &EagleServer{
		eagles: make(map[string]*Eagle),
		router: httprouter.New(),
	}

	server.router.POST("/", server.handleData)
	server.router.GET("/", server.handleHome)

	server.address = cfg.ListenAddress + ":" + cfg.Port
	address, err := net.ResolveTCPAddr("tcp4", server.address)
	if err != nil {
		log.Fatalf("Error resolving address %s (%s)", server.address, err.Error())
	}
	http.Handle("/", server.router)
	log.Noticef("Starting HTTP Server on %s", server.address)
	if cfg.TLSHost != "" {
		m := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(cfg.TLSHost),
			Cache:      autocert.DirCache("certs"),
		}
		s := &http.Server{
			Addr:      address.String(),
			TLSConfig: &tls.Config{GetCertificate: m.GetCertificate},
		}
		log.Fatal(s.ListenAndServeTLS("", ""))
	} else {
		srv := &http.Server{
			Addr: address.String(),
		}
		log.Fatal(srv.ListenAndServe())
	}
}

func (srv *EagleServer) handleData(rw http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	// parse XML
	var resp Response
	defer req.Body.Close()
	dec := xml.NewDecoder(req.Body)
	if err := dec.Decode(&resp); err != nil {
		log.Error(fmt.Sprintf("Could not decode response: %s", err))
		rw.Write([]byte(fmt.Sprintf("Could not decode response: %s", err)))
		rw.WriteHeader(500)
		return
	}
	// TODO: have this method return any configuration struct
	srv.HandleMessage(resp)
}

func (srv *EagleServer) handleHome(rw http.ResponseWriter, req *http.Request, ps httprouter.Params) {
}

func (srv *EagleServer) HandleMessage(resp Response) {
	// if we haven't seen this eagle before, ignore the message.
	// We only want to register off of the NetworkInfo messages

	// handle registration of a new eagle
	if resp.NetworkInfo != nil {
		info := resp.NetworkInfo
		srv.eagleLock.Lock()
		defer srv.eagleLock.Unlock()

		var eagle *Eagle
		var found bool

		if eagle, found = srv.eagles[resp.MacID]; !found {
			eagle = &Eagle{}
		}
		eagle.DeviceMAC = info.DeviceMacId
		eagle.InstallCode = info.InstallCode
		eagle.LinkKey = info.LinkKey
		eagle.FWVersion = info.FWVersion
		eagle.HWVersion = info.HWVersion
		eagle.ImageType = info.ImageType
		eagle.Manufacturer = info.Manufacturer
		eagle.ModelID = info.ModelID
		eagle.DateCode = info.DateCode
		srv.eagles[resp.MacID] = eagle

		if !found {
			log.Noticef("Registering new Eagle with MAC %s", eagle.DeviceMAC)
		}

		// TODO: send configuration struct in reply

		return
	}

	// handle meter data
	if resp.InstantaneousDemand != nil {
		// update the object with the Meter MAC address
		// but only if we've seen the Eagle before; else, drop this
		srv.eagleLock.Lock()
		defer srv.eagleLock.Unlock()
		eagle, found := srv.eagles[resp.MacID]
		if !found {
			return
		}

		// adjust the timestamp with the EAGLE Epoch and get the actual kW demand as a float
		info := resp.InstantaneousDemand
		info.ActualTimestamp = int64(*info.TimeStamp + HexInt64(EAGLE_EPOCH))
		info.ActualDemand = float64(*info.Demand) * float64(*info.Multiplier) / float64(*info.Divisor)
		eagle.MeterMAC = info.MeterMacId
		srv.eagles[resp.MacID] = eagle

		srv.forwardDemandData(info)

		return
	}

	if resp.PriceCluster != nil {
		log.Debugf("%+v", resp.PriceCluster)
		return
	}

	if resp.MessageCluster != nil {
		log.Debugf("%+v", resp.MessageCluster)
		return
	}

	if resp.CurrentSummationDelivered != nil {
		// update the object with the Meter MAC address
		// but only if we've seen the Eagle before; else, drop this
		srv.eagleLock.Lock()
		defer srv.eagleLock.Unlock()
		eagle, found := srv.eagles[resp.MacID]
		if !found {
			return
		}

		info := resp.CurrentSummationDelivered
		info.ActualTimestamp = int64(*info.TimeStamp + HexInt64(EAGLE_EPOCH))
		info.ActualSummationDelivered = float64(*info.SummationDelivered) * float64(*info.Multiplier) / float64(*info.Divisor)
		info.ActualSummationReceived = float64(*info.SummationReceived) * float64(*info.Multiplier) / float64(*info.Divisor)

		eagle.MeterMAC = info.MeterMacId
		srv.eagles[resp.MacID] = eagle

		info.Dump()

		return
	}

	log.Warning("Got unrecognized message")
}
