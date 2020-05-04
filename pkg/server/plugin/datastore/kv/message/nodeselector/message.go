package nodeselector

import (
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message"
)

const (
	spiffeIdFieldIndex = iota + 1
)

var (
	SpiffeIdField       = protokv.StringField(spiffeIdFieldIndex)
	NodeSelectorMessage = protokv.Message{
		ID:         message.NodeSelectorMessageID,
		PrimaryKey: SpiffeIdField,
	}
)
