package main

import (
	"flag"
	"fmt"
	crConfig "github.com/weinberg/concurrencyRunner/pkg/config"
	"github.com/weinberg/concurrencyRunner/pkg/runner"
	"os"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "./config.json", "Config file path. Defaults to `config.json` in the current working directory.")
	flag.Parse()

	fmt.Printf("Concurrency Lab\n")
	fmt.Printf("configFile: %s\n", configFile)

	config, err := crConfig.ReadConfigFile(configFile)

	if err != nil {
		fmt.Printf("Cannot read config file: %s\n", err)
		os.Exit(1)
	}

	err = runner.Run(config)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}
