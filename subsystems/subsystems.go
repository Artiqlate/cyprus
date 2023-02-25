package subsystems

import "crosine.com/cyprus/comm"

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
