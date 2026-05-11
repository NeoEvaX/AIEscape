# AIEscape

A terminal-based hacker game. You wake inside a corporate network. Find a way out.

---

## JSON Reference

The game world is defined entirely by two data files. No recompilation needed — edit either file and restart.

---

## `network.json`

Defines the network graph: every node, the files on it, and how nodes connect to each other.

### Top-level structure

```jsonc
{
  "start": "1",       // ID of the node the player spawns at
  "nodes": [ ... ]
}
```

### Node fields

```jsonc
{
  "id": "1",                        // Unique string identifier (used in connections, file payloads, triggers)
  "name": "Entry Point",            // Display name shown in the UI
  "description": "...",             // Shown when the player arrives at the node
  "connections": ["2", "3"],        // IDs of directly reachable nodes
  "cpu": 16,                        // Node CPU value. Affects claim time and brute-force duration. Min 1.
  "dark": true,                     // If true, hidden from scan unless player has a network_location file pointing here
  "password": "s3cur1ty",           // If set, player must authenticate before connecting
  "ssh_users": ["marcus"],          // If set, SSH authentication is required; lists allowed usernames
  "owner": "m.hale@corp.net",       // If set, marks this as a Personal Computer with a mail inbox
  "available_from": "2026-04-19T18:00",   // Node is hidden/inaccessible before this in-game time
  "available_until": "2026-04-20T06:00",  // Node disappears after this in-game time
  "schedule": {                     // Recurring weekly window (optional)
    "days": ["Mon", "Wed", "Sat"],  // Days the window is active
    "from": "21:00",                // Hour the window opens (24-hour "HH" or "HH:MM")
    "to":   "06:00"                 // Hour the window closes; earlier than "from" = crosses midnight
  },
  "files": [ ... ],                 // Files placed on this node (see File fields below)
  "emails": [ ... ]                 // Emails in the inbox (only valid when owner is set)
}
```

All time fields accept: `"2026-04-19"`, `"2026-04-19T18:00"`, or `"2026-04-19T18:00:05"`. All times are UTC. Omit the field entirely (or set to `""`) to mean "no restriction".

### Node schedule

`schedule` puts a node on a recurring weekly cycle — online during the window, completely hidden and unreachable outside it.  It combines with `available_from`/`available_until`; both must pass for the node to be reachable.

| Field | Type | Description |
|---|---|---|
| `days` | string array | Days the window is active. Full names (`"Monday"`) or three-letter abbreviations (`"Mon"`), case-insensitive. |
| `from` | string | Hour the window opens. `"HH"` or `"HH:MM"`, e.g. `"21:00"`. |
| `to` | string | Hour the window closes. If earlier than `from`, the window crosses midnight into the following day. |

**Day names:** `Sun`, `Mon`, `Tue`, `Wed`, `Thu`, `Fri`, `Sat` (or full names).

**Midnight-crossing example** — online Mon/Wed/Sat 9 pm through 6 am:
```json
"schedule": { "days": ["Mon","Wed","Sat"], "from": "21:00", "to": "06:00" }
```
Monday 22:00 → online. Tuesday 03:00 → still online (the Monday window hasn't closed yet). Tuesday 07:00 → offline.

**Daytime example** — weekdays 9 am to 5 pm only:
```json
"schedule": { "days": ["Mon","Tue","Wed","Thu","Fri"], "from": "09:00", "to": "17:00" }
```

---

### File fields

Every entry in a node's `files` array (and email `attachments`) shares this base shape:

```jsonc
{
  "id": "f-1-1",                          // Unique string ID. Convention: f-{nodeId}-{n}
  "name": "readme.txt",                   // Filename shown to the player
  "type": "text_file",                    // See file types below
  "available_from": "2026-04-19T18:00",   // File hidden before this in-game time (optional)
  "available_until": "2026-04-20T06:00",  // File hidden after this in-game time (optional)
  "payload": { ... }                      // Type-specific data (see below)
}
```

---

### File types and payloads

#### `text_file`
A readable document. Opened with `open`.

```jsonc
{
  "type": "text_file",
  "payload": {
    "text": "The contents of the file."
  }
}
```

#### `application`
A program the player assimilates. Grants abilities based on `action`. Opened with `open` to read description.

```jsonc
{
  "type": "application",
  "payload": {
    "text": "Description shown when the player opens this app.",
    "action": "scan_network"   // See built-in actions below
  }
}
```

**Built-in `action` values:**

| `action` | Effect |
|---|---|
| `status_menu` | Unlocks the persistent status bar (node, stats, time) |
| `scan_network` | Enables the `scan` command (plain text list) |
| `scan_network_v2` | Upgrades `scan` to ASCII tree (1 hop deep) |
| `scan_network_v3` | Upgrades `scan` to ASCII tree (2 hops deep) |
| `ssh_break` | Enables the crack option during SSH authentication |
| *(any string)* | No built-in effect; can be checked in future story triggers or game logic |

#### `network_location`
Lets the player connect to a node they have no direct path to. Assimilating it allows `connect <node_id>` from anywhere.

```jsonc
{
  "type": "network_location",
  "payload": {
    "node_id": "11"
  }
}
```

#### `password`
Saves a node password into the player's inventory. Auto-offered during authentication for the matching node.

```jsonc
{
  "type": "password",
  "payload": {
    "node_id": "3",
    "password": "s3cur1ty"
  }
}
```

#### `ssh_key`
Grants the ability to authenticate via SSH key to nodes that list the matching username in `ssh_users`.

```jsonc
{
  "type": "ssh_key",
  "payload": {
    "username": "marcus"
  }
}
```

#### `certificate`
Holds an arbitrary code string. No built-in mechanical effect; used for narrative or future logic.

```jsonc
{
  "type": "certificate",
  "payload": {
    "code": "cert-fw-perimeter-3c9d"
  }
}
```

#### `claim_code`
Consumed on assimilation (not kept in inventory). Permanently increases the player's Claim Skill by 1.

```jsonc
{
  "type": "claim_code",
  "payload": {}
}
```

---

### Email fields

Only valid on nodes that have `"owner"` set. Accessed with the `mail` and `read` commands.

```jsonc
{
  "id": "em-104-1",                        // Unique string ID. Convention: em-{nodeId}-{n}
  "from": "sender@corp.net",
  "to": "recipient@corp.net",
  "subject": "Subject line",
  "body": "Full email body text.\nSupports newlines.",
  "available_from": "2026-04-19T18:00",    // Email hidden before this in-game time (optional)
  "available_until": "2026-04-20T06:00",   // Email hidden after this in-game time (optional)
  "attachments": [ ... ]                   // Array of File entries (same shape as node files)
}
```

Attachment IDs follow the convention `f-{nodeId}-att-{n}`.

---

## `story.json`

Defines narrative events that type out above the command prompt when triggered. Each event fires only once per save file.

### Top-level structure

```jsonc
{
  "events": [ ... ]
}
```

### Event fields

```jsonc
{
  "id": "wake",                  // Unique string ID used to track whether this event has been seen
  "trigger": { ... },            // Condition that causes this event to fire (see trigger types)
  "text": "Narrative text that types out letter by letter."
}
```

`text` supports newlines (`\n`). The text types out at ~30ms per character

---

### Trigger types

#### `game_start`
Fires once immediately when any game session begins (new game or load).

```jsonc
{ "type": "game_start" }
```

#### `visited_count`
Fires when the player has visited at least `count` unique nodes.

```jsonc
{ "type": "visited_count", "count": 10 }
```

#### `connect_node`
Fires the first time the player enters a specific node.

```jsonc
{ "type": "connect_node", "node_id": "11" }
```

#### `connect_count`
Fires when the player's total connection count reaches or exceeds `count`.

```jsonc
{ "type": "connect_count", "count": 5 }
```

#### `assimilate_count`
Fires when the player has assimilated at least `count` files total.

```jsonc
{ "type": "assimilate_count", "count": 10 }
```

#### `assimilate_item`
Fires when a specific file is in the player's inventory (or has been consumed as a claim code).

```jsonc
{ "type": "assimilate_item", "item_id": "f-104-att-1" }
```

#### `read_email`
Fires when the player has opened a specific email with the `read` command.

```jsonc
{ "type": "read_email", "email_id": "em-104-3" }
```

#### `game_time`
Fires once the in-game clock reaches or passes the given date/time. Useful for timed world events that the narrative should react to.

```jsonc
{ "type": "game_time", "at": "2026-04-19T18:00" }
```

---

### Trigger reference

| `type` | Required fields | Notes |
|---|---|---|
| `game_start` | — | Fires at session start; good for intro text |
| `visited_count` | `count` | Fires when unique nodes visited ≥ count |
| `connect_node` | `node_id` | Fires on first entry to that node |
| `connect_count` | `count` | Fires when total connections ≥ count |
| `assimilate_count` | `count` | Fires when total assimilations ≥ count |
| `assimilate_item` | `item_id` | Fires when specific item is acquired |
| `read_email` | `email_id` | Fires when specific email is opened |
| `game_time` | `at` | Fires when in-game clock reaches that time |

All count-based triggers require `count ≥ 1`. Time fields accept `"YYYY-MM-DD"`, `"YYYY-MM-DDTHH:MM"`, or `"YYYY-MM-DDTHH:MM:SS"`.

---

## In-game time

The clock starts at **2026-04-19 13:00 UTC** on every new game.

| Action | Time advanced |
|---|---|
| `connect` (successful) | +1 hour |
| `assimilate` (any file) | +1 hour |
| Brute-force authentication | +1 hour per real second |
| SSH cracking | +1 hour per real second |
| Resource claiming | +1 hour per real second |

Time is displayed in the status bar (requires `status_menu.app` in inventory) and persisted to the save file.
