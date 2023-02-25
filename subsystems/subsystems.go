package subsystems

type ServerSubsystems struct {
	mp MediaPlayerSubsystem
}

func NewServerSubsystem() *ServerSubsystems {
	return &ServerSubsystems{
		mp: nil,
	}
}

func (s *ServerSubsystems) SetupMediaPlayer() error {
	if s.mp != nil {
		newMediaPlayer, mediaPlayerSetupErr := NewMediaPlayerSubsystem()
		if mediaPlayerSetupErr != nil {
			return mediaPlayerSetupErr
		}
		s.mp = newMediaPlayer
	}
	return nil
}
