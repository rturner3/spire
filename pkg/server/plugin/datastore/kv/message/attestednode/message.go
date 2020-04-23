package attestednode

import (
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message"
)

var (
	spiffeIdField = protokv.StringField(1)

	attestedNodeMessage = protokv.Message{
		ID:         message.AttestedNodeMessageID,
		PrimaryKey: spiffeIdField,
	}
)
