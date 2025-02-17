// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

// Package services provides mechanisms to start multiple services in the background
package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/cartesi/rollups-node/internal/logger"
)

// A service that runs in the background endlessly until the context is canceled
type Service interface {
	fmt.Stringer

	// Start a service that will run until completion or until the context is
	// canceled
	Start(ctx context.Context) error
}

const DefaultServiceTimeout = 15 * time.Second

// simpleService implements the context cancelation logic of the Service interface
type simpleService struct {
	serviceName string
	binaryName  string
}

func (s simpleService) Start(ctx context.Context) error {
	cmd := exec.Command(s.binaryName)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		logger.Debug.Printf("%v: %v\n", s.String(), ctx.Err())
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			msg := "%v: failed to send SIGTERM to %v\n"
			logger.Error.Printf(msg, s.String(), s.binaryName)
		}
	}()

	err := cmd.Wait()
	if err != nil && cmd.ProcessState.ExitCode() != int(syscall.SIGTERM) {
		return err
	}
	return nil
}

func (s simpleService) String() string {
	return s.serviceName
}

// The Run function serves as a very simple supervisor: it will start all the
// services provided to it and will run until the first of them finishes. Next
// it will try to stop the remaining services or timeout if they take too long
func Run(services []Service) {
	if len(services) == 0 {
		logger.Error.Panic("there are no services to run")
	}

	// start services
	ctx, cancel := context.WithCancel(context.Background())
	exit := make(chan struct{})
	for _, service := range services {
		service := service
		go func() {
			if err := service.Start(ctx); err != nil {
				msg := "main: service '%v' exited with error: %v\n"
				logger.Error.Printf(msg, service.String(), err)
			} else {
				msg := "main: service '%v' exited successfully\n"
				logger.Info.Printf(msg, service.String())
			}
			exit <- struct{}{}
		}()
	}

	// wait for first service to exit
	<-exit

	// send stop message to all other services and wait for them to finish
	// or timeout
	wait := make(chan struct{})
	go func() {
		cancel()
		for i := 0; i < len(services)-1; i++ {
			<-exit
		}
		wait <- struct{}{}
	}()

	select {
	case <-wait:
		logger.Info.Println("main: all services were shutdown")
	case <-time.After(DefaultServiceTimeout):
		logger.Warning.Println("main: exited after timeout")
	}
}

var (
	GraphQLServer Service = simpleService{
		serviceName: "graphql-server",
		binaryName:  "cartesi-rollups-graphql-server",
	}
	Indexer Service = simpleService{
		serviceName: "indexer",
		binaryName:  "cartesi-rollups-indexer",
	}
)
