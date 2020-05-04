package jointoken

import (
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message"
)

const (
	tokenFieldIndex = iota + 1
)

var (
	tokenField = protokv.StringField(tokenFieldIndex)

	JoinTokenMessage = protokv.Message{
		ID:         message.JoinTokenMessageID,
		PrimaryKey: tokenField,
	}
)
