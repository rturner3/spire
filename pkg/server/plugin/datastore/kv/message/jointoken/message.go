package jointoken

import (
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message"
)

var (
	tokenField = protokv.StringField(1)

	joinTokenMessage = protokv.Message{
		ID:         message.JoinTokenMessageID,
		PrimaryKey: tokenField,
	}
)
