package registrationentry

import "github.com/gofrs/uuid"

func newEntryID() (string, error) {
	u, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
