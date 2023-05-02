package media_player

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	// 3rd party imports
	"github.com/Pauloo27/go-mpris"
	"github.com/godbus/dbus/v5"
	"github.com/vmihailenco/msgpack/v5"

	// 1st party imports
	"crosine.com/cyprus/comm"
	ext_mp "crosine.com/cyprus/ext_models/mp"
	"crosine.com/cyprus/utils"
	"github.com/CrosineEnterprises/ganymede/models"
	"github.com/CrosineEnterprises/ganymede/models/mp"
	mp_signals "github.com/CrosineEnterprises/ganymede/models/mp/signals"
)

// -- MP:Linux Methods
// TODO: Move to ganymede.
const (
	MethodInit                  = "init"
	MethodSeeked                = "seeked"
	MethodMetadataUpdated       = "mu"
	MethodPlaybackStatusUpdated = "psu"
	MethodRList                 = "rlist"
	// "NameOwnerChanged" Signals
	MethodPlayerCreated = "cr"
	MethodPlayerUpdated = "up"
	MethodPlayerRemoved = "rm"
	// TODO: Switch this to "init" soon
	MethodRSetupMetadata = "rsetup_metadata"
)

// -- DBus Specific Methods

const (
	DBusMPRISPath          = "/org/mpris/MediaPlayer2"
	SeekedMember           = "Seeked"
	PlayerSeekedMemberName = "org.mpris.MediaPlayer2.Player.Seeked"
)

// type PlayerWrap struct {
// 	Player *mpris.Player
// }

// func NewPlayer(bus *dbus.Conn, playerName string) *PlayerWrap {
// 	player := mpris.New(bus, playerName)
// 	return &PlayerWrap{
// 		Player: player,
// 	}
// }

// func (pl *PlayerWrap) Next() error {
// 	return pl.Player.Next()
// }

// func (pl *PlayerWrap) Previous() error {
// 	return pl.Player.Previous()
// }

// func (pl *PlayerWrap) Pause() error {
// 	return pl.Player.Pause()
// }

// func (pl *PlayerWrap) PlayPause() error {
// 	return pl.Player.PlayPause()
// }

// func (pl *PlayerWrap) Stop() error {
// 	return pl.Player.PlayPause()
// }

// func (pl *PlayerWrap) Play() error {
// 	return pl.Player.Play()
// }

// func (pl *PlayerWrap) Seek(offset float64) error {
// 	return pl.Player.Seek(offset)
// }

// func (pl *PlayerWrap) SetPosition(position float64) error {
// 	return pl.Player.SetPosition(position)
// }

type LinuxMediaPlayerSubsystem struct {
	logf         func(string, ...interface{})
	bus          *dbus.Conn
	bidirChannel *comm.BiDirMessageChannel
	// Loop break signal
	signalLoopBreak chan bool
	// Linux-specific operations
	// TODO: Remove playerNames. We'll move this logic to client-side.
	playerNames     []string
	playerMap       map[string]*mpris.Player
	senderPlayerMap map[string]string
	playerSigChan   chan *dbus.Signal
}

func NewLinuxMediaPlayerSubsystem(bidirChan *comm.BiDirMessageChannel) *LinuxMediaPlayerSubsystem {
	return &LinuxMediaPlayerSubsystem{
		logf: func(f string, v ...interface{}) {
			utils.LogFunc("MPL", f, v...)
		},
		bidirChannel:    bidirChan,
		signalLoopBreak: make(chan bool, 1),
		playerSigChan:   make(chan *dbus.Signal, 5),
		// BUILD IT WITH THESE
		playerNames:     []string{},
		playerMap:       make(map[string]*mpris.Player),
		senderPlayerMap: make(map[string]string),
	}
}

// -- UTILITY METHODS --

func (lmp *LinuxMediaPlayerSubsystem) findPlayerAndIndex(signal *dbus.Signal) (string, int, bool) {
	if playerName, playerExists := lmp.senderPlayerMap[signal.Sender]; playerExists {
		for playerIdx, playerVal := range lmp.playerNames {
			if playerVal == playerName {
				return playerName, playerIdx, true
			}
		}
	}
	return "", 0, false
}

func (lmp *LinuxMediaPlayerSubsystem) removePlayerValues(playerToRemove string) {
	delete(lmp.playerMap, playerToRemove)
	senderExists := false
	for senderName, senderVal := range lmp.senderPlayerMap {
		if senderVal == playerToRemove {
			delete(lmp.senderPlayerMap, senderName)
			senderExists = true
		}
	}
	if !senderExists {
		lmp.logf("WARN: SenderPlayer sender not found.")
	}
	playerNameExists := false
	for playerIndex, playerValue := range lmp.playerNames {
		if playerValue == playerToRemove {
			lmp.playerNames = append(
				lmp.playerNames[:playerIndex],
				lmp.playerNames[playerIndex+1:]...,
			)
			playerNameExists = true
		}
	}
	if !playerNameExists {
		lmp.logf("WARN: Player name not found.")
	}
}

// -- MEDIA PLAYER, PLAYER METHODS --

// - Remove Player
func (lmp *LinuxMediaPlayerSubsystem) removePlayer(playerName string) bool {
	if playerToRemove, playerExists := lmp.playerMap[playerName]; playerExists {
		lmp.bus.RemoveMatchSignal(
			dbus.WithMatchSender(playerName),
			dbus.WithMatchObjectPath(lmp.bus.Object(playerName, DBusMPRISPath).(*dbus.Object).Path()),
			dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		)
		// Quit the player
		playerToRemove.Quit()
		// Delete all values
		lmp.removePlayerValues(playerName)
		return true
	}
	return false
}

// - Add Player
func (lmp *LinuxMediaPlayerSubsystem) addPlayer(playerName string, isSetup bool) {
	if lmp.removePlayer(playerName) {
		lmp.logf("WARN: Player previously existed. Removing.")
	}
	// If it's a change signal, allow 1/2 a second delay to let media player
	// set itself up.
	if !isSetup {
		time.Sleep(time.Second / 2)
	}
	// Create a new player
	player := mpris.New(lmp.bus, playerName)
	if player != nil {
		// Register "org.freedesktop.DBus.Properties.PropertiesChanged"
		lmp.bus.AddMatchSignal(
			dbus.WithMatchSender(playerName),
			dbus.WithMatchObjectPath(lmp.bus.Object(playerName, DBusMPRISPath).Path()),
			dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		)
		lmp.logf("PLAYER NAME: %s", playerName)
		// Switch to temporary signal so that this signal won't be listened to,
		// elsewhere.
		tempSignal := make(chan *dbus.Signal, 3)
		if !isSetup {
			lmp.bus.RemoveSignal(lmp.playerSigChan)
		}
		lmp.bus.Signal(tempSignal)
		// Register sender value
		player.PlayPause()
		sender := (<-tempSignal).Sender
		player.PlayPause()
		// Register "org.mpris.MediaPlayer2.Player.Seeked"
		lmp.bus.AddMatchSignal(
			dbus.WithMatchSender(sender),
			dbus.WithMatchObjectPath(lmp.bus.Object(playerName, DBusMPRISPath).Path()),
			dbus.WithMatchInterface(mpris.PlayerInterface),
			dbus.WithMatchMember(SeekedMember),
		)
		<-tempSignal
		// Switch it back to the other signal.
		lmp.bus.RemoveSignal(tempSignal)
		if !isSetup {
			lmp.bus.Signal(lmp.playerSigChan)
		}

		// Store the players and senders
		lmp.playerMap[playerName] = player
		lmp.senderPlayerMap[sender] = playerName
		lmp.playerNames = append(lmp.playerNames, playerName)
	}
}

// --- SETUP METHODS ---

// This adds all players for for setting up.
// TODO: Rewrite this entire method.
func (lmp *LinuxMediaPlayerSubsystem) setupAddPlayers() error {
	mediaPlayerNames, playerListErr := mpris.List(lmp.bus)
	if playerListErr != nil {
		return playerListErr
	}
	var setupStatuses []mp.Status
	for i, mPlayerName := range mediaPlayerNames {
		lmp.addPlayer(mPlayerName, true)
		// Get playback status
		plStatus, statusErr := lmp.playerMap[mPlayerName].GetPlaybackStatus()
		if statusErr != nil {
			lmp.logf("Setup: PlaybackStatus for %d (%s): %v", i, mPlayerName, statusErr)
			continue
		}
		// Get Metadata
		metadataVal, metadataErr := lmp.playerMap[mPlayerName].GetMetadata()
		if metadataErr != nil {
			lmp.logf("Setup: Metadata for %d (%s): %v", i, mPlayerName, metadataErr)
		}
		metadata := mp.MetadataFromMPRIS(metadataVal)
		// Append it to setupStatuses values
		setupStatuses = append(setupStatuses, mp.Status{
			Status:   string(plStatus),
			Index:    i,
			Name:     mPlayerName,
			Metadata: *metadata,
		})
		// TODO: Change this to `mp:init`, and move this to `Setup()`
		lmp.bidirChannel.OutChannel <- models.Message{
			Method: MPMethod(MethodRSetupMetadata),
			Args: &mp.SetupStatus{
				Statuses: setupStatuses,
			},
		}
	}
	return nil
}

// Main Setup Method
//
// This sets up the media player subsystem for linux, which includes setting up
// DBus Session Connections for MPRIS and Adding Players for the first launch,
// and anything else related to the same.
func (lmp *LinuxMediaPlayerSubsystem) Setup() error {
	// Set up Desktop Bus for Media Player Subsystem (Linux)
	busConn, sessionBusErr := dbus.SessionBus()
	if sessionBusErr != nil {
		return sessionBusErr
	}
	lmp.bus = busConn

	// Add signal for create/remove for player objects.
	dbusConnAddSignalErr := lmp.bus.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg0Namespace("org.mpris.MediaPlayer2"),
	)
	if dbusConnAddSignalErr != nil {
		return dbusConnAddSignalErr
	}
	// Add the currently alive players @ launch.
	lmp.setupAddPlayers()
	// Bind DBus singal
	lmp.bus.Signal(lmp.playerSigChan)
	lmp.logf("Players + Senders added: %d", len(lmp.playerNames))
	return nil
}

//	--- HANDLERS ---

// Handles the `Seeked` signal from Media Player
//
// This signals the client with player name, player index and the seeked time in microseconds (μs).
func (lmp *LinuxMediaPlayerSubsystem) handleSeeked(signal *dbus.Signal) {
	// TODO: Remove player index
	playerName, playerIndex, playerExists := lmp.findPlayerAndIndex(signal)
	if playerExists {
		seekedTime := signal.Body[0].(int64)
		// lmp.logf("Player %s seeked @ time %s", playerName, time.Duration(seekedTime*1000).String())
		// Send "Seeked" signal.
		lmp.bidirChannel.OutChannel <- models.Message{
			Method: MPAutoPlatformMethod(MethodSeeked),
			Args: &mp_signals.Seeked{
				// TODO: Remove player index
				PlayerIndex: playerIndex,
				PlayerName:  playerName,
				SeekedInUs:  seekedTime,
			},
		}
	}
}

// Property handler for handlePropertiesChanged
//
// This handles all the propertyChanged signals, for informing the user about
// the same.
//
// Currently, the following properties are supported:
// 1. `PlaybackStatus`: When Player Playback Status changes.
// 2. `Metadata`: When Player Metadata changes (When media changes).
func (lmp *LinuxMediaPlayerSubsystem) parseProperty(
	playerIdx int,
	playerName string,
	property map[string]dbus.Variant,
) error {
	for propKey, propValue := range property {
		switch propKey {
		case "PlaybackStatus":
			newPlaybackStatus, psParseError := ext_mp.ParsePlaybackStatus(propValue.Value().(string))
			if psParseError != nil {
				return psParseError
			}
			lmp.logf("Player %d (%s): %s", playerIdx, playerName, newPlaybackStatus)

			// Decode on whether you need more data/context to be sent in this data-structure.
			lmp.bidirChannel.OutChannel <- models.Message{
				Method: MPAutoPlatformMethod(MethodPlaybackStatusUpdated),
				Args: &mp_signals.PlaybackStatusChanged{
					PlayerIndex:    playerIdx,
					PlayerName:     playerName,
					PlaybackStatus: newPlaybackStatus,
				},
			}
		case "Metadata":
			metadataVariant, metadataGetErr := lmp.playerMap[playerName].GetMetadata()
			if metadataGetErr != nil {
				return metadataGetErr
			}
			metadata := mp.MetadataFromMPRIS(metadataVariant)
			lmp.bidirChannel.OutChannel <- models.Message{
				Method: MPAutoPlatformMethod(MethodMetadataUpdated),
				Args: &mp_signals.MetadataChanged{
					PlayerIndex: playerIdx,
					PlayerName:  playerName,
					Metadata:    metadata,
				},
			}
		default:
			return fmt.Errorf("key not found: KEY(%s): %s", propKey, propValue)
		}
	}
	return nil
}

// Signal handler for "PropertiesChanged"
//
// This method handles "PropertiesChanged" DBus Signal
func (lmp *LinuxMediaPlayerSubsystem) handlePropertiesChanged(signal *dbus.Signal) error {
	// signal.Body[0] = "org.mpris.MediaPlayer2.Player", representing interface
	// name. Ignore that value.
	lmp.logf("Signal: %+v", signal.Body)
	// TODO: Remove the index value, let client handle that.
	playerName, playerIdx, playerExists := lmp.findPlayerAndIndex(signal)
	if playerExists {
		for _, signalProp := range signal.Body[1:] {
			// Two kinds of value for signal body value are expected here:
			// 1. map[string]dbus.Variant
			// 2. []string (empty string)
			// We need 1, ignore 2.
			if property, propertyExists := signalProp.(map[string]dbus.Variant); propertyExists {
				parseErr := lmp.parseProperty(playerIdx, playerName, property)
				if parseErr != nil {
					return parseErr
				}
			}
		}
	}
	return nil
}

// TODO: Have a better naming scheme

// Signal handler for "NameOwnerChanged"
//
// This handles "org.freedesktop.DBus.NameOwnerChanged", for seeing the owner
// changes to a specific player.
func (lmp *LinuxMediaPlayerSubsystem) handleNameOwnerChanged(busSignal *dbus.Signal) {
	if len(busSignal.Body) != 3 {
		lmp.logf("ERROR: Incorrect name owner value length.")
		return
	}
	playerName, oldValue, newValue := busSignal.Body[0].(string),
		busSignal.Body[1].(string),
		busSignal.Body[2].(string)
	// ---- NOTE ABOUT **SIGNALS** ----
	// 	There's 3 arguments here. Each argument describes something.
	// 	Every change is represented by values in `NameOwnerChanged` signal.
	//	Arg 0: Media Player Name (org.mpris.MediaPlayer2.spotify).
	//	Arg 1: "Old Value" (oldValue).
	//	Arg 2: "New Value" (newValue).
	//	-- TYPES OF CHANGES --
	// Table shows value emptiness (empty string or "" is ❎, non-empty is ✅).
	// +----------+----------+---------------+
	// | OldValue | NewValue |     Change    |
	// +----------+----------+---------------+
	// |    ❎    |    ✅    | Create Player |
	// |    ✅    |    ❎    | Remove Player |
	// |  	✅    |    ✅    | Update Player |
	// +----------+----------+---------------+

	// Also send the "create"/"change"/"remove" operation contexts
	// to the client also, or at least work on implementing the same.
	if oldValue == "" {
		// CREATE PLAYER
		lmp.addPlayer(playerName, false)
		player := lmp.playerMap[playerName]
		// Player Playback Status
		playbackStatus, playbackStatusErr := player.GetPlaybackStatus()
		if playbackStatusErr != nil {
			lmp.logf("PlaybackErr: %v", playbackStatusErr)
			return
		}
		// Player Metadata
		unparsedMetadata, getMetadataErr := player.GetMetadata()
		if getMetadataErr != nil {
			lmp.logf("Get Metadata (unparsed): %v", getMetadataErr)
			return
		}
		parsedMetadata := mp.MetadataFromMPRIS(unparsedMetadata)
		// Player Index Check
		lmp.bidirChannel.OutChannel <- models.Message{
			Method: MPAutoPlatformMethod(MethodPlayerCreated),
			Args: &mp_signals.PlayerCreated{
				UpdatedPlayerNames: lmp.playerNames,
				PlayerData: mp.PlayerData{
					PlayerName:     playerName,
					PlaybackStatus: string(playbackStatus),
					Metadata:       *parsedMetadata,
				},
			},
		}
		lmp.logf("Player Added: %s", playerName)
	} else if newValue == "" {
		// -- DELETE PLAYER
		lmp.removePlayer(playerName)
		lmp.bidirChannel.OutChannel <- models.Message{
			Method: MPAutoPlatformMethod(MethodPlayerRemoved),
			Args: &mp_signals.PlayerRemoved{
				PlayerName:         playerName,
				UpdatedPlayerNames: lmp.playerNames,
			},
		}
		lmp.logf("Player Removed: %s", playerName)
	} else {
		// -- UPDATE PLAYER
		lmp.removePlayer(playerName)
		lmp.addPlayer(playerName, false)
		player := lmp.playerMap[playerName]
		// Player Playback Status
		playbackStatus, playbackStatusErr := player.GetPlaybackStatus()
		if playbackStatusErr != nil {
			lmp.logf("PlaybackErr: %v", playbackStatusErr)
			return
		}
		// Player Metadata
		unparsedMetadata, getMetadataErr := player.GetMetadata()
		if getMetadataErr != nil {
			lmp.logf("Get Metadata (unparsed): %v", getMetadataErr)
			return
		}
		parsedMetadata := mp.MetadataFromMPRIS(unparsedMetadata)
		lmp.bidirChannel.OutChannel <- models.Message{
			Method: MPAutoPlatformMethod(MethodPlayerUpdated),
			Args: &mp_signals.PlayerUpdated{
				PlayerData: mp.PlayerData{
					PlayerName:     playerName,
					PlaybackStatus: string(playbackStatus),
					Metadata:       *parsedMetadata,
				},
				UpdatedPlayerNames: lmp.playerNames,
			},
		}
		lmp.logf("Player Changed: %s", playerName)
	}
}

// DBus Signal Loop
//
// This listens to signals from MPRIS (through DBus) and emits it to the client.
func (lmp *LinuxMediaPlayerSubsystem) signalLoop() {
	lmp.logf("SignalLoop: start")
signalLoop:
	for {
		select {
		case value := <-lmp.playerSigChan:
			if value == nil {
				lmp.logf("Warning: DBus signal being sent is 'nil'")
				continue signalLoop
			}
			switch value.Name {
			case "org.freedesktop.DBus.Properties.PropertiesChanged":
				lmp.handlePropertiesChanged(value)
			case "org.freedesktop.DBus.NameOwnerChanged":
				lmp.handleNameOwnerChanged(value)
			case "org.mpris.MediaPlayer2.Player.Seeked":
				lmp.handleSeeked(value)
			default:
				lmp.logf("WARNING: MPRIS Signal")
			}
		// if need to break loop (produced by Close).
		case <-lmp.signalLoopBreak:
			break signalLoop
		}
	}
}

type PlayerSelection struct {
	//lint:ignore U1000 `msgpack` options, not for serialization.
	_msgpack   struct{} `msgpack:",as_array"`
	PlayerName string
}

// Main Subsystem Routine + Communication Loop
//
// This loop reads from command channel (from other modules) and communication
// channel (for communication with client).
func (lmp *LinuxMediaPlayerSubsystem) Routine() {
	lmp.logf("Routine: starting")
	if lmp.bidirChannel.InChannel == nil || lmp.bidirChannel.OutChannel == nil {
		return
	}
	// Run the signal loop to send the change events to client.
	go lmp.signalLoop()
	// Run the routine to pass in commands to validate values
lmpForRoutine:
	for {
		select {
		case readData := <-lmp.bidirChannel.InChannel:
			// This read channel will recieve the and will run actions which are deemed required
			decoder := msgpack.NewDecoder(bytes.NewReader(readData))
			// Validate Array-based Msgpack-RPC (by checking array length)
			payloadErr := utils.ValidateDecoder(decoder)
			if payloadErr != nil {
				lmp.logf("payloadErr: %v", payloadErr)
			}

			methodData, decodeErr := decoder.DecodeString()
			if decodeErr != nil {
				lmp.logf("Routine: decodeErr: %v", decodeErr)
			}

			methodWithoutValue, method, methodExists := strings.Cut(methodData, ":")
			if !methodExists {
				lmp.logf("Routine: method doesn't exist")
				method = methodWithoutValue
			}
			switch method {
			// -- FUNCTIONS --
			case "close":
				break lmpForRoutine
			case "list":
				players := make([]string, len(lmp.playerNames))
				copy(players, lmp.playerNames)
				lmp.logf("Players: %s", players)
				lmp.bidirChannel.OutChannel <- models.Message{
					Method: MPAutoPlatformMethod(MethodRList),
					Args:   &mp.MPlayerList{Players: players},
				}
			// -- METHODS --
			// NAME METHODS
			// INDEX METHODS
			case "iplay":
				var mpPlayVal mp.PlayerIndex
				mpParseErr := decoder.Decode(&mpPlayVal)
				if mpParseErr != nil {
					lmp.logf("Parse error: %v", mpParseErr)
				}
				lmp.logf("Play on Player %d\n", mpPlayVal.PlayerIndex)
				if len(lmp.playerNames) > mpPlayVal.PlayerIndex {
					playerName := lmp.playerNames[mpPlayVal.PlayerIndex]
					if selectedPlayer, playerExists := lmp.playerMap[playerName]; playerExists {
						selectedPlayer.Play()
					}
				}
			case "ipause":
				var mpPauseArgument mp.PlayerIndex
				mpParseErr := decoder.Decode(&mpPauseArgument)
				if mpParseErr != nil {
					lmp.logf("Pause::parseErr: %v", mpParseErr)
				}
				lmp.logf("Pause on Player %d", mpPauseArgument.PlayerIndex)
				if len(lmp.playerNames) > mpPauseArgument.PlayerIndex {
					playerName := lmp.playerNames[mpPauseArgument.PlayerIndex]
					if selectedPlayer, playerExists := lmp.playerMap[playerName]; playerExists {
						selectedPlayer.Pause()
					}
				}
			case "iplaypause":
				var mpPlayPause mp.PlayerIndex
				mpParseError := decoder.Decode(&mpPlayPause)
				if mpParseError != nil {
					lmp.logf("Playpause::parseErr: %v", mpParseError)
				}
				lmp.logf("Play/Pause on player %d", mpPlayPause.PlayerIndex)
				if len(lmp.playerNames) > mpPlayPause.PlayerIndex {
					playerName := lmp.playerNames[mpPlayPause.PlayerIndex]
					if selectedPlayer, playerExists := lmp.playerMap[playerName]; playerExists {
						selectedPlayer.PlayPause()
					}
				}
			case "ifwd":
				var mpFwdArgument mp.PlayerIndex
				mpParseErr := decoder.Decode(&mpFwdArgument)
				if mpParseErr != nil {
					lmp.logf("fwd::parseErr: %v", mpParseErr)
				}
				lmp.logf("Fwd on player %d", mpFwdArgument.PlayerIndex)
				if len(lmp.playerNames) > mpFwdArgument.PlayerIndex {
					playerName := lmp.playerNames[mpFwdArgument.PlayerIndex]
					if selectedPlayer, playerExists := lmp.playerMap[playerName]; playerExists {
						selectedPlayer.Next()
					}
				}
			case "iprv":
				var mpPrvArgument mp.PlayerIndex
				mpParseErr := decoder.Decode(&mpPrvArgument)
				if mpParseErr != nil {
					lmp.logf("prv::parseErr: %v", mpParseErr)
				}
				lmp.logf("Prv on player %d", mpPrvArgument.PlayerIndex)

				if len(lmp.playerNames) > mpPrvArgument.PlayerIndex {
					playerName := lmp.playerNames[mpPrvArgument.PlayerIndex]
					if selectedPlayer, playerExists := lmp.playerMap[playerName]; playerExists {
						selectedPlayer.Previous()
					}
				}
			default:
				lmp.logf("Method: %s unimplemented", method)
			}
		case moduleCommand := <-lmp.bidirChannel.CommandChannel:
			// If there's any other commands, put here
			switch moduleCommand {
			case "close":
				break lmpForRoutine
			default:
				lmp.logf("ERROR: Unexpected command passed in!")
				break lmpForRoutine
			}
		}
	}
	lmp.logf("Stopping")
}

func (lmp *LinuxMediaPlayerSubsystem) Shutdown() {
	// Stop the write loop & signal read loop
	lmp.bidirChannel.CommandChannel <- "close"
	lmp.signalLoopBreak <- false
	for _, playerName := range lmp.playerNames {
		lmp.removePlayerValues(playerName)
	}
	lmp.playerNames, lmp.playerMap, lmp.senderPlayerMap = []string{},
		make(map[string]*mpris.Player),
		make(map[string]string)
	// Close and remove the message bus
	lmp.bus.Close()
	lmp.bus = nil
	lmp.logf("Shutdown complete")
}
