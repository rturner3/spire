package registrationentry

import (
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message"
)

// Registration entry field indexes
const (
	selectorsFieldIndex = iota + 1
	parentIdFieldIndex
	spiffeIdFieldIndex
	ttlFieldIndex
	federatesWithFieldIndex
	entryIdFieldIndex
)

// Selectors field indexes
const (
	selectorTypeFieldIndex = iota + 1
	selectorValueFieldIndex
)

var (
	selectorTypeField        = protokv.StringField(selectorTypeFieldIndex)
	selectorValueField       = protokv.StringField(selectorValueFieldIndex)
	selectorsField           = protokv.RepeatedSet(protokv.MessageField(selectorsFieldIndex, selectorTypeField, selectorValueField))
	parentIdField            = protokv.StringField(parentIdFieldIndex)
	spiffeIdField            = protokv.StringField(spiffeIdFieldIndex)
	ttlField                 = protokv.Int32Field(ttlFieldIndex)
	federatesWithField       = protokv.RepeatedSet(protokv.StringField(federatesWithFieldIndex))
	entryIdField             = protokv.StringField(entryIdFieldIndex)
	registrationEntryMessage = protokv.Message{
		ID:         message.EntryMessageID,
		PrimaryKey: entryIdField,
		Indices: []protokv.Field{
			selectorsField,
			parentIdField,
			spiffeIdField,
			ttlField,
			federatesWithField,
		},
	}
)
