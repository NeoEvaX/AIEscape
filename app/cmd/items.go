package main

import (
	"encoding/json"
	"fmt"
	"time"
)

type ItemType string

const (
	ItemTypeTextFile        ItemType = "text_file"
	ItemTypeApplication     ItemType = "application"
	ItemTypeCertificate     ItemType = "certificate"
	ItemTypeNetworkLocation ItemType = "network_location"
	ItemTypeNetworkBridge   ItemType = "network_bridge"
	ItemTypeClaimCode       ItemType = "claim_code"
	ItemTypePassword        ItemType = "password"
	ItemTypeSSHKey          ItemType = "ssh_key"
)

func (t ItemType) Display() string {
	switch t {
	case ItemTypeTextFile:
		return "Text File"
	case ItemTypeApplication:
		return "Application"
	case ItemTypeCertificate:
		return "Certificate"
	case ItemTypeNetworkLocation:
		return "Network Location"
	case ItemTypeNetworkBridge:
		return "Network Bridge"
	case ItemTypeClaimCode:
		return "Claim Code"
	case ItemTypePassword:
		return "Password"
	case ItemTypeSSHKey:
		return "SSH Key"
	default:
		return string(t)
	}
}

type Item struct {
	ID             string
	Name           string
	Type           ItemType
	Payload        json.RawMessage
	AvailableFrom  time.Time // zero = no lower bound
	AvailableUntil time.Time // zero = no upper bound
}

// isAvailableAt returns whether [from, until] contains gameTime (zero bounds are open).
func isAvailableAt(from, until, gameTime time.Time) bool {
	if !from.IsZero() && gameTime.Before(from) {
		return false
	}
	if !until.IsZero() && gameTime.After(until) {
		return false
	}
	return true
}

func (item *Item) IsAvailable(gameTime time.Time) bool {
	return isAvailableAt(item.AvailableFrom, item.AvailableUntil, gameTime)
}

// ── Typed payload accessors ───────────────────────────────────────────────────

type TextFilePayload struct {
	Text string `json:"text"`
}

type ApplicationPayload struct {
	Text   string `json:"text"`
	Action string `json:"action"`
}

type CertificatePayload struct {
	// Code is an ID that can be referenced elsewhere in the game world.
	Code string `json:"code"`
}

type NetworkLocationPayload struct {
	// NodeID is the ID of the node this location points to.
	NodeID string `json:"node_id"`
}

func (item *Item) AsTextFile() (*TextFilePayload, error) {
	if item.Type != ItemTypeTextFile {
		return nil, fmt.Errorf("item %q is not a text file", item.ID)
	}
	var p TextFilePayload
	return &p, json.Unmarshal(item.Payload, &p)
}

func (item *Item) AsApplication() (*ApplicationPayload, error) {
	if item.Type != ItemTypeApplication {
		return nil, fmt.Errorf("item %q is not an application", item.ID)
	}
	var p ApplicationPayload
	return &p, json.Unmarshal(item.Payload, &p)
}

func (item *Item) AsCertificate() (*CertificatePayload, error) {
	if item.Type != ItemTypeCertificate {
		return nil, fmt.Errorf("item %q is not a certificate", item.ID)
	}
	var p CertificatePayload
	return &p, json.Unmarshal(item.Payload, &p)
}

type PasswordPayload struct {
	NodeID   string `json:"node_id"`
	Password string `json:"password"`
}

type SSHKeyPayload struct {
	Username string `json:"username"`
}

func (item *Item) AsSSHKey() (*SSHKeyPayload, error) {
	if item.Type != ItemTypeSSHKey {
		return nil, fmt.Errorf("item %q is not an SSH key", item.ID)
	}
	var p SSHKeyPayload
	return &p, json.Unmarshal(item.Payload, &p)
}

func (item *Item) AsPassword() (*PasswordPayload, error) {
	if item.Type != ItemTypePassword {
		return nil, fmt.Errorf("item %q is not a password", item.ID)
	}
	var p PasswordPayload
	return &p, json.Unmarshal(item.Payload, &p)
}

func (item *Item) AsNetworkLocation() (*NetworkLocationPayload, error) {
	if item.Type != ItemTypeNetworkLocation {
		return nil, fmt.Errorf("item %q is not a network location", item.ID)
	}
	var p NetworkLocationPayload
	return &p, json.Unmarshal(item.Payload, &p)
}

// NetworkBridgePayload grants the player the ability to cross from one logical
// network island to another via a direct node connection. One-directional: only
// FromNetwork → ToNetwork works; the reverse requires a separate item.
type NetworkBridgePayload struct {
	FromNetwork string `json:"from_network"`
	ToNetwork   string `json:"to_network"`
}

func (item *Item) AsNetworkBridge() (*NetworkBridgePayload, error) {
	if item.Type != ItemTypeNetworkBridge {
		return nil, fmt.Errorf("item %q is not a network bridge", item.ID)
	}
	var p NetworkBridgePayload
	return &p, json.Unmarshal(item.Payload, &p)
}

// Open returns the human-readable text content of the item.
// Returns ("", false) if the item type has no readable text.
func (item *Item) Open() (string, bool) {
	switch item.Type {
	case ItemTypeTextFile:
		p, err := item.AsTextFile()
		if err != nil {
			return "", false
		}
		return p.Text, true
	case ItemTypeApplication:
		p, err := item.AsApplication()
		if err != nil {
			return "", false
		}
		return p.Text, true
	}
	return "", false
}
