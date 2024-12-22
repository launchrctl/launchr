package driver

import (
	"io"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/namesgenerator"

	"github.com/launchrctl/launchr/internal/launchr"
)

// GetRandomName generates a random human-friendly name.
func GetRandomName(retry int) string {
	return namesgenerator.GetRandomName(retry)
}

// DockerDisplayJSONMessages prints docker json output to streams.
func DockerDisplayJSONMessages(in io.Reader, streams launchr.Streams) error {
	err := jsonmessage.DisplayJSONMessagesToStream(in, streams.Out(), nil)
	if err != nil {
		if jerr, ok := err.(*jsonmessage.JSONError); ok {
			// If no error code is set, default to 1
			if jerr.Code == 0 {
				jerr.Code = 1
			}
			return jerr
		}
	}
	return err
}
