package registrationentry

import (
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message"
)

var (
	selectorTypeField  = protokv.StringField(1)
	selectorValueField = protokv.StringField(2)
	selectorsField     = protokv.RepeatedSet(protokv.MessageField(1, selectorTypeField, selectorValueField))
	parentIdField      = protokv.StringField(2)
	spiffeIdField      = protokv.StringField(3)
	entryIdField       = protokv.StringField(6)
	ttlField           = protokv.Int32Field(4)

	registrationEntryMessage = protokv.Message{
		ID:         message.EntryMessageID,
		PrimaryKey: entryIdField,
		Indices: []protokv.Field{
			selectorsField,
			parentIdField,
			spiffeIdField,
			ttlField,
		},
	}
)
