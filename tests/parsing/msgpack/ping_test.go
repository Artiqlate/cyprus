package cyprus

import (
	"sync"
	"testing"
	"time"

	"crosine.com/cyprus/models"
	"github.com/stretchr/testify/assert"
)

func TestPing(t *testing.T) {
	// Create variables for managing race conditions
	var producerLock sync.WaitGroup
	var consumerLock bool = true

	// Create byte-channel and unserialized parsing channel (for comparison)
	byteChannel := make(chan []byte)
	unparsedChannel := make(chan models.Ping)

	// Data Consumer Coroutine
	go func(channel chan []byte, unSerChannel chan models.Ping) {
		for consumerLock || len(channel) > 0 {
			unparsedBinary := <-channel
			t.Logf("Unparsed binary: %2x", unparsedBinary)
			parsedSerObj, err := models.DecodePing(unparsedBinary)
			if err != nil {
				t.Errorf("cannot decode parsed value: %s", err)
			}
			t.Logf("ELE: %d %s %s %s", parsedSerObj.GetType(), parsedSerObj.GetId().String(), parsedSerObj.GetTimestamp().Format(time.RFC3339), parsedSerObj.GetMessage())
			unserialObj := <-unSerChannel
			assert.Equal(t, parsedSerObj.GetId(), unserialObj.GetId())
			assert.Equal(t, parsedSerObj.GetType(), unserialObj.GetType())
			assert.Equal(t, parsedSerObj.GetTimestamp().Format(time.RFC3339), unserialObj.GetTimestamp().Format(time.RFC3339))
		}
	}(byteChannel, unparsedChannel)

	// Data Producer Coroutine
	producerLock.Add(1)
	go func(channel chan []byte, unSerChannel chan models.Ping) {
		for i := 0; i < 20; i++ {
			// Create a serialized object
			pingObj, err := models.NewPing("Hello, World!")
			if err != nil {
				t.Errorf("cannot generate serializable: %v", err)
			}
			// Encode
			binaryEncoded, err := pingObj.Encode()
			t.Logf("Encoded value: %x\n", binaryEncoded)
			if err != nil {
				t.Errorf("cannot encode into msgpack: %v", err)
			}
			channel <- binaryEncoded
			unSerChannel <- *pingObj
		}
		consumerLock = false
		producerLock.Done()
	}(byteChannel, unparsedChannel)

	producerLock.Wait()
}
