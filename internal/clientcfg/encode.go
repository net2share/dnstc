package clientcfg

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

const urlPrefix = "dnstm://"

// Encode marshals a ClientConfig into a dnstm:// URL string.
func Encode(cfg *ClientConfig) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is nil")
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	encoded := base64.RawURLEncoding.EncodeToString(data)
	return urlPrefix + encoded, nil
}
