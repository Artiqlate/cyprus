package media_player

import (
	"bytes"
	"log"
	"strings"
	"time"

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
	// Loop break signal
	signalLoopBreak chan bool
	// Linux-specific operations
	playerNames     []string
	playerMap       map[string]*mpris.Player
	senderPlayerMap map[string]string
	playerSigChan   chan *dbus.Signal
}

func NewLinuxMediaPlayerSubsystem(bidirChan *comm.BiDirMessageChannel) *LinuxMediaPlayerSubsystem {
	return &LinuxMediaPlayerSubsystem{
		logf: func(s string, i ...interface{}) {
			log.Printf("MP: "+s, i...)
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

// -- Some utility methods, for finding, and removal of specific data

func (lmp *LinuxMediaPlayerSubsystem) findPlayerName(player string) (int, bool) {
	for i, val := range lmp.playerNames {
		if val == player {
			return i, true
		}
	}
	return 0, false
}

func (lmp *LinuxMediaPlayerSubsystem) findSender(sender string) (string, bool) {
	playerName, senderExists := lmp.senderPlayerMap[sender]
	return playerName, senderExists
}

func (lmp *LinuxMediaPlayerSubsystem) findPlayerNameIdx(playerName string) (int, bool) {
	for i, val := range lmp.playerNames {
		if val == playerName {
			return i, true
		}
	}
	return 0, false
}

func (lmp *LinuxMediaPlayerSubsystem) removeSenderByName(playerName string) bool {
	for senderName, senderPlayerVal := range lmp.senderPlayerMap {
		if senderPlayerVal == playerName {
			delete(lmp.senderPlayerMap, senderName)
			return true
		}
	}
	return false
}

// -- PLAYER METHODS --

// - List Players
func (l *LinuxMediaPlayerSubsystem) ListPlayers() ([]string, error) {
	return l.playerNames, nil
}

// - Remove Player
func (lmp *LinuxMediaPlayerSubsystem) RemovePlayer(playerName string) bool {
	if playerToRemove, playerExists := lmp.playerMap[playerName]; playerExists {
		lmp.bus.RemoveMatchSignal(
			dbus.WithMatchSender(playerName),
			dbus.WithMatchObjectPath(lmp.bus.Object(playerName, dbusObjectPath).(*dbus.Object).Path()),
			dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		)
		// Quit the player
		playerToRemove.Quit()
		// Delete all values
		delete(lmp.playerMap, playerName)
		if !lmp.removeSenderByName(playerName) {
			lmp.logf("WARN: SenderPlayer sender not found.")
		}
		if playerIdx, playerIdxExists := lmp.findPlayerNameIdx(playerName); playerIdxExists {
			// Remove the player from the playerNames list
			lmp.playerNames = append(lmp.playerNames[:playerIdx], lmp.playerNames[playerIdx+1:]...)
		}
		return true
	}
	return false
}

// - Add Player
func (lmp *LinuxMediaPlayerSubsystem) AddPlayer(playerName string, isSetup bool) {
	if lmp.RemovePlayer(playerName) {
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
		// Register signal for the player
		lmp.bus.AddMatchSignal(
			dbus.WithMatchSender(playerName),
			dbus.WithMatchObjectPath(lmp.bus.Object(playerName, dbusObjectPath).Path()),
			dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		)
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
		// Throw away the value, to flip back
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

// -- SETUP METHODS

// - Add Players (for setting up the first time)
func (lmp *LinuxMediaPlayerSubsystem) AddPlayers() error {
	mediaPlayerNames, playerListErr := mpris.List(lmp.bus)
	if playerListErr != nil {
		return playerListErr
	}
	var setupStatuses []mp.Status
	for i, mPlayerName := range mediaPlayerNames {
		// Add Player
		lmp.AddPlayer(mPlayerName, true)
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
		metadata := mp.MediaPlayerFromMpris(metadataVal)
		// Append it to setupStatuses values
		setupStatuses = append(setupStatuses, mp.Status{
			Status:   string(plStatus),
			Index:    i,
			Name:     mPlayerName,
			Metadata: *metadata,
		})
		lmp.bidirChannel.OutChannel <- models.Message{
			Method: "mp:rsetup_metadata",
			Args: &mp.SetupStatus{
				Statuses: setupStatuses,
			},
		}
	}
	return nil
}

func (lmp *LinuxMediaPlayerSubsystem) Setup() error {
	// Set up Desktop Bus for Media Player Subsystem (Linux)
	busConn, sessionBusErr := dbus.SessionBus()
	if sessionBusErr != nil {
		return sessionBusErr
	}
	lmp.bus = busConn

	// Add signal for creation/removal of media players.
	dbusConnAddSignalErr := lmp.bus.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg0Namespace("org.mpris.MediaPlayer2"),
	)
	if dbusConnAddSignalErr != nil {
		return dbusConnAddSignalErr
	}
	// Add the currently alive players @ launch.
	lmp.AddPlayers()
	// Wire DBus signals to Go Signals
	lmp.bus.Signal(lmp.playerSigChan)
	// Assign the signal destination
	lmp.logf("Players + Senders added: %d", len(lmp.playerNames))
	return nil
}

func (lmp *LinuxMediaPlayerSubsystem) SignalLoop() {
	// Start the singal loop
	lmp.logf("SignalLoop: start")
signalLoop:
	for {
		select {
		case value := <-lmp.playerSigChan:
			// Checking if it is DBus variant value
			switch value.Name {
			case "org.freedesktop.DBus.Properties.PropertiesChanged":
				// Value Properties
				properties := value.Body[1].(map[string]dbus.Variant)
				playerName, playerExists := lmp.findSender(value.Sender)
				playerNameIdx, playerNameIdxExists := lmp.findPlayerName(playerName)

				// -- PLAYBACK STATUS --
				if playbackStatusProp, ok := properties["PlaybackStatus"]; playerExists && playerNameIdxExists && ok {
					playbackStatus := playbackStatusProp.Value().(string)
					lmp.logf("PLAYER %d (%s): STATUS %s", playerNameIdx, playerName, playbackStatus)
					metadataVal, metadataErr := lmp.playerMap[playerName].GetMetadata()
					if metadataErr != nil {
						lmp.logf("Metadata error: %v", metadataErr)
					}
					metadata := mp.MediaPlayerFromMpris(metadataVal)
					lmp.bidirChannel.OutChannel <- models.Message{
						Method: "mp:rstatus",
						Args: &mp.Status{
							Status:   playbackStatus,
							Index:    playerNameIdx,
							Name:     playerName,
							Metadata: *metadata,
						},
					}
				}

				// -- METADATA STATUS --
				if _, ok := properties["Metadata"]; ok {
					metadata, mprisErr := lmp.playerMap[lmp.playerNames[playerNameIdx]].GetMetadata()
					if mprisErr != nil {
						lmp.logf("mprisErr: %v", mprisErr)
						continue signalLoop
					}
					mplayerMeta := mp.MediaPlayerFromMpris(metadata)
					lmp.logf("Metadata: %v", mplayerMeta)
					lmp.bidirChannel.OutChannel <- models.Message{
						Method: "mp:metadata",
						Args:   &mplayerMeta,
					}
				}
			case "org.freedesktop.DBus.NameOwnerChanged":
				lmp.handleNameOwnerChanged(value)
			}
		case <-lmp.signalLoopBreak:
			break signalLoop
		}
	}
}

func (lmp *LinuxMediaPlayerSubsystem) handleNameOwnerChanged(busSignal *dbus.Signal) {
	if len(busSignal.Body) != 3 {
		lmp.logf("ERROR: Incorrect name owner value length.")
		return
	}
	playerName := busSignal.Body[0].(string)
	oldValue := busSignal.Body[1].(string)
	newValue := busSignal.Body[2].(string)

	// NOTE: ABOUT SIGNALS HERE
	// 	There's 3 arguments here. Each argument describes something.
	// 	Every change is represented by values in `NameOwnerChanged` signal.
	//	Arg 0: Media Player Name (org.mpris.MediaPlayer2.spotify).
	//	Arg 1: "Old Value" (oldValue).
	//	Arg 2: "New Value" (newValue).
	//	-- TYPES OF CHANGES --
	//	- If `oldValue` == "" and `newValue` != "", that means a new player has
	// 	  been added (PLAYER ADDED).
	//	- If `oldValue` != "" and `newValue` == "", that means a player has been
	//	  removed (PLAYER REMOVED).
	//	- If `oldValue` != "" and `newValue` != "", that means there had been
	//	  changes in the player, needs to be reset (PLAYER RESET).
	// IF PLAYER IS BEING ADDED
	if oldValue == "" {
		lmp.AddPlayer(playerName, false)
		lmp.logf("Player Added: %s", playerName)
	} else if newValue == "" {
		lmp.RemovePlayer(playerName)
		lmp.logf("Player Removed: %s", playerName)
	} else {
		lmp.RemovePlayer(playerName)
		lmp.AddPlayer(playerName, false)
		lmp.logf("Player Changed: %s", playerName)
	}
}

func (l *LinuxMediaPlayerSubsystem) Routine() {
	l.logf("Routine: starting")
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
				l.logf("Routine: decodeErr: %v", decodeErr)
			}

			methodWithoutValue, method, methodExists := strings.Cut(methodData, ":")
			if !methodExists {
				l.logf("Routine: method doesn't exist")
				method = methodWithoutValue
			}
			switch method {
			case "close":
				break lmpForRoutine
			case "list":
				// Throw the error our, it's always nil (might change depending on
				// platform, but not required in Linux).
				players, _ := l.ListPlayers()
				l.logf("Players: %s", players)
				l.bidirChannel.OutChannel <- models.Message{
					Method: "mp:rlist",
					Args:   &mp.MPlayerList{Players: players},
				}
			case "play":
				var mpPlayVal mp.MPlayerPlay
				mpParseErr := decoder.Decode(&mpPlayVal)
				if mpParseErr != nil {
					l.logf("Parse error: %v", mpParseErr)
				}
				l.logf("Play on Player %d\n", mpPlayVal.PlayerIndex)
				// Play the value
				if len(l.playerNames) > mpPlayVal.PlayerIndex {
					playerName := l.playerNames[mpPlayVal.PlayerIndex]
					if selectedPlayer, playerExists := l.playerMap[playerName]; playerExists {
						selectedPlayer.Play()
					}
				}
			case "pause":
				var mpPauseArgument mp.MPlayerPlay
				mpParseErr := decoder.Decode(&mpPauseArgument)
				if mpParseErr != nil {
					l.logf("Pause::parseErr: %v", mpParseErr)
				}
				l.logf("Pause on Player %d", mpPauseArgument.PlayerIndex)
				if len(l.playerNames) > mpPauseArgument.PlayerIndex {
					playerName := l.playerNames[mpPauseArgument.PlayerIndex]
					if selectedPlayer, playerExists := l.playerMap[playerName]; playerExists {
						selectedPlayer.Pause()
					}
				}
				// l.bidirChannel.OutChannel <- models.Message{Method: "mp:rpause", Args: nil}
			case "playpause":
				var mpPlayPause mp.MPlayerPlay
				mpParseError := decoder.Decode(&mpPlayPause)
				if mpParseError != nil {
					l.logf("Playpause::parseErr: %v", mpParseError)
				}
				l.logf("Play/Pause on player %d", mpPlayPause.PlayerIndex)
				if len(l.playerNames) > mpPlayPause.PlayerIndex {
					playerName := l.playerNames[mpPlayPause.PlayerIndex]
					if selectedPlayer, playerExists := l.playerMap[playerName]; playerExists {
						selectedPlayer.PlayPause()
					}
				}
			case "fwd":
				var mpFwdArgument mp.MPlayerPlay
				mpParseErr := decoder.Decode(&mpFwdArgument)
				if mpParseErr != nil {
					l.logf("fwd::parseErr: %v", mpParseErr)
				}
				l.logf("Fwd on player %d", mpFwdArgument.PlayerIndex)
				if len(l.playerNames) > mpFwdArgument.PlayerIndex {
					playerName := l.playerNames[mpFwdArgument.PlayerIndex]
					if selectedPlayer, playerExists := l.playerMap[playerName]; playerExists {
						selectedPlayer.Next()
					}
				}
			case "prv":
				var mpPrvArgument mp.MPlayerPlay
				mpParseErr := decoder.Decode(&mpPrvArgument)
				if mpParseErr != nil {
					l.logf("prv::parseErr: %v", mpParseErr)
				}
				l.logf("Prv on player %d", mpPrvArgument.PlayerIndex)

				if len(l.playerNames) > mpPrvArgument.PlayerIndex {
					playerName := l.playerNames[mpPrvArgument.PlayerIndex]
					if selectedPlayer, playerExists := l.playerMap[playerName]; playerExists {
						selectedPlayer.Previous()
					}
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
	// Stop the write loop
	l.bidirChannel.CommandChannel <- "close"
	// Stop the signal read loop
	l.signalLoopBreak <- false
	// Stop all the player values
	for _, playerName := range l.playerNames {
		if player, playerExists := l.playerMap[playerName]; playerExists {
			player.Quit()
		}
	}
	l.playerNames = []string{}
	l.playerMap = make(map[string]*mpris.Player)
	l.senderPlayerMap = make(map[string]string)
	l.bus.Close()
	l.bus = nil
	l.logf("Shutdown complete")
}
