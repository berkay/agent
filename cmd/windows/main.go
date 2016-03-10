// Package cmd/main is the starting point for Neptune.io's windows agent, which runs as a service.
package main

import (
	"log"
	"os"

	"github.com/kardianos/service"
	"github.com/neptuneio/agent/cmd"
)

var logger service.Logger

type NeptuneAgent struct {
	exit chan struct{}
}

var (
	// Channel to capture all agent errors. We redirect these to logger to be able to debug
	// agent issues when it's running as windows service.
	errs = make(chan error, 5)
)

func (p *NeptuneAgent) Start(s service.Service) error {
	p.exit = make(chan struct{})

	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}
func (p *NeptuneAgent) run() error {
	logger.Infof("Running NeptuneAgent...")
	return cmd.MainLoop(errs, p.exit)
}

func (p *NeptuneAgent) Stop(s service.Service) error {
	// Any work in Stop should be quick, usually a few seconds at most.
	logger.Info("Stopping NeptuneAgent!")
	close(p.exit)
	return nil
}

func main() {
	argsWithoutProg := os.Args[1:]

	svcConfig := &service.Config{
		Name:        "NeptuneAgent",
		DisplayName: "NeptuneAgent",
		Description: "Service to run Neptune agent.",
	}

	prg := &NeptuneAgent{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}
	logger, err = s.Logger(errs)
	if err != nil {
		log.Fatal(err)
	}

	// Start a GO routine to redirect all errors to logger.
	go func() {
		for {
			err := <-errs
			if err != nil {
				logger.Error(err)
			}
		}
	}()

	if len(argsWithoutProg) != 0 {
		err := service.Control(s, argsWithoutProg[0])
		if err != nil {
			log.Printf("Valid actions: %q\n", service.ControlAction)
			log.Fatal(err)
		}
		return
	}
	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
}
