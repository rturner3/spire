package bundle

import (
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message"
)

const (
	trustDomainIDFieldIndex = iota + 1
)

var (
	TrustDomainIDField = protokv.StringField(trustDomainIDFieldIndex)

	Message = protokv.Message{
		ID:         message.BundleMessageID,
		PrimaryKey: TrustDomainIDField,
	}
)
