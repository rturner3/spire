package attestednode

import "encoding/base64"

func encodePaginationToken(token []byte) string {
	return base64.RawURLEncoding.EncodeToString(token)
}

func decodePaginationToken(token string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(token)
}
