package cyprus

/**
Cyprus: Server Application

This is the main server application method that will be run when program is started.

ServerModule is the "main" server structure, and Run() method runs the server.

Copyright (C) 2023 Goutham Krishna K V
*/

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"crosine.com/cyprus/comm"
	"crosine.com/cyprus/subsystems"
	"crosine.com/cyprus/transmission"
	"crosine.com/cyprus/utils"
	"github.com/CrosineEnterprises/ganymede/models"
	"github.com/CrosineEnterprises/ganymede/models/base"
)

// Default server port
const DefaultPort = 3969

type ServerSignalChannels struct {
	moduleInitChannel  chan []string
	moduleCloseChannel chan bool
	netTransmissionErr chan error
	progSignals        chan os.Signal
	commChannels       *comm.CommChannels
}

func NewServerSignalChannels(
	moduleInitChan chan []string,
	moduleCloseChan chan bool,
) *ServerSignalChannels {
	return &ServerSignalChannels{
		moduleInitChannel:  moduleInitChan,
		moduleCloseChannel: moduleCloseChan,
		netTransmissionErr: make(chan error, 1),
		progSignals:        make(chan os.Signal, 1),
		commChannels:       comm.NewCommChannels(),
	}
}

type ServerModule struct {
	serverPort   int
	logf         func(string, ...interface{})
	writeChannel chan models.Message
	nt           *transmission.NetworkTransmissionServer
	mp           subsystems.MediaPlayerSubsystem
	nd           *subsystems.NetworkDiscovery
	signals      *ServerSignalChannels
}

func NewServerModule(port int) (*ServerModule, error) {
	moduleInitChan := make(chan []string, 20)
	moduleCloseChan := make(chan bool)
	serverWriteChannel := make(chan models.Message)
	serverSignalChannels := NewServerSignalChannels(moduleInitChan, moduleCloseChan)
	logf := func(s string, i ...interface{}) {
		utils.LogFunc("SRV", s, i...)
	}
	if port == 0 {
		logf("Default port value requested. Port = %d", DefaultPort)
		port = DefaultPort
	}
	return &ServerModule{
		serverPort:   port,
		logf:         logf,
		writeChannel: serverWriteChannel,
		nt: transmission.NewNetworkTransmissionServer(
			serverWriteChannel,
			moduleInitChan,
			moduleCloseChan,
			serverSignalChannels.commChannels,
			port,
		),
		signals: serverSignalChannels,
		// - Modules
		// 1. mp: Media Player
		mp: nil,
		// 2. nd: Network Discovery
		nd: nil,
	}, nil
}

func (s *ServerModule) setup() {
	// Interrupt will hit this signal, should make everything
	signal.Notify(s.signals.progSignals, os.Interrupt)

	// -- NETWORK DISCOVERY (this module needs to be set-up on launch so that
	//	it can be discovered by other devices over the network).
	networkDiscoveryModule, ndErr := subsystems.NewNetworkDiscovery(s.serverPort)
	if ndErr != nil {
		s.logf("NetworkDiscoveryError: %v", ndErr)
	} else {
		s.nd = networkDiscoveryModule
		s.logf("Advertising server capabilities through the network...")
	}

	// -- Setup for any other modules
}

func (s *ServerModule) initializeModule(mods []string) []string {
	enabledModules := []string{}
	for _, mod := range mods {
		// TODO: find a better way to transfer the errors
		s.logf("Enabling modules: %s\n", mod)
		switch mod {
		case "mp":
			// Initialize new media player
			mPlayer, mPlayerErr := subsystems.NewMediaPlayerSubsystem(&s.signals.commChannels.MPChannel)
			if mPlayerErr != nil {
				s.logf("mPlayerErr: %s", mPlayerErr)
			} else {
				s.mp = mPlayer
				// Setup and run coroutine
				s.mp.Setup()
				go s.mp.Routine()
				enabledModules = append(enabledModules, mod)
			}
		}
	}
	return enabledModules
}

func (s *ServerModule) closeModule() {
	// -- MEDIA PLAYER
	if s.mp != nil {
		s.logf("Stopping MediaPlayer")
		s.mp.Shutdown()
		s.mp = nil
	}
	// Network Discovery
	if s.nd == nil {
		s.logf("Restarting NetworkDiscovery")
		ndServer, ndServErr := subsystems.NewNetworkDiscovery(s.serverPort)
		if ndServErr != nil {
			s.logf("closeModule(networkDiscovery) Error: %v", ndServErr)
		} else {
			s.nd = ndServer
		}
	}
}

func (s *ServerModule) routine() {
	// -- TRANSMISSION MODULE --
	go s.nt.Coroutine(s.signals.netTransmissionErr)
	// Stop Network discovery as soon as connection is established.
routineForLoop:
	for {
		select {
		// Module Initialization Channel
		case initModule := <-s.signals.moduleInitChannel:
			initializedModules := s.initializeModule(initModule)
			s.logf("Initializing Modules : %s\n", initializedModules)
			s.writeChannel <- *base.NewInitWithCapabilities(initializedModules).GenMessage("rinit")
			// Stop network discovery so nobody else can see this server after connection
			if s.nd != nil {
				s.logf("Stopping Network Discovery")
				s.nd.Shutdown()
				s.nd = nil
			}
			continue routineForLoop
		// Module Close Channel
		case <-s.signals.moduleCloseChannel:
			s.logf("close triggered")
			s.closeModule()
		// If the server encounters an error
		case servErr := <-s.signals.netTransmissionErr:
			s.logf("NetworkTransmission error: %v", servErr)
			s.signals.netTransmissionErr <- servErr
			break routineForLoop
		// When Interrupt Calls are Sent
		case <-s.signals.progSignals:
			s.logf("Stopping")
			break routineForLoop
		}
	}
}

func (s *ServerModule) shutdown() {
	shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	if len(s.signals.netTransmissionErr) != 0 {
		s.logf("server error: %v", <-s.signals.netTransmissionErr)
	}

	// -- MEDIA PLAYER SHUTDOWN
	if s.mp != nil {
		s.mp.Shutdown()
	}

	// -- NETWORK TRANSMISSION SHUTDOWN
	shutDownErr := s.nt.Shutdown(shutdownContext)
	if shutDownErr != nil {
		log.Fatalf("server shutdown err: %v", shutDownErr)
	}

	// -- NETWORK DISCOVERY SHUTDOWN
	if s.nd != nil {
		s.logf("Shutting down Network Discovery")
		s.nd.Shutdown()
		s.nd = nil
	}
}

func (s *ServerModule) Run() {
	// -- SETUP
	s.setup()

	// -- RUN ROUTINE
	s.routine()

	// SHUT DOWN ALL MODULES
	s.shutdown()
}
