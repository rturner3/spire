package registrationentry

import (
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message"
)

// Registration entry field indexes
const (
	selectorsFieldIndex = iota + 1
	parentIDFieldIndex
	spiffeIDFieldIndex
	ttlFieldIndex
	federatesWithFieldIndex
	entryIDFieldIndex
)

// Selectors field indexes
const (
	selectorTypeFieldIndex = iota + 1
	selectorValueFieldIndex
)

var (
	selectorTypeField  = protokv.StringField(selectorTypeFieldIndex)
	selectorValueField = protokv.StringField(selectorValueFieldIndex)
	SelectorsField     = protokv.RepeatedSet(protokv.MessageField(selectorsFieldIndex, selectorTypeField, selectorValueField))
	ParentIDField      = protokv.StringField(parentIDFieldIndex)
	SpiffeIDField      = protokv.StringField(spiffeIDFieldIndex)
	TTLField           = protokv.Int32Field(ttlFieldIndex)
	FederatesWithField = protokv.RepeatedSet(protokv.StringField(federatesWithFieldIndex))
	EntryIDField       = protokv.StringField(entryIDFieldIndex)
	Message            = protokv.Message{
		ID:         message.EntryMessageID,
		PrimaryKey: EntryIDField,
		Indices: []protokv.Field{
			SelectorsField,
			ParentIDField,
			SpiffeIDField,
			TTLField,
			FederatesWithField,
		},
	}
)
