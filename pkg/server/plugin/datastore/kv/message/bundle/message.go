package bundle

import (
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message"
)

var (
	trustDomainIdField = protokv.StringField(1)

	bundleMessage = protokv.Message{
		ID:         message.BundleMessageID,
		PrimaryKey: trustDomainIdField,
	}
)
