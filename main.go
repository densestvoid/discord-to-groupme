package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var configFilename string
	flag.StringVar(&configFilename, "c", "tokens.json", "specifies the name of the config file to use")
	flag.Parse()
	config, err := ReadConfig(configFilename)
	if err != nil {
		fmt.Println("failed to start: ", err)
		return
	}

	app := NewApp(config)
	finsihSig, err := app.Start()
	if err != nil {
		fmt.Println("failed to start: ", err)
		return
	}
	sysSigs := make(chan os.Signal, 1)
	signal.Notify(sysSigs, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sysSigs:
		app.Stop()
	case <-finsihSig:
	}

}
