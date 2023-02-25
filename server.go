package cyprus

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"crosine.com/cyprus/ext_models"
	"crosine.com/cyprus/subsystems"
	"crosine.com/cyprus/transmission"
	"github.com/CrosineEnterprises/ganymede/models"
)

type ServerSignalChannels struct {
	moduleInitChannel  chan []string
	netTransmissionErr chan error
	progSignals        chan os.Signal
	commChannels       *transmission.CommChannels
}

func NewServerSignalChannels(moduleInitChan chan []string) *ServerSignalChannels {
	return &ServerSignalChannels{
		moduleInitChannel:  moduleInitChan,
		netTransmissionErr: make(chan error, 1),
		progSignals:        make(chan os.Signal, 1),
		commChannels:       transmission.NewCommChannels(),
	}
}

type ServerModule struct {
	writeChannel chan models.Message
	nt           *transmission.NetworkTransmissionServer
	mp           subsystems.MediaPlayerSubsystem
	signals      *ServerSignalChannels
}

func NewServerModule() (*ServerModule, error) {
	moduleInitChan := make(chan []string, 20)
	serverWriteChannel := make(chan models.Message)
	serverSignalChannels := NewServerSignalChannels(moduleInitChan)
	return &ServerModule{
		writeChannel: serverWriteChannel,
		nt:           transmission.NewNetworkTransmissionServer(serverWriteChannel, moduleInitChan, serverSignalChannels.commChannels),
		signals:      serverSignalChannels,
		mp:           nil,
	}, nil
}

func (s *ServerModule) Setup() {
	// Interrupt will hit this signal, should make everything
	signal.Notify(s.signals.progSignals, os.Interrupt)

	// -- Setup for any other modules
}

func (s *ServerModule) initializeModule(mods []string) {
	// enabledModules := make([]string, len(mods))
	// errorList, statusList := make([]error, len(mods)), make([]bool, len(mods))
	// for _, mod := range mods {
	// 	switch mod {
	// 	case "ping":
	// 		mPlayer, mPlayerErr := subsystems.NewMediaPlayerSubsystem()
	// 		if mPlayerErr != nil {
	// 			errorList = append(errorList, mPlayerErr)
	// 			statusList = append(statusList, false)
	// 		} else {
	// 			s.mp = mPlayer
	// 			errorList = append(errorList, nil)
	// 			statusList = append(statusList, true)
	// 			enabledModules = append(enabledModules, mod)
	// 		}
	// 	default:
	// 		errorList = append(errorList, fmt.Errorf("could not start %s", mod))
	// 		statusList = append(statusList, false)
	// 	}
	// }
	// return enabledModules, statusList, errorList
}

func (s *ServerModule) routine() {
routineForLoop:
	for {
		select {
		case initModule := <-s.signals.moduleInitChannel:
			fmt.Printf("Initialization recieved: %s\n", initModule)
			s.initializeModule(initModule)
			// TODO: Pushing to this channel blocks the app. Try fixing this issue here.
			s.writeChannel <- models.Message{Method: "rinit", Args: &ext_models.InitModules{EnabledModules: initModule}}
			fmt.Println("After channel")
			continue routineForLoop
		case servErr := <-s.signals.netTransmissionErr:
			log.Printf("NetworkTransmission error: %v", servErr)
			s.signals.netTransmissionErr <- servErr
			break routineForLoop
		case <-s.signals.progSignals:
			log.Printf("Server Stopping\n")
			break routineForLoop
		}
	}
}

func (s *ServerModule) shutdown() {
	shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	if len(s.signals.netTransmissionErr) != 0 {
		log.Printf("server error: %v", <-s.signals.netTransmissionErr)
	}

	// -- NETWORK TRANSMISSION SHUTDOWN
	shutDownErr := s.nt.Shutdown(shutdownContext)
	if shutDownErr != nil {
		log.Fatalf("server shutdown err: %v", shutDownErr)
	}
}

func (s *ServerModule) Run() {
	// -- SETUP
	s.Setup()

	// -- TRANSMISSION MODULE --
	go s.nt.Coroutine(s.signals.netTransmissionErr)

	// -- MEDIAPLAYER MODULE --

	// -- RUN ROUTINE
	s.routine()

	// SHUT DOWN ALL MODULES
	s.shutdown()
}
