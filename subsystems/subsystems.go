package subsystems

import "github.com/Artiqlate/cyprus/comm"

type ServerSubsystems struct {
	mp MediaPlayerSubsystem
}

func NewServerSubsystem() *ServerSubsystems {
	return &ServerSubsystems{
		mp: nil,
	}
}

func (s *ServerSubsystems) SetupMediaPlayer(mpBiDirChan *comm.BiDirMessageChannel) error {
	if s.mp != nil {
		newMediaPlayer, mediaPlayerSetupErr := NewMediaPlayerSubsystem(mpBiDirChan)
		if mediaPlayerSetupErr != nil {
			return mediaPlayerSetupErr
		}
		s.mp = newMediaPlayer
	}
	return nil
}
