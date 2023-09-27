package transport

type SendTrack struct {
}

func (t *SendTrack) Write(p []byte) (n int, err error) {
	return 0, nil
}
