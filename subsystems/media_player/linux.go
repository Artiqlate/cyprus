package media_player

import (
	"bytes"
	"fmt"
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

type MPlayerPlay struct {
	_msgpack    struct{} `msgpack:",as_array"`
	PlayerIndex int
}

type LinuxMediaPlayerSubsystem struct {
	logf         func(string, ...interface{})
	bus          *dbus.Conn
	bidirChannel *comm.BiDirMessageChannel
	// Specific requirements
	signalLoopBreak chan bool
	// Linux-specific operations
	players       []*mpris.Player
	senders       []string
	playerSigChan chan *dbus.Signal
}

func NewLinuxMediaPlayerSubsystem(bidirChan *comm.BiDirMessageChannel) *LinuxMediaPlayerSubsystem {
	return &LinuxMediaPlayerSubsystem{
		logf: func(s string, i ...interface{}) {
			utils.LogFunc("MP", s, i...)
		},
		bidirChannel:    bidirChan,
		signalLoopBreak: make(chan bool),
		players:         []*mpris.Player{},
		senders:         []string{},
		playerSigChan:   make(chan *dbus.Signal, 5),
	}
}

type MPlayerList struct {
	_msgpack struct{} `msgpack:",as_array"`
	Players  []string
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
	playerNames, playerListErr := mpris.List(lmp.bus)
	if playerListErr != nil {
		return playerListErr
	}
	for _, playerName := range playerNames {
		// Check this first
		player := mpris.New(lmp.bus, playerName)
		if player != nil {
			lmp.bus.AddMatchSignal(
				dbus.WithMatchSender(playerName),
				dbus.WithMatchObjectPath(lmp.bus.Object(playerName, dbusObjectPath).(*dbus.Object).Path()),
				dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
			)
			lmp.bus.Signal(lmp.playerSigChan)
			// We're quickly starting & pausing so that we can find which sender it is
			player.PlayPause()
			lmp.senders = append(lmp.senders, (<-lmp.playerSigChan).Sender)
			player.PlayPause()
			// Throw away the value from the channel
			<-lmp.playerSigChan
			lmp.players = append(lmp.players, player)
		}
	}
	return nil
}

func (lmp *LinuxMediaPlayerSubsystem) Setup() error {
	busConn, sessionBusErr := dbus.SessionBus()
	if sessionBusErr != nil {
		return sessionBusErr
	}
	lmp.bus = busConn
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

			properties := value.Body[1].(map[string]dbus.Variant)
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
	go l.SignalLoop()
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
			// TODO: Switch to just "close". Strip the submodule name if exists
			case "mp:close", "close":
				break lmpForRoutine
			case "list":
				l.logf("List")
				players, playerListErr := l.List()
				if playerListErr != nil {
					l.logf("playerListErr: %v", playerListErr)
				} else {
					l.logf("Players: %s", players)
					l.bidirChannel.OutChannel <- models.Message{Method: "mp:rlist", Args: &MPlayerList{Players: players}}
				}
			case "play":
				var mpPlayVal MPlayerPlay
				mpParseErr := decoder.Decode(&mpPlayVal)
				if mpParseErr != nil {
					l.logf("Parse error: %v", mpParseErr)
				}
				l.logf("Play on Player %d\n", mpPlayVal.PlayerIndex)
				if len(l.players) > mpPlayVal.PlayerIndex {
					l.players[mpPlayVal.PlayerIndex].Play()
					// processAndSend(l.bidirChannel.OutChannel, "rplay", playErr)
					// if playErr != nil {
					// 	l.bidirChannel.OutChannel <- models.Message{
					// 		Method: "mp:rplay",
					// 		Args:   &MPlayerStatus{Status: false, PlayerErr: fmt.Sprintf("%v", playErr)},
					// 	}
					// } else {
					// 	l.bidirChannel.OutChannel <- models.Message{
					// 		Method: "mp:rplay",
					// 		Args:   &MPlayerStatus{Status: true, PlayerErr: ""},
					// 	}
					// }
				}
			case "pause":
				var mpPause MPlayerPlay
				mpParseErr := decoder.Decode(&mpPause)
				if mpParseErr != nil {
					l.logf("Pause::parseErr: %v", mpParseErr)
				}
				l.logf("Pause on Player %d", mpPause.PlayerIndex)
				if len(l.players) > mpPause.PlayerIndex {
					// TODO: Error handling
					l.players[mpPause.PlayerIndex].Pause()
					// processAndSend(l.bidirChannel.OutChannel, "rpause", pauseErr)
					// if pauseErr != nil {
					// 	l.bidirChannel.OutChannel <- models.Message{
					// 		Method: "mp:rpause",
					// 		Args:   &MPlayerStatus{Status: false, PlayerErr: fmt.Sprintf("%v", pauseErr)},
					// 	}
					// } else {
					// 	l.bidirChannel.OutChannel <- models.Message{
					// 		Method: "mp:rpause",
					// 		Args:   &MPlayerStatus{Status: true, PlayerErr: ""},
					// 	}
					// }
				}
				l.bidirChannel.OutChannel <- models.Message{Method: "mp:rpause", Args: nil}
			case "playpause":
				var mpPlayPause MPlayerPlay
				mpParseError := decoder.Decode(&mpPlayPause)
				if mpParseError != nil {
					l.logf("Playpause::parseErr: %v", mpParseError)
				}
				l.logf("Play/Pause on player %d", mpPlayPause.PlayerIndex)
				if len(l.players) > mpPlayPause.PlayerIndex {
					// TODO: Error handling
					l.players[mpPlayPause.PlayerIndex].PlayPause()
					// processAndSend(l.bidirChannel.OutChannel, "mp:rplaypause", playPauseErr)
				}
			case "fwd":
				var mpFwd MPlayerPlay
				mpParseErr := decoder.Decode(&mpFwd)
				if mpParseErr != nil {
					l.logf("fwd::parseErr: %v", mpParseErr)
				}
				l.logf("Fwd on player %d", mpFwd.PlayerIndex)
				if len(l.players) > mpFwd.PlayerIndex {
					// TODO: Error handling
					l.players[mpFwd.PlayerIndex].Next()
					// processAndSend(l.bidirChannel.OutChannel, "rfwd", fwdErr)
				}
			case "prv":
				var mpFwd MPlayerPlay
				mpParseErr := decoder.Decode(&mpFwd)
				if mpParseErr != nil {
					l.logf("prv::parseErr: %v", mpParseErr)
				}
				l.logf("Prv on player %d", mpFwd.PlayerIndex)
				if len(l.players) > mpFwd.PlayerIndex {
					// TODO: Error handling
					l.players[mpFwd.PlayerIndex].Previous()
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
	fmt.Println("MP: Stopping")
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
