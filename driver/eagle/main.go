package main

func main() {
	cfg := &Config{
		Port:          "8000",
		ListenAddress: "0.0.0.0",
		TLSHost:       "",
	}
	StartEagleServer(cfg)
}
