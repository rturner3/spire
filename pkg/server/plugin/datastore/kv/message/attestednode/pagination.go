package attestednode

import "encoding/base64"

// TODO: Evaluate moving these to a common package to be shared by all message handlers
func encodePaginationToken(token []byte) string {
	return base64.RawURLEncoding.EncodeToString(token)
}

func decodePaginationToken(token string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(token)
}
