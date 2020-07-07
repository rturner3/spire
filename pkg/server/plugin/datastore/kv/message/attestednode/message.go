package attestednode

import (
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message"
)

const (
	spiffeIDFieldIndex = iota + 1
)

var (
	SpiffeIDField = protokv.StringField(spiffeIDFieldIndex)

	Message = protokv.Message{
		ID:         message.AttestedNodeMessageID,
		PrimaryKey: SpiffeIDField,
	}
)
