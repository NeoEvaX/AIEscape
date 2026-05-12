package main

import (
	"testing"
	"time"
)

// ── parseGameTime ─────────────────────────────────────────────────────────────

func TestParseGameTime(t *testing.T) {
	utc := time.UTC
	cases := []struct {
		in   string
		want time.Time
	}{
		{"", time.Time{}},
		{"bad", time.Time{}},
		{"2026-04-19", time.Date(2026, 4, 19, 0, 0, 0, 0, utc)},
		{"2026-04-19T18:00", time.Date(2026, 4, 19, 18, 0, 0, 0, utc)},
		{"2026-04-19T18:00:05", time.Date(2026, 4, 19, 18, 0, 5, 0, utc)},
	}
	for _, c := range cases {
		got := parseGameTime(c.in)
		if !got.Equal(c.want) {
			t.Errorf("parseGameTime(%q) = %v; want %v", c.in, got, c.want)
		}
	}
}

// ── parseScheduleHour ─────────────────────────────────────────────────────────

func TestParseScheduleHour(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"0", 0},
		{"9", 9},
		{"21", 21},
		{"23", 23},
		{"06:00", 6},
		{"21:30", 21},
		{"24", 23},  // clamped to 23
		{"-1", 0},   // negative clamped to 0
	}
	for _, c := range cases {
		got := parseScheduleHour(c.in)
		if got != c.want {
			t.Errorf("parseScheduleHour(%q) = %d; want %d", c.in, got, c.want)
		}
	}
}

// ── parseSchedule ─────────────────────────────────────────────────────────────

func TestParseSchedule_Nil(t *testing.T) {
	if got := parseSchedule(nil); got != nil {
		t.Errorf("parseSchedule(nil) = %v; want nil", got)
	}
}

func TestParseSchedule_EmptyDays(t *testing.T) {
	ns := &networkSchedule{Days: []string{}, From: "09:00", To: "17:00"}
	if got := parseSchedule(ns); got != nil {
		t.Errorf("parseSchedule(empty days) = %v; want nil", got)
	}
}

func TestParseSchedule_AllInvalidDays(t *testing.T) {
	ns := &networkSchedule{Days: []string{"Foo", "Bar"}, From: "09:00", To: "17:00"}
	if got := parseSchedule(ns); got != nil {
		t.Errorf("parseSchedule(invalid days) = %v; want nil", got)
	}
}

func TestParseSchedule_Valid(t *testing.T) {
	ns := &networkSchedule{
		Days: []string{"Mon", "Wednesday", "fri"},
		From: "09:00",
		To:   "17:00",
	}
	got := parseSchedule(ns)
	if got == nil {
		t.Fatal("parseSchedule returned nil; want a schedule")
	}
	if got.From != 9 {
		t.Errorf("From = %d; want 9", got.From)
	}
	if got.To != 17 {
		t.Errorf("To = %d; want 17", got.To)
	}
	wantDays := map[time.Weekday]bool{
		time.Monday: true, time.Wednesday: true, time.Friday: true,
	}
	if len(got.Days) != len(wantDays) {
		t.Fatalf("len(Days) = %d; want %d", len(got.Days), len(wantDays))
	}
	for _, d := range got.Days {
		if !wantDays[d] {
			t.Errorf("unexpected day %v in schedule", d)
		}
	}
}

// ── loadNetworkFromBytes ──────────────────────────────────────────────────────

const minimalNetJSON = `{
  "start": "1",
  "nodes": [{"id":"1","name":"Root","connections":[],"cpu":10}]
}`

func TestLoadNetworkFromBytes_Valid(t *testing.T) {
	net, err := loadNetworkFromBytes([]byte(minimalNetJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if net.StartNodeID != "1" {
		t.Errorf("StartNodeID = %q; want %q", net.StartNodeID, "1")
	}
	if _, ok := net.Nodes["1"]; !ok {
		t.Error("node '1' not found in Nodes map")
	}
}

func TestLoadNetworkFromBytes_EmptyStartUsesFirst(t *testing.T) {
	js := `{"start":"","nodes":[{"id":"42","name":"Root","connections":[],"cpu":10}]}`
	net, err := loadNetworkFromBytes([]byte(js))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if net.StartNodeID != "42" {
		t.Errorf("StartNodeID = %q; want %q", net.StartNodeID, "42")
	}
}

func TestLoadNetworkFromBytes_MissingStartNode(t *testing.T) {
	js := `{"start":"99","nodes":[{"id":"1","name":"Root","connections":[],"cpu":10}]}`
	_, err := loadNetworkFromBytes([]byte(js))
	if err == nil {
		t.Error("expected error for missing start node; got nil")
	}
}

func TestLoadNetworkFromBytes_NoNodes(t *testing.T) {
	js := `{"start":"","nodes":[]}`
	_, err := loadNetworkFromBytes([]byte(js))
	if err == nil {
		t.Error("expected error for empty nodes; got nil")
	}
}

func TestLoadNetworkFromBytes_InvalidJSON(t *testing.T) {
	_, err := loadNetworkFromBytes([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON; got nil")
	}
}

func TestLoadNetworkFromBytes_CPUMin(t *testing.T) {
	js := `{"start":"1","nodes":[{"id":"1","name":"Root","connections":[],"cpu":0}]}`
	net, err := loadNetworkFromBytes([]byte(js))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if net.Nodes["1"].CPU < 1 {
		t.Errorf("CPU = %d; want >= 1", net.Nodes["1"].CPU)
	}
}

func TestLoadNetworkFromBytes_FieldMapping(t *testing.T) {
	js := `{
		"start": "1",
		"nodes": [{
			"id": "1",
			"name": "Test Node",
			"description": "A test node",
			"connections": ["2"],
			"cpu": 16,
			"dark": true,
			"network": "corp",
			"password": "secret",
			"ssh_users": ["alice", "bob"],
			"owner": "alice@corp.test",
			"available_from": "2026-04-19T10:00",
			"available_until": "2026-04-20T10:00",
			"schedule": {"days": ["Mon","Fri"], "from": "09:00", "to": "17:00"}
		}]
	}`
	net, err := loadNetworkFromBytes([]byte(js))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := net.Nodes["1"]
	if n.Name != "Test Node" {
		t.Errorf("Name = %q; want %q", n.Name, "Test Node")
	}
	if n.Description != "A test node" {
		t.Errorf("Description = %q; want %q", n.Description, "A test node")
	}
	if n.CPU != 16 {
		t.Errorf("CPU = %d; want 16", n.CPU)
	}
	if !n.Dark {
		t.Error("Dark = false; want true")
	}
	if n.Network != "corp" {
		t.Errorf("Network = %q; want %q", n.Network, "corp")
	}
	if n.Password != "secret" {
		t.Errorf("Password = %q; want %q", n.Password, "secret")
	}
	if len(n.SSHUsers) != 2 {
		t.Errorf("len(SSHUsers) = %d; want 2", len(n.SSHUsers))
	}
	if n.Owner != "alice@corp.test" {
		t.Errorf("Owner = %q; want %q", n.Owner, "alice@corp.test")
	}
	if n.Schedule == nil {
		t.Fatal("Schedule = nil; want a schedule")
	}
	if n.Schedule.From != 9 || n.Schedule.To != 17 {
		t.Errorf("Schedule From/To = %d/%d; want 9/17", n.Schedule.From, n.Schedule.To)
	}
	if n.AvailableFrom.IsZero() {
		t.Error("AvailableFrom is zero; want 2026-04-19T10:00")
	}
	if n.AvailableUntil.IsZero() {
		t.Error("AvailableUntil is zero; want 2026-04-20T10:00")
	}
	// Auto-generated loc file should be appended.
	hasLoc := false
	for _, f := range n.Files {
		if f.Type == ItemTypeNetworkLocation {
			hasLoc = true
		}
	}
	if !hasLoc {
		t.Error("auto-generated network_location file not found on node")
	}
}

func TestLoadNetworkFromBytes_NetworkBridgeItem(t *testing.T) {
	js := `{
		"start": "1",
		"nodes": [{
			"id": "1",
			"name": "Root",
			"connections": [],
			"cpu": 10,
			"files": [{
				"id": "f-1-1",
				"name": "bridge.bin",
				"type": "network_bridge",
				"payload": {"from_network": "corp", "to_network": "dmz"}
			}]
		}]
	}`
	net, err := loadNetworkFromBytes([]byte(js))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := net.Nodes["1"]
	found := false
	for _, f := range n.Files {
		if f.Type == ItemTypeNetworkBridge {
			found = true
			p, err := f.AsNetworkBridge()
			if err != nil {
				t.Fatalf("AsNetworkBridge error: %v", err)
			}
			if p.FromNetwork != "corp" || p.ToNetwork != "dmz" {
				t.Errorf("bridge payload = (%q, %q); want (corp, dmz)", p.FromNetwork, p.ToNetwork)
			}
		}
	}
	if !found {
		t.Error("network_bridge item not found after loading")
	}
}
