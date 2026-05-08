package main

import (
	"encoding/json"
	"fmt"
)

type ItemType string

const (
	ItemTypeTextFile        ItemType = "text_file"
	ItemTypeApplication     ItemType = "application"
	ItemTypeCertificate     ItemType = "certificate"
	ItemTypeNetworkLocation ItemType = "network_location"
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
	default:
		return string(t)
	}
}

type Item struct {
	ID      string
	Name    string
	Type    ItemType
	Payload json.RawMessage
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

func (item *Item) AsNetworkLocation() (*NetworkLocationPayload, error) {
	if item.Type != ItemTypeNetworkLocation {
		return nil, fmt.Errorf("item %q is not a network location", item.ID)
	}
	var p NetworkLocationPayload
	return &p, json.Unmarshal(item.Payload, &p)
}
