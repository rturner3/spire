package attestednode

import (
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message"
)

const (
	spiffeIdFieldIndex = iota + 1
)

var (
	spiffeIdField = protokv.StringField(spiffeIdFieldIndex)

	attestedNodeMessage = protokv.Message{
		ID:         message.AttestedNodeMessageID,
		PrimaryKey: spiffeIdField,
	}
)
