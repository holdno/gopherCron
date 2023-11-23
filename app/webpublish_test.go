package app

import (
	"encoding/json"
	"testing"

	"github.com/holdno/firetower/protocol"
)

func TestGenericMessage(t *testing.T) {
	// msg := CloudEventWithNil{}

	raw := `{"type":2,"topic":"/task/status/project/59","data":""}`

	var data protocol.TopicMessage[CloudEventWithNil]

	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}
}
