package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/influxdata/telegraf/plugins/common/shim"
	_ "github.com/pensando/ionic-telegraf-plugin/plugins/inputs/nicctl"
)

var (
	pollInterval = flag.Duration("poll_interval", 1*time.Second, "polling interval for execd")
	configFile   = flag.String("config", "", "path to plugin config file")
)

func main() {
	flag.Parse()

	s := shim.New()

	if *configFile != "" {
		if err := s.LoadConfig(configFile); err != nil {
			fmt.Fprintf(os.Stderr, "error loading config: %s\n", err)
			os.Exit(1)
		}
	}

	if err := s.RunInput(*pollInterval); err != nil {
		fmt.Fprintf(os.Stderr, "error running plugin: %s\n", err)
		os.Exit(1)
	}
}
