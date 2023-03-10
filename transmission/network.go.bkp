package transmission

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"crosine.com/cyprus/comm"
	"crosine.com/cyprus/ext_models"
	"crosine.com/cyprus/subsystems"
	"github.com/CrosineEnterprises/ganymede/models"
	"github.com/CrosineEnterprises/ganymede/models/base"
	"github.com/vmihailenco/msgpack/v5"
	"nhooyr.io/websocket"
)

// TODO: Move this to the ganymede library
// const DEFAULT_PORT = 8000

type NetworkTransmission struct {
	context      context.Context
	serveMux     http.ServeMux
	wsConn       *websocket.Conn
	logf         func(f string, v ...interface{})
	capabilities map[string]bool
	// Media Player Subsystem
	serverSubsystem *subsystems.ServerSubsystems
	mpBiDirChan     *comm.BiDirMessageChannel
}

// -- SETUP METHODS --

func SetupNetworkTransmission() *http.Server {
	// -- NETWORK TRANSMISSION --
	netTrans := NewNetworkTransmission()

	// Initialize and setup HTTP
	server := &http.Server{
		Addr:         "localhost:8000",
		Handler:      netTrans,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}

	return server
}

func NewNetworkTransmission() *NetworkTransmission {
	// mpSub, mpSubError := NewMediaPlayerSubsystem()
	// if mpSubError != nil {
	// 	return nil, mpSubError
	// }
	newNT := &NetworkTransmission{
		logf:            log.Printf,
		capabilities:    map[string]bool{},
		serverSubsystem: subsystems.NewServerSubsystem(),
		mpBiDirChan:     comm.NewBiDirMessageChannel(),
	}
	newNT.serveMux.HandleFunc("/", newNT.HandleWS)
	return newNT
}

// -- SUBSYSTEM METHODS --

// -- NETWORK SERVER METHODS --

func (nt *NetworkTransmission) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	nt.serveMux.ServeHTTP(w, req)
}

func (nt *NetworkTransmission) CloseConnection() {
	log.Printf("Closing connection to client")
	nt.wsConn.Close(websocket.StatusNormalClosure, "")
	nt.wsConn = nil
}

// -- READ/WRITE METHODS --

func (nt *NetworkTransmission) Write(data interface{}) {
	marshaledData, marshalErr := msgpack.Marshal(data)
	if marshalErr != nil {
		log.Printf("Write: MarshalErr: %v", marshalErr)
	}
	wsWriteErr := nt.wsConn.Write(nt.context, websocket.MessageBinary, marshaledData)
	if wsWriteErr != nil {
		log.Printf("Write: WSWriteErr: %v", wsWriteErr)
	}
}

// -- WEBSOCKET-SPECIFIC METHODS --

func (nt *NetworkTransmission) switchToWebsockets(w http.ResponseWriter, req *http.Request) (*websocket.Conn, error) {
	return websocket.Accept(w, req, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
}

func (nt *NetworkTransmission) HandleWS(w http.ResponseWriter, req *http.Request) {
	if nt.wsConn != nil {
		// Handle 400 Status Code (Something to describe server is already connected)
		return
	}
	// Upgrade to WebSockets
	wsConn, wsConnErr := nt.switchToWebsockets(w, req)
	if wsConnErr != nil {
		nt.logf("WebSocket Accept Err: %v", wsConnErr)
		return
	}
	// On failure, send back internal error
	defer wsConn.Close(websocket.StatusInternalError, "close")

	nt.wsConn = wsConn

	// Get background context
	nt.context = context.Background()

	// Run Read Loop
	nt.ReadLoop()
}

// -- WEBSOCKET SPECIFIC READ/WRITE LOOPS
func (nt *NetworkTransmission) ReadLoop() {
	// Readloop
	close := false
	for !close {
		// if errors.Is(wsConnErr, context.Canceled) {
		// 	return
		// }
		// Read from WS
		_, data, readErr := nt.wsConn.Read(nt.context)
		if readErr != nil {
			// If the server is closed normally, break the loop
			if websocket.CloseStatus(readErr) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(readErr) == websocket.StatusGoingAway {
				break
			}
			log.Printf("WS Error: %v", readErr)
			close = true
			break
		}

		// Create a decoder for the data
		decoder := msgpack.NewDecoder(bytes.NewReader(data))

		// Find the length of the array. If not correct, error out
		decodeErr := validateDecoder(decoder)
		if decodeErr != nil {
			nt.logf("validateDecoderErr: %v", decodeErr)
			break
		}

		// Command must be the first element. Let's see
		methodString, methodDecodeErr := decoder.DecodeString()
		if methodDecodeErr != nil {
			log.Println(methodDecodeErr)
		}

		fmt.Printf("METHOD: %s\n", methodString)

		subsystem, method, subMethodExists := strings.Cut(methodString, ":")

		// If subsystem isn't given, run it here
		if !subMethodExists {
			subsystem = method
		}

		// Go through each method to find what is the right one
		switch subsystem {
		case "ping":
			// Process the ping request
			ping, pingErr := ext_models.ProcessInit(nt.wsConn, decoder)
			if pingErr != nil {
				log.Printf("Ping Err: %v\n", pingErr)
			}
			enabledCapabilities := []string{}
			for _, v := range ping.Capabilities {
				nt.capabilities[v] = false
				if v == "mp" {
					mpSetupErr := nt.serverSubsystem.SetupMediaPlayer(nt.mpBiDirChan)
					if mpSetupErr != nil {
						log.Printf("MediaPlayerSetup: %v\n", mpSetupErr)
					} else {
						fmt.Println("MediaPlayer Set up")
						enabledCapabilities = append(enabledCapabilities, "mp")
					}
				}
			}
			go nt.Write(&models.Message{Method: "rping", Args: base.NewRPing(enabledCapabilities)})
		case "mp":
			if !subMethodExists {
				log.Printf("Submethod doesn't exist!")
			}
		case "close":
			fmt.Println("Close Instruction Recieved. Closing")
			nt.wsConn.Write(nt.context, websocket.MessageText, []byte("close"))
			close = true
		}
	}
	if close {
		nt.CloseConnection()
	}
}

func RunServer() {
	// -- TRANSMISSION MODULE --
	// Server Error Flag
	isServerErrored := false
	// Initialize and setup Network Transmission
	server := SetupNetworkTransmission()
	// Server Error Channel
	servErrChnl := make(chan error, 1)
	// Run Server
	go func() {
		// Catch the error and send it through the error channel
		log.Print("Attempting to start server")
		// servErrChnl <- server.Serve(netListener)
		servErrChnl <- server.ListenAndServe()
	}()

	// -- TODO: ADD OTHER MODULES HERE --

	// -- ADD SIGNAL HANDLER --
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// -- WAIT FOR EVENTS, WHICHEVER SIGNAL COMES EARLIER --
	select {
	case serverErr := <-servErrChnl:
		log.Printf("Failed to serve %v", serverErr)
		isServerErrored = true
	case <-sigChan:
		log.Printf("Stopping server")
	}

	// -- SHUTDOWNS --
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// - Server Shutdown
	shutdownErr := server.Shutdown(ctx)
	if shutdownErr != nil {
		log.Fatalf("Server shutdown err: %v", shutdownErr)
	}
	// Failed exit if server error
	if isServerErrored {
		os.Exit(1)
	}
}

// -- OTHER UTILITY METHODS
// TODO: Move this to "utils" or similar
func validateDecoder(decoder *msgpack.Decoder) error {
	arrLen, arrayLenErr := decoder.DecodeArrayLen()
	if arrayLenErr != nil {
		return arrayLenErr

	}
	if arrLen < 2 {
		log.Println("WARN: Method only, no arguments")
	}
	return nil
}
