package ext_mp

import (
	"math"

	"github.com/CrosineEnterprises/ganymede/models/mp"
	"github.com/Pauloo27/go-mpris"
)

func NewPlayerDataFromPlayer(player *mpris.Player) (*mp.PlayerData, error) {
	playbackStatus, psError := player.GetPlaybackStatus()
	if psError != nil {
		return nil, psError
	}
	loopStatus, lsError := player.GetLoopStatus()
	if lsError != nil {
		// return nil, lsError
		loopStatus = mp.LoopStatusNone
	}
	loopStatusVal := string(loopStatus)
	rate, rateErr := player.GetRate()
	if rateErr != nil {
		return nil, rateErr
	}
	shuffle, shuffleErr := player.GetShuffle()
	if shuffleErr != nil {
		// return nil, shuffleErr
		shuffle = false
	}
	metadata, metaErr := player.GetMetadata()
	if metaErr != nil {
		return nil, metaErr
	}
	volume, volumeErr := player.GetVolume()
	if volumeErr != nil {
		return nil, volumeErr
	}
	position, positionErr := player.GetPosition()
	if positionErr != nil {
		return nil, positionErr
	}

	return &mp.PlayerData{
		PlaybackStatus: string(playbackStatus),
		LoopStatus:     &loopStatusVal,
		Rate:           rate,
		Shuffle:        &shuffle,
		Metadata:       mp.MetadataFromMPRIS(metadata),
		Volume:         volume,
		Position:       int64(position),
		MinimumRate:    math.NaN(),
		MaximumRate:    math.NaN(),
	}, nil
}
