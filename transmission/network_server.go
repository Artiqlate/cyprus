package transmission

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"crosine.com/cyprus/comm"
	"crosine.com/cyprus/ext_models"
	"github.com/CrosineEnterprises/ganymede/models"
	"github.com/vmihailenco/msgpack/v5"
	"nhooyr.io/websocket"
)

// TODO: MOVE TO GANYMEDE, SUBJECT TO CHANGE
const DEFAULT_PORT = 8000

type NetworkTransmissionServer struct {
	moduleInitChan  chan []string
	moduleCloseChan chan bool
	context         context.Context
	httpServer      *http.Server
	serveMux        http.ServeMux
	wsConn          *websocket.Conn
	writeChannel    chan models.Message
	commChannels    *comm.CommChannels
	logf            func(f string, v ...interface{})
}

// -- CONSTRUCTOR
func NewNetworkTransmissionServer(writeChannel chan models.Message, moduleInitChan chan []string, moduleCloseChan chan bool, commChannels *comm.CommChannels) *NetworkTransmissionServer {
	newNT := &NetworkTransmissionServer{
		moduleInitChan:  moduleInitChan,
		moduleCloseChan: moduleCloseChan,
		commChannels:    commChannels,
		writeChannel:    writeChannel,
		logf:            log.Printf,
	}
	newNT.serveMux.HandleFunc("/", newNT.WebsocketHandler)
	return newNT
}

// -- COROUTINE FOR SERVER
func (nt *NetworkTransmissionServer) Coroutine(errChan chan error) {
	nt.logf("Attempting to start server")
	errChan <- nt.Serve()
}

// -- DATA DECODE AND PARSING
func (nt *NetworkTransmissionServer) decodeData(data []byte) error {
	fmt.Printf("decodeData\n")
	// Initialize the decoder object
	decoder := msgpack.NewDecoder(bytes.NewReader(data))

	// Decode length of the array. If it's less than 2, error out.
	arrLen, arrLenErr := decoder.DecodeArrayLen()
	if arrLenErr != nil {
		return arrLenErr
	}
	if arrLen < 2 {
		log.Println("WARN: Method only, no arguments")
	}

	// Command must be the first element
	methodAndSubsystem, msDecodeErr := decoder.DecodeString()
	if msDecodeErr != nil {
		nt.logf("method decode: %v", msDecodeErr)
	}

	subsystem, method, subsystemMethodExists := strings.Cut(methodAndSubsystem, ":")
	fmt.Printf("Subsystem: %s, Method: %s\n", subsystem, method)

	switch subsystem {
	// Add all subsystem-based methods here
	case "mp":
		if !subsystemMethodExists {
			log.Printf("mp:%s method doesn't exist", method)
		}
		// Pass the data directly as the decoder has internal state we don't
		// want to work with, in other coroutines
		nt.commChannels.MPChannel.InChannel <- data
	case "init":
		init, initErr := ext_models.ProcessInit(nt.wsConn, decoder)
		if initErr != nil {
			nt.logf("Ping err: %v\n", initErr)
		}
		// Send it to main module for processing
		nt.moduleInitChan <- init.Capabilities
	case "close":
		fmt.Println("CLOSE command received from remote. Server Closing")
		return nil
	}
	// Remove this later
	return nil
}

func (nt *NetworkTransmissionServer) write(msgData models.Message) error {
	// Marshal the given message to data
	encodedData, marshalErr := msgpack.Marshal(&msgData)
	if marshalErr != nil {
		return marshalErr
	}
	fmt.Printf("%x\n", encodedData)
	// Encode and send the binary data through WS
	return nt.wsConn.Write(nt.context, websocket.MessageBinary, encodedData)
}

// -- HTTP SPECIFIC --

// -- Start Server
func (nt *NetworkTransmissionServer) Serve() error {
	// return http.ListenAndServe(fmt.Sprintf(":%d", DEFAULT_PORT), &nt.serveMux)
	nt.httpServer = &http.Server{
		Handler:      &nt.serveMux,
		Addr:         fmt.Sprintf(":%d", DEFAULT_PORT),
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}

	return nt.httpServer.ListenAndServe()
}

// -- Shutdown Server
func (nt *NetworkTransmissionServer) Shutdown(context context.Context) error {
	return nt.httpServer.Shutdown(context)
}

// -- WEBSOCKET-SPECIFIC --

// - UPGRADE TO WS
func (nt *NetworkTransmissionServer) upgradeToWebsockets(w http.ResponseWriter, req *http.Request) error {
	if nt.wsConn != nil {
		http.Error(w, "Server already connected, cannot accept more connections.", http.StatusLocked)
		return fmt.Errorf("connection already established")
	}
	wsConn, wsConnAcceptErr := websocket.Accept(w, req, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if wsConnAcceptErr != nil {
		return fmt.Errorf("wsConnAcceptErr %v", wsConnAcceptErr)
	}
	nt.wsConn = wsConn
	return nil
}

// - WEBSOCKET CLOSE
func (nt *NetworkTransmissionServer) wsClose(statusCode websocket.StatusCode, reason string) {
	if nt.wsConn != nil {
		nt.logf("WS Connection Closing")
		nt.wsConn.Close(statusCode, reason)
		nt.wsConn = nil
	}
}

// - TODO: WS REQUEST HANDLER
func (nt *NetworkTransmissionServer) WebsocketHandler(w http.ResponseWriter, req *http.Request) {
	// Upgrade to websockets if possible
	wsUpgrdErr := nt.upgradeToWebsockets(w, req)
	if wsUpgrdErr != nil {
		nt.logf("WS Upgrade Error: %v", wsUpgrdErr)
	}
	// on WS Error
	defer nt.wsClose(websocket.StatusInternalError, "SERVER ERROR")

	// Setup function context
	nt.context = context.Background()

	// Run Write Loop
	// TODO: Add synchronization if needed
	go nt.writeLoop()

	// Read loop
	readErr := nt.readLoop()
	nt.moduleCloseChan <- true
	if readErr != nil {
		log.Printf("Read Error: %v", readErr)
	} else {
		nt.wsClose(websocket.StatusNormalClosure, "THANK YOU")
	}
}

// -- READ AND WRITE LOOPS

func (nt *NetworkTransmissionServer) readLoop() error {
	for {
		_, data, readErr := nt.wsConn.Read(nt.context)
		nt.logf("readLoop:DATA: %x", data)
		if readErr != nil {
			if websocket.CloseStatus(readErr) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(readErr) == websocket.StatusGoingAway {
				break
			}
			return readErr
		}
		decodeErr := nt.decodeData(data)
		if decodeErr != nil {
			return decodeErr
		}
	}
	return nil
}

func (nt *NetworkTransmissionServer) writeLoop() {
	for nt.wsConn != nil {
		select {
		case writeObject := <-nt.writeChannel:
			nt.write(writeObject)
		case mpObject := <-nt.commChannels.MPChannel.OutChannel:
			nt.write(mpObject)
		}
	}
}
