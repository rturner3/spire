package bundle

import (
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message"
)

const (
	trustDomainIdFieldIndex = iota + 1
)

var (
	trustDomainIdField = protokv.StringField(trustDomainIdFieldIndex)

	bundleMessage = protokv.Message{
		ID:         message.BundleMessageID,
		PrimaryKey: trustDomainIdField,
	}
)
