package cyprus

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"crosine.com/cyprus/comm"
	"crosine.com/cyprus/subsystems"
	"crosine.com/cyprus/transmission"
	"github.com/CrosineEnterprises/ganymede/models"
	"github.com/CrosineEnterprises/ganymede/models/base"
)

type ServerSignalChannels struct {
	moduleInitChannel  chan []string
	moduleCloseChannel chan bool
	netTransmissionErr chan error
	progSignals        chan os.Signal
	commChannels       *comm.CommChannels
}

func NewServerSignalChannels(moduleInitChan chan []string, moduleCloseChan chan bool) *ServerSignalChannels {
	return &ServerSignalChannels{
		moduleInitChannel:  moduleInitChan,
		moduleCloseChannel: moduleCloseChan,
		netTransmissionErr: make(chan error, 1),
		progSignals:        make(chan os.Signal, 1),
		commChannels:       comm.NewCommChannels(),
	}
}

type ServerModule struct {
	logf         func(string, ...interface{})
	writeChannel chan models.Message
	nt           *transmission.NetworkTransmissionServer
	mp           subsystems.MediaPlayerSubsystem
	signals      *ServerSignalChannels
}

func NewServerModule() (*ServerModule, error) {
	moduleInitChan := make(chan []string, 20)
	moduleCloseChan := make(chan bool)
	serverWriteChannel := make(chan models.Message)
	serverSignalChannels := NewServerSignalChannels(moduleInitChan, moduleCloseChan)
	logf := func(s string, i ...interface{}) {
		fmt.Printf("SRV: %s"+s, time.Now().Format(time.RFC3339), i)
	}
	return &ServerModule{
		logf:         logf,
		writeChannel: serverWriteChannel,
		nt:           transmission.NewNetworkTransmissionServer(serverWriteChannel, moduleInitChan, moduleCloseChan, serverSignalChannels.commChannels),
		signals:      serverSignalChannels,
		// Modules: Add modules here. This is "mp", media_player module
		mp: nil,
	}, nil
}

func (s *ServerModule) setup() {
	// Interrupt will hit this signal, should make everything
	signal.Notify(s.signals.progSignals, os.Interrupt)

	// -- Setup for any other modules
}

func (s *ServerModule) initializeModule(mods []string) []string {
	enabledModules := []string{}
	// errorList, statusList := make([]error, len(mods)), make([]bool, len(mods))
	for _, mod := range mods {
		// TODO: find a better way to transfer the errors
		fmt.Printf("Enabling modules: %s\n", mod)
		switch mod {
		case "mp":
			mPlayer, mPlayerErr := subsystems.NewMediaPlayerSubsystem(&s.signals.commChannels.MPChannel)
			if mPlayerErr != nil {
				fmt.Printf("mPlayerErr: %s", mPlayerErr)
			} else {
				s.mp = mPlayer
				// Run media player coroutine
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
		s.mp.Shutdown()
		s.mp = nil
	}
}

func (s *ServerModule) routine() {
routineForLoop:
	for {
		select {
		// Module Initialization Channel
		case initModule := <-s.signals.moduleInitChannel:
			initializedModules := s.initializeModule(initModule)
			s.logf("SRV: Initializing Modules : %s\n", initializedModules)
			// TODO: Pushing to this channel blocks the app. Try fixing this issue here.
			// s.writeChannel <- models.Message{Method: "rinit", Args: base.NewInitFromArgs(initializedModules)}
			s.writeChannel <- *base.NewInitFromArgs(initializedModules).GenMessage("rinit")
			continue routineForLoop
		// Module Close Channel
		case <-s.signals.moduleCloseChannel:
			s.logf("SRV: close triggered")
			s.closeModule()
		// If the server encounters an error
		case servErr := <-s.signals.netTransmissionErr:
			s.logf("NetworkTransmission error: %v", servErr)
			s.signals.netTransmissionErr <- servErr
			break routineForLoop
		// When Interrupt Calls are Sent
		case <-s.signals.progSignals:
			s.logf("SRV: Stopping\n")
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
}

func (s *ServerModule) Run() {
	// -- SETUP
	s.setup()

	// -- TRANSMISSION MODULE --
	go s.nt.Coroutine(s.signals.netTransmissionErr)

	// -- RUN ROUTINE
	s.routine()

	// SHUT DOWN ALL MODULES
	s.shutdown()
}
