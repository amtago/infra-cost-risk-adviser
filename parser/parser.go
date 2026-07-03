// Package parser reads terraform show -json output and extracts resource changes.
package parser

import (
	"encoding/json"
	"fmt"
	"os"
)

// ChangeType represents the type of change for a resource.
type ChangeType string

const (
	ChangeCreate  ChangeType = "create"
	ChangeUpdate  ChangeType = "update"
	ChangeDelete  ChangeType = "delete"
	ChangeReplace ChangeType = "replace"
	ChangeNoOp    ChangeType = "no-op"
)

// ResourceChange represents a single resource change from the plan.
type ResourceChange struct {
	Address      string
	ProviderName string
	Type         string
	Name         string
	ChangeType   ChangeType
	Before       map[string]interface{}
	After        map[string]interface{}
}

// tfPlan is the raw structure of terraform show -json output.
type tfPlan struct {
	ResourceChanges []tfResourceChange `json:"resource_changes"`
}

type tfResourceChange struct {
	Address       string   `json:"address"`
	ProviderName  string   `json:"provider_name"`
	Type          string   `json:"type"`
	Name          string   `json:"name"`
	Change        tfChange `json:"change"`
}

type tfChange struct {
	Actions []string               `json:"actions"`
	Before  map[string]interface{} `json:"before"`
	After   map[string]interface{} `json:"after"`
}

// ParseFile reads a terraform plan JSON file and returns resource changes.
func ParseFile(path string) ([]ResourceChange, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading plan file: %w", err)
	}
	return Parse(data)
}

// Parse parses raw terraform plan JSON bytes and returns resource changes.
func Parse(data []byte) ([]ResourceChange, error) {
	var plan tfPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("parsing plan JSON: %w", err)
	}

	var changes []ResourceChange
	for _, rc := range plan.ResourceChanges {
		ct := actionsToChangeType(rc.Change.Actions)
		if ct == ChangeNoOp {
			continue
		}
		changes = append(changes, ResourceChange{
			Address:      rc.Address,
			ProviderName: rc.ProviderName,
			Type:         rc.Type,
			Name:         rc.Name,
			ChangeType:   ct,
			Before:       rc.Change.Before,
			After:        rc.Change.After,
		})
	}
	return changes, nil
}

func actionsToChangeType(actions []string) ChangeType {
	switch {
	case len(actions) == 1 && actions[0] == "create":
		return ChangeCreate
	case len(actions) == 1 && actions[0] == "update":
		return ChangeUpdate
	case len(actions) == 1 && actions[0] == "delete":
		return ChangeDelete
	case len(actions) == 2: // ["create","delete"] or ["delete","create"]
		return ChangeReplace
	default:
		return ChangeNoOp
	}
}
