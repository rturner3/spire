package nodeselector

import (
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message"
)

var (
	spiffeIdField       = protokv.StringField(1)
	nodeSelectorMessage = protokv.Message{
		ID:         message.NodeSelectorMessageID,
		PrimaryKey: spiffeIdField,
	}
)
