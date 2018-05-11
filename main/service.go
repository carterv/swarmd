package main

import (
	"github.com/kardianos/service"
	"log"
	"swarmd/util"
	"path/filepath"
	"io/ioutil"
	"errors"
	"encoding/json"
	"swarmd/tasks"
)

type program struct {
	killFlag bool
	bootstrapHost string
	bootstrapPort int
	encryptionKey string
}

type jsonConfig struct {
	BoostrapHost string
	BootstrapPort int
	EncryptionKey string
}

func (p *program) Start(s service.Service) error {
	// Get the path to the config file
	configPath := filepath.Join(util.GetBasePath(), "config.json")
	// Read from the file
	file, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Printf("Unable to locate config: %s", configPath)
		return errors.New("config not found")
	}
	// Parse the json from the raw byte stream
	config := new(jsonConfig)
	json.Unmarshal(file, config)
	// Copy the config values over to the program struct
	p.bootstrapHost = config.BoostrapHost
	p.bootstrapPort = config.BootstrapPort
	p.encryptionKey = config.EncryptionKey
	log.Printf("Starting node with configuration:")
	if p.bootstrapHost != "" {
		log.Printf("\tBootstrap node: %s:%d", p.bootstrapHost, p.bootstrapPort)
	}
	log.Printf("\tKey: %s", p.encryptionKey)
	// Initialize non-config values in the program struct
	p.killFlag = false
	// Launch the run routine
	go p.run() // Pass config values to p.run()
	return nil
}

func (p *program) run() {
	// Use this as a wrapper around tasks.Run
	tasks.Run(&p.killFlag, p.bootstrapHost, p.bootstrapPort, p.encryptionKey)
}

func (p *program) Stop(s service.Service) error {
	p.killFlag = true
	return nil
}

func main() {
	svcConfig := &service.Config{
		Name: "swarmd",
		DisplayName: "swarmd",
		Description: "Distributed deployment service",
	}

	prg := &program{}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Println("Unable to start service")
		log.Fatal(err)
	}
	logger, err := s.Logger(nil)
	if err != nil {
		log.Println("Unable to create logger")
		log.Fatal(err)
	}
	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
}