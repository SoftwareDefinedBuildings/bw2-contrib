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
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/gtfierro/spawnpoint/spawnable"
	"github.com/op/go-logging"
	"github.com/xyproto/permissionbolt"
	"github.com/xyproto/pinterface"
	"golang.org/x/crypto/acme/autocert"
	bw2 "gopkg.in/immesys/bw2bind.v5"
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

type permissionHandler struct {
	perm pinterface.IPermissions
	mux  *http.ServeMux
}

// Implement the ServeHTTP method to make a permissionHandler a http.Handler
func (ph *permissionHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Check if the user has the right admin/user rights
	if ph.perm.Rejected(w, req) {
		// Let the user know, by calling the custom "permission denied" function
		ph.perm.DenyFunction()(w, req)
		// Reject the request
		return
	}
	// Serve the requested page if permissions were granted
	ph.mux.ServeHTTP(w, req)
}

type EagleServer struct {
	// MAC address -> eagle instance
	eagles     map[string]*Eagle
	eagleLock  sync.RWMutex
	multiplier float64

	// HTTPS server
	address   string
	hostname  string
	tlshost   string
	userstate *permissionbolt.UserState
	user      string
	secretkey []byte

	// bosswave
	bwclient *bw2.BW2Client
	vk       string
}

func StartEagleServer() {
	server := &EagleServer{
		eagles:   make(map[string]*Eagle),
		bwclient: bw2.ConnectOrExit(""),
	}

	mux := http.NewServeMux()
	perm, err := permissionbolt.New()
	if err != nil {
		log.Fatal(err)
	}
	// Custom handler for when permissions are denied
	perm.SetDenyFunction(func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, "Permission denied!", http.StatusForbidden)
	})

	//perm.Clear() // -- no default permissions
	server.userstate = perm.UserState().(*permissionbolt.UserState)
	server.userstate.SetCookieTimeout(120) // 2 min

	params := spawnable.GetParamsOrExit()
	server.multiplier = float64(params.MustInt("multiplier"))

	// config bw2
	server.bwclient.OverrideAutoChainTo(true)
	server.vk = server.bwclient.SetEntityFileOrExit(params.MustString("entityfile"))

	// add admin user
	user := params.MustString("user")
	server.user = user
	pass := params.MustString("pass")
	server.userstate.AddUser(user, pass, "") // blank email
	perm.AddPublicPath("/")
	perm.AddPublicPath("/login")
	perm.AddPublicPath("/eagle")
	perm.AddUserPath("/config")

	server.secretkey = []byte(params.MustString("secretkey"))

	// setup bosswave service
	//server.svc = server.bwclient.RegisterService(baseuri, "s.Eagle")
	//fmt.Println(server.svc.FullURI())

	port := params.MustString("port")
	listenaddr := params.MustString("listenaddress")
	tlshost := params.MustString("tlshost")

	server.address = listenaddr + ":" + port
	if port != "80" {
		server.hostname += ":" + port
	}
	// configure TLS host
	if tlshost != "" {
		server.hostname = tlshost
		port = "443"
		server.address = listenaddr + ":" + port
	} else {
		server.hostname = params.MustString("hostname")
	}
	server.tlshost = params.MustString("tlshost")
	address, err := net.ResolveTCPAddr("tcp4", server.address)
	if err != nil {
		log.Fatalf("Error resolving address %s (%s)", server.address, err.Error())
	}
	mux.HandleFunc("/", server.handleLogin)
	mux.HandleFunc("/login", server.handleLogin)
	mux.HandleFunc("/config", server.handleConfig)
	mux.HandleFunc("/eagle", server.handleData)
	log.Noticef("Starting HTTP Server on %s", server.address)
	if tlshost != "" {
		m := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(tlshost),
			Cache:      autocert.DirCache("certs"),
		}
		s := &http.Server{
			Addr:      address.String(),
			Handler:   &permissionHandler{perm, mux},
			TLSConfig: &tls.Config{GetCertificate: m.GetCertificate},
		}
		log.Fatal(s.ListenAndServeTLS("", ""))
	} else {
		srv := &http.Server{
			Addr:    address.String(),
			Handler: &permissionHandler{perm, mux},
		}
		log.Fatal(srv.ListenAndServe())
	}
}

func (srv *EagleServer) handleLogin(rw http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	// if we have a GET
	if req.Method == http.MethodGet {
		log.Debug("is logged in?")
		if !srv.userstate.IsLoggedIn(srv.user) {
			log.Debug("NO")
			rw.Write(_INDEX)
			return
		} else {
			log.Debug("YES")
			rw.Write(_INDEX)
			return
		}
	} else if req.Method == http.MethodPost {
		// handle login
		if srv.userstate.IsLoggedIn(srv.user) {
			http.Redirect(rw, req, "/config", http.StatusSeeOther)
		}
		// pull values, check using srv.userstate.CorrectPassword(username, pass) => bool
		// then if that's good, then run userstate.Login(rw, username)
		if err := req.ParseForm(); err != nil {
			http.Error(rw, err.Error(), 400)
			return
		}
		user := req.Form.Get("user")
		pass := req.Form.Get("pass")
		if srv.userstate.CorrectPassword(user, pass) {
			log.Debug("correct!")
			log.Error(srv.userstate.Login(rw, user))
			http.Redirect(rw, req, "/config", http.StatusSeeOther)
			rw.Write(_CONFIG)
			return
		} else {
			log.Debug("incorrect...")
			http.Redirect(rw, req, "/login", http.StatusSeeOther)
		}
	}
	http.Redirect(rw, req, "/login", http.StatusSeeOther)
}

func (srv *EagleServer) handleConfig(rw http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	if req.Method == http.MethodGet {
		rw.Write(_CONFIG)
		return
	} else if req.Method == http.MethodPost {
		log.Debug("CONFIG POST")
		if err := req.ParseForm(); err != nil {
			log.Error(err)
			http.Error(rw, err.Error(), 400)
			return
		}
		// now can use req.Form
		// when we get a config form, do the following
		// 1. check that the URI is valid
		// 2. see if we can build a chain onto uri/s.Eagle/*
		// 3. generate some SHA1 hash and save that mapping somewhere (sha1 -> uri)
		baseuri := req.Form.Get("baseuri")
		useuri := baseuri + "/s.Eagle/*"
		log.Debug(useuri, srv.vk)
		if chain, err := srv.bwclient.BuildAnyChain(useuri, "P", srv.vk); err != nil {
			if err.Error() == "No result" {
				rw.Write([]byte(fmt.Sprintf("Chain does not exist on %s to %s", useuri, srv.vk)))
				return
			}
			log.Error(err)
			http.Error(rw, err.Error(), 500)
			return
		} else if chain == nil {
			if err := _RESULT.Execute(rw, map[string]interface{}{"error": "No chain exists"}); err != nil {
				log.Error(err)
				http.Error(rw, err.Error(), 500)
			}
			return
		}

		// generate key
		mac := hmac.New(sha256.New, srv.secretkey)
		mac.Write([]byte(baseuri))
		hash := mac.Sum(nil)
		stringhash := hex.EncodeToString(hash)

		var eagleurl string
		if srv.tlshost != "" {
			eagleurl = fmt.Sprintf("https://%s/eagle?key=%s&baseuri=%s", srv.hostname, stringhash, baseuri)
		} else {
			eagleurl = fmt.Sprintf("http://%s/eagle?key=%s&baseuri=%s", srv.hostname, stringhash, baseuri)
		}

		if err := _RESULT.Execute(rw, map[string]interface{}{"error": "", "baseuri": baseuri, "hash": stringhash, "reporturl": eagleurl}); err != nil {
			http.Error(rw, err.Error(), 500)
		}

		return
	}
}

func (srv *EagleServer) handleData(rw http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	if req.Method == http.MethodPost {

		// get the request URL and verify that its good
		values := req.URL.Query()
		providedhash := values.Get("key")
		baseuri := values.Get("baseuri")

		// check if its authorized
		mac := hmac.New(sha256.New, srv.secretkey)
		mac.Write([]byte(baseuri))
		hash := mac.Sum(nil)
		expectedhash := hex.EncodeToString(hash)
		log.Debug(providedhash, baseuri, expectedhash)
		if providedhash != expectedhash {
			http.Error(rw, "Not a valid key", 400)
			return
		}

		// parse XML
		var resp Response
		dec := xml.NewDecoder(req.Body)
		if err := dec.Decode(&resp); err != nil {
			log.Error(fmt.Sprintf("Could not decode response: %s", err))
			rw.Write([]byte(fmt.Sprintf("Could not decode response: %s", err)))
			rw.WriteHeader(500)
			return
		}
		// TODO: have this method return any configuration struct
		log.Debugf("%+v", resp)
		srv.HandleMessage(resp, baseuri)

		// this is the reply the eagle expects
		rw.Header().Set("Connection", "close")
		rw.Write([]byte{'\n', '\n'})
	} else if req.Method == http.MethodGet {
		rw.Header().Set("Content-Type", "text/html")
		rw.Write(_INDEX)
	}
}

func (srv *EagleServer) HandleMessage(resp Response, baseuri string) {
	// if we haven't seen this eagle before, ignore the message.
	// We only want to register off of the NetworkInfo messages

	// handle registration of a new eagle
	if resp.NetworkInfo != nil {
		log.Info("NETWORK INFO")
		info := resp.NetworkInfo
		srv.eagleLock.Lock()
		defer srv.eagleLock.Unlock()

		var eagle *Eagle
		var found bool

		// create new eeeaaagleeeee
		if eagle, found = srv.eagles[info.DeviceMacId]; !found {
			eagle = &Eagle{}
			// TODO: set metadata on these uris
			eagle.svc = srv.bwclient.RegisterService(baseuri, "s.Eagle")
			eagle.iface = eagle.svc.RegisterInterface(info.DeviceMacId, "i.meter")
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
		srv.eagles[info.DeviceMacId] = eagle

		if !found {
			log.Noticef("Registering new Eagle with MAC %s", eagle.DeviceMAC)
		}

		// TODO: send configuration struct in reply

		return
	}

	// handle meter data
	if resp.InstantaneousDemand != nil {
		info := resp.InstantaneousDemand
		log.Info("INST DEMAND")
		// update the object with the Meter MAC address
		// but only if we've seen the Eagle before; else, drop this
		srv.eagleLock.Lock()
		eagle, found := srv.eagles[info.DeviceMacId]
		srv.eagleLock.Unlock()
		if !found {
			log.Warning("Got Instantaneous demand for unregistered Eagle")
			return
		}

		// adjust the timestamp with the EAGLE Epoch and get the actual kW demand as a float
		eagle.current_time = int64(*info.TimeStamp + HexInt64(EAGLE_EPOCH))
		eagle.current_demand = float64(*info.Demand) * float64(*info.Multiplier) / float64(*info.Divisor)
		eagle.current_demand *= srv.multiplier // extra multiplier

		eagle.MeterMAC = info.MeterMacId
		srv.eagles[info.DeviceMacId] = eagle

		srv.forwardData(eagle)

		return
	}

	if resp.PriceCluster != nil {
		info := resp.PriceCluster
		log.Info("PRICE CLUSTER")
		srv.eagleLock.Lock()
		eagle, found := srv.eagles[info.DeviceMacId]
		srv.eagleLock.Unlock()
		if !found {
			log.Warning("Got price cluster for unregistered Eagle")
			return
		}
		eagle.current_time = int64(*info.TimeStamp + HexInt64(EAGLE_EPOCH))
		eagle.current_price = float64(*info.Price) / math.Pow(10, float64(*info.TrailingDigits))
		eagle.current_tier = int64(*info.Tier)
		srv.eagles[info.DeviceMacId] = eagle

		srv.forwardData(eagle)
		return
	}

	if resp.MessageCluster != nil {
		log.Debugf("MessageCluster %+v", resp.MessageCluster)
		return
	}

	if resp.CurrentSummationDelivered != nil {
		// update the object with the Meter MAC address
		// but only if we've seen the Eagle before; else, drop this
		info := resp.CurrentSummationDelivered
		srv.eagleLock.Lock()
		eagle, found := srv.eagles[info.DeviceMacId]
		srv.eagleLock.Unlock()
		if !found {
			log.Warning("Got price cluster for unregistered Eagle")
			return
		}

		eagle.current_time = int64(*info.TimeStamp + HexInt64(EAGLE_EPOCH))
		eagle.current_summation_delivered = float64(*info.SummationDelivered) * float64(*info.Multiplier) / float64(*info.Divisor)
		eagle.current_summation_received = float64(*info.SummationReceived) * float64(*info.Multiplier) / float64(*info.Divisor)

		eagle.MeterMAC = info.MeterMacId
		srv.eagles[info.DeviceMacId] = eagle

		srv.forwardData(eagle)

		return
	}

	log.Warning("Got unrecognized message")
}

func main() {
	StartEagleServer()
}
