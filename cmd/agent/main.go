// Package cmd/main is the starting point for Neptune.io agent. This does not run as a service
// so there will be a daemon to lookafter this process.
package main

import (
	"fmt"
	"github.com/neptuneio/agent/cmd"
)

func main() {
	// Run the agent directly.
	errs := make(chan error, 5)
	exitCh := make(chan struct{})

	// Start a go routine to log errors to console so that we can quickly see if there are any issues
	// in starting the agent.
	go func() {
		for {
			err := <-errs
			if err != nil {
				fmt.Print(err)
			}
		}
	}()

	// Start the agent main loop.
	cmd.MainLoop(errs, exitCh)
}
