package bundle

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

const Scheme = "mirage://"

type Bundle struct {
	Version     int            `json:"version"`
	ProfileID   string         `json:"profile_id"`
	DisplayName string         `json:"display_name,omitempty"`
	Server      map[string]any `json:"server"`
	Client      map[string]any `json:"client"`
	Transports  map[string]any `json:"transports"`
	DNS         map[string]any `json:"dns"`
	Routing     map[string]any `json:"routing"`
}

func Encode(b Bundle) (string, error) {
	payload, err := json.Marshal(b)
	if err != nil {
		return "", err
	}
	return Scheme + base64.RawURLEncoding.EncodeToString(payload), nil
}

func Decode(link string) (Bundle, error) {
	if !strings.HasPrefix(link, Scheme) {
		return Bundle{}, errors.New("bundle must start with mirage://")
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(link, Scheme))
	if err != nil {
		return Bundle{}, err
	}
	var b Bundle
	if err := json.Unmarshal(payload, &b); err != nil {
		return Bundle{}, err
	}
	if b.Version != 1 {
		return Bundle{}, errors.New("unsupported bundle version")
	}
	return b, nil
}

func (b Bundle) Name() string {
	if b.DisplayName != "" {
		return b.DisplayName
	}
	if b.ProfileID != "" {
		return b.ProfileID
	}
	return b.ServerHost()
}

func (b Bundle) ServerHost() string {
	if b.Server == nil {
		return ""
	}
	if value, _ := b.Server["host"].(string); value != "" {
		return value
	}
	if value, _ := b.Server["domain"].(string); value != "" {
		return value
	}
	return ""
}

func (b Bundle) CamouflageSite() string {
	if b.Server == nil {
		return ""
	}
	value, _ := b.Server["camouflage_site"].(string)
	return value
}
