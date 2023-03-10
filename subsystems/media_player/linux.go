package media_player

import (
	"bytes"
	"log"
	"strings"

	"crosine.com/cyprus/comm"
	"crosine.com/cyprus/utils"
	"github.com/CrosineEnterprises/ganymede/models"
	"github.com/CrosineEnterprises/ganymede/models/mp"
	"github.com/Pauloo27/go-mpris"
	"github.com/godbus/dbus/v5"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	dbusObjectPath = "/org/mpris/MediaPlayer2"
)

type LinuxMediaPlayerSubsystem struct {
	logf         func(string, ...interface{})
	bus          *dbus.Conn
	bidirChannel *comm.BiDirMessageChannel
	// Specific requirements
	signalLoopBreak    chan bool
	signalMediaChanged chan *dbus.Signal
	// Linux-specific operations
	players       []*mpris.Player
	senders       []string
	playerSigChan chan *dbus.Signal
}

func NewLinuxMediaPlayerSubsystem(bidirChan *comm.BiDirMessageChannel) *LinuxMediaPlayerSubsystem {
	return &LinuxMediaPlayerSubsystem{
		logf: func(s string, i ...interface{}) {
			log.Printf("MP: "+s, i...)
		},
		bidirChannel:       bidirChan,
		signalLoopBreak:    make(chan bool),
		signalMediaChanged: make(chan *dbus.Signal),
		players:            []*mpris.Player{},
		senders:            []string{},
		playerSigChan:      make(chan *dbus.Signal, 5),
	}
}

func (lmp *LinuxMediaPlayerSubsystem) findSender(sender string) (int, bool) {
	for i, val := range lmp.senders {
		if val == sender {
			return i, true
		}
	}
	return 0, false
}

func (lmp *LinuxMediaPlayerSubsystem) AddPlayers() error {
	mediaPlayerNames, playerListErr := mpris.List(lmp.bus)
	if playerListErr != nil {
		return playerListErr
	}
	lmp.bus.Signal(lmp.playerSigChan)
	for _, mPlayerName := range mediaPlayerNames {
		// Check this first
		player := mpris.New(lmp.bus, mPlayerName)
		if player != nil {
			lmp.bus.AddMatchSignal(
				dbus.WithMatchSender(mPlayerName),
				dbus.WithMatchObjectPath(lmp.bus.Object(mPlayerName, dbusObjectPath).(*dbus.Object).Path()),
				dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
			)
			// lmp.bus.Signal(lmp.playerSigChan)
			// We're quickly starting & pausing so that we can find which sender it is
			player.PlayPause()
			lmp.senders = append(lmp.senders, (<-lmp.playerSigChan).Sender)
			player.PlayPause()
			// Throw away the value (from flipping-back) from the channel
			<-lmp.playerSigChan
			lmp.players = append(lmp.players, player)
		}
	}
	return nil
}

func (lmp *LinuxMediaPlayerSubsystem) ResetPlayers() {
	// newPlayerList, playerListErr := mpris.List(lmp.bus)
	lmp.logf("RESET TRIGGERED")
}

func (lmp *LinuxMediaPlayerSubsystem) Setup() error {
	busConn, sessionBusErr := dbus.SessionBus()
	if sessionBusErr != nil {
		return sessionBusErr
	}
	lmp.bus = busConn

	// Add signal for
	dbusConnAddSignalErr := lmp.bus.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
	)
	if dbusConnAddSignalErr != nil {
		lmp.logf("Setup: ConnAddError: %v", dbusConnAddSignalErr)
	}

	// lmp.bus.Signal(lmp.signalMediaChanged)
	lmp.AddPlayers()
	lmp.logf("Players added: %d", len(lmp.players))
	lmp.logf("Senders: %s", lmp.senders)
	return nil
}

func (l *LinuxMediaPlayerSubsystem) List() ([]string, error) {
	return mpris.List(l.bus)
}

func (lmp *LinuxMediaPlayerSubsystem) SignalLoop() {
	lmp.logf("Signal Loop: Start")
signalLoop:
	for {
		select {
		case value := <-lmp.playerSigChan:
			playerIdx, _ := lmp.findSender(value.Sender)
			lmp.logf("Name: %s", value.Name)
			// Checking if it is DBus variant value
			switch value.Name {
			case "org.freedesktop.DBus.Properties.PropertiesChanged":
				lmp.logf("properties changed")
				properties, dbusVarOk := value.Body[1].(map[string]dbus.Variant)
				// DBus Variant: YES
				if dbusVarOk {
					if playbackStatus, ok := properties["PlaybackStatus"]; ok {
						playPauseStatus := playbackStatus.Value().(string)
						lmp.logf("Player Play/Pause: %s", playPauseStatus)
						lmp.bidirChannel.OutChannel <- models.Message{
							Method: "mp:rplayerstatus",
							Args: &mp.MPlayerStatus{
								PlayStatus:  playPauseStatus,
								PlayerIndex: playerIdx,
							},
						}
					}
					if _, ok := properties["Metadata"]; ok {
						metadata, mprisErr := lmp.players[playerIdx].GetMetadata()
						if mprisErr != nil {
							lmp.logf("mprisErr: %v", mprisErr)
						}
						mplayerMeta := mp.MediaPlayerFromMpris(metadata)
						lmp.logf("Metadata: %v", mplayerMeta)
						lmp.bidirChannel.OutChannel <- models.Message{
							Method: "mp:metadata",
							Args:   &mplayerMeta,
						}
					}
				} else {
					lmp.logf("Args: %v", value.Body)
				}
			case "org.freedesktop.DBus.NameOwnerChanged":
				lmp.logf("Name owner changed. (TODO: Read more data)")
				// String values
				lmp.logf("[NOWNCH] Body: %v", value.Body)
				lmp.logf("ARG0: %s", value.Body[0])
				if len(value.Body) == 2 {
					lmp.logf("ARG1: %s", value.Body[1])
				}
				if len(value.Body) > 2 {
					lmp.logf("ALL: %v", value.Body)
				}
				// strVal, strOk := value.Body[1].(string)
				// // String: YES
				// if strOk {
				// 	if i, ok := lmp.findSender(strVal); ok {
				// 		lmp.logf("Player %d CHANGED", i)
				// 	} else {
				// 		lmp.logf("New sender: %s", strVal)
				// 	}
				// }
			}
		case <-lmp.signalLoopBreak:
			break signalLoop
		}
	}
}

func (l *LinuxMediaPlayerSubsystem) Routine() {
	l.logf("Starting")
	if l.bidirChannel.InChannel == nil || l.bidirChannel.OutChannel == nil {
		return
	}
	// Run the signal loop to send the change events to client.
	go l.SignalLoop()
	// Run the routine to pass in commands to validate values
lmpForRoutine:
	for {
		select {
		case readData := <-l.bidirChannel.InChannel:
			// This read channel will recieve the and will run actions which are deemed required
			decoder := msgpack.NewDecoder(bytes.NewReader(readData))
			// Validate Array-based Msgpack-RPC (by checking array length)
			payloadErr := utils.ValidateDecoder(decoder)
			if payloadErr != nil {
				l.logf("payloadErr: %v", payloadErr)
			}

			methodData, decodeErr := decoder.DecodeString()
			if decodeErr != nil {
				l.logf("decodeErr: %v", decodeErr)
			}

			methodWithoutValue, method, methodExists := strings.Cut(methodData, ":")
			if !methodExists {
				l.logf("method doesn't exist")
				method = methodWithoutValue
			}
			switch method {
			case "close":
				break lmpForRoutine
			case "list":
				players, playerListErr := l.List()
				l.logf("list: %v", players)
				if playerListErr != nil {
					l.logf("playerListErr: %v", playerListErr)
				} else {
					l.logf("Players: %s", players)
					l.bidirChannel.OutChannel <- models.Message{Method: "mp:rlist", Args: &mp.MPlayerList{Players: players}}
				}
			case "play":
				var mpPlayVal mp.MPlayerPlay
				mpParseErr := decoder.Decode(&mpPlayVal)
				if mpParseErr != nil {
					l.logf("Parse error: %v", mpParseErr)
				}
				l.logf("Play on Player %d\n", mpPlayVal.PlayerIndex)
				if len(l.players) > mpPlayVal.PlayerIndex {
					l.players[mpPlayVal.PlayerIndex].Play()
				}
			case "pause":
				var mpPauseArgument mp.MPlayerPlay
				mpParseErr := decoder.Decode(&mpPauseArgument)
				if mpParseErr != nil {
					l.logf("Pause::parseErr: %v", mpParseErr)
				}
				l.logf("Pause on Player %d", mpPauseArgument.PlayerIndex)
				if len(l.players) > mpPauseArgument.PlayerIndex {
					// TODO: Error handling
					// Pause the player
					l.players[mpPauseArgument.PlayerIndex].Pause()
				}
				l.bidirChannel.OutChannel <- models.Message{Method: "mp:rpause", Args: nil}
			case "playpause":
				var mpPlayPause mp.MPlayerPlay
				mpParseError := decoder.Decode(&mpPlayPause)
				if mpParseError != nil {
					l.logf("Playpause::parseErr: %v", mpParseError)
				}
				l.logf("Play/Pause on player %d", mpPlayPause.PlayerIndex)
				if len(l.players) > mpPlayPause.PlayerIndex {
					// TODO: Error handling
					// Play-pause the player
					l.players[mpPlayPause.PlayerIndex].PlayPause()
					// processAndSend(l.bidirChannel.OutChannel, "mp:rplaypause", playPauseErr)
				}
			case "fwd":
				var mpFwdArgument mp.MPlayerPlay
				mpParseErr := decoder.Decode(&mpFwdArgument)
				if mpParseErr != nil {
					l.logf("fwd::parseErr: %v", mpParseErr)
				}
				l.logf("Fwd on player %d", mpFwdArgument.PlayerIndex)
				if len(l.players) > mpFwdArgument.PlayerIndex {
					// TODO: Error handling
					// Click "next" on the player
					l.players[mpFwdArgument.PlayerIndex].Next()
					// processAndSend(l.bidirChannel.OutChannel, "rfwd", fwdErr)
				}
			case "prv":
				var mpPrvArgument mp.MPlayerPlay
				mpParseErr := decoder.Decode(&mpPrvArgument)
				if mpParseErr != nil {
					l.logf("prv::parseErr: %v", mpParseErr)
				}
				l.logf("Prv on player %d", mpPrvArgument.PlayerIndex)
				if len(l.players) > mpPrvArgument.PlayerIndex {
					// TODO: Error handling
					// Click "previous" on the player
					l.players[mpPrvArgument.PlayerIndex].Previous()
					// processAndSend(l.bidirChannel.OutChannel, "rprv", prvErr)
				}
			default:
				l.logf("Method: %s unimplemented", method)
			}
		case moduleCommand := <-l.bidirChannel.CommandChannel:
			// If there's any other commands, put here
			switch moduleCommand {
			case "close":
				break lmpForRoutine
			}
		}
	}
	l.logf("Stopping")
}

func (l *LinuxMediaPlayerSubsystem) Shutdown() {
	l.bidirChannel.CommandChannel <- "close"
	for _, player := range l.players {
		player.Quit()
	}
	l.signalLoopBreak <- false
	l.players = []*mpris.Player{}
	l.senders = []string{}
}
