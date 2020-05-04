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
	SelectorsField           = protokv.RepeatedSet(protokv.MessageField(selectorsFieldIndex, selectorTypeField, selectorValueField))
	ParentIdField            = protokv.StringField(parentIdFieldIndex)
	SpiffeIdField            = protokv.StringField(spiffeIdFieldIndex)
	TtlField                 = protokv.Int32Field(ttlFieldIndex)
	FederatesWithField       = protokv.RepeatedSet(protokv.StringField(federatesWithFieldIndex))
	EntryIdField             = protokv.StringField(entryIdFieldIndex)
	RegistrationEntryMessage = protokv.Message{
		ID:         message.EntryMessageID,
		PrimaryKey: EntryIdField,
		Indices: []protokv.Field{
			SelectorsField,
			ParentIdField,
			SpiffeIdField,
			TtlField,
			FederatesWithField,
		},
	}
)
