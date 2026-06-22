# 🏎️ F1 Season Simulator

A multiplayer terminal-based Formula 1 season simulator written in Go. Players draft teams, hire drivers and team principals, configure their cars, and race through a full 24-round calendar with qualifying simulation, race results, DNFs, and driver development across seasons.

> **🚧 Active Development** — The simulation engine is currently being reworked and new features are in progress. An online multiplayer version is planned for release after the engine rewrite is complete.

---

## 🚀 Quick Start

### Requirements

- Go 1.21+
- GCC (required to build `go-sqlite3`)
  - Linux: `sudo apt install gcc`
  - macOS: Xcode Command Line Tools — `xcode-select --install`
  - Windows: [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) or WSL

### Installation & Run

```bash
# 1. Clone the repository
git clone <repo-url>
cd f1

# 2. Download dependencies
go mod download

# 3. Run the simulator
go run ./cmd/sim/main.go
```

> **The bundled `f1_simulation.db` already contains all necessary data** — teams, drivers, tracks, engines and team principals are pre-loaded. No additional setup or seeding is required.

---

## 📂 Custom Data via CSV

All game data is driven by CSV files located in `initial_data/`. You can replace or edit any of them to load your own rosters, tracks, teams, or engine lineups — no code changes needed.

| File | What you can customize |
|------|------------------------|
| `pilot.csv` | Driver names, ratings, driving style, experience, tyre management, price, and all other attributes |
| `pilot_track.csv` | Per-driver familiarity level (0–20) for each track |
| `base.csv` | Team names, car level, base infrastructure, engineer/sim/tunnel ratings, budget |
| `track.csv` | Track names, downforce type, difficulty, qualifying impact, rain probability, tyre demand |
| `engine.csv` | Engine suppliers, price and base power level |
| `team_principal.csv` | Principal names, hiring price and management level |

After editing the CSVs, re-seed the database to apply changes:

```bash
go run ./cmd/data/seed.go -d -c -s
```

> Column order matters — refer to the existing rows in each file as a template before adding or modifying entries.

---

## 🗺️ Roadmap

| Status | Item |
|--------|------|
| ✅ Done | Core race & qualifying simulation |
| ✅ Done | Driver rating development system |
| ✅ Done | Multi-season gameplay with transfers |
| 🔧 In progress | Simulation engine rewrite & accuracy improvements |
| 🔧 In progress | New gameplay features |
| 🔜 Planned | **Online multiplayer web version** |

---

## 🎮 Gameplay

### 1. Draft Phase

Each player takes a turn to:
1. Pick a **team** (Ferrari, Mercedes, Red Bull, etc.)
2. Sign **two drivers** from the available pool
3. Choose a **Team Principal**
4. Select an **engine supplier** (if the team is not a manufacturer)
5. Optionally buy extra **tokens** with budget (1M = 1 token)

### 2. Car Configuration

Players distribute **tokens** across car components before the season starts:

| Component   | Effect |
|-------------|--------|
| Aerodynamics | Performance on high-downforce tracks |
| Engine | Performance on high-drag / power tracks |
| Chassis | Balance across most track types |
| Floor | Overall pace contribution |
| Tyres | Tyre wear management during races |
| Reliability | Reduces mechanical DNF chance (55 tokens = 0% DNF chance) |

### 3. Season Simulation

**24 rounds** run back-to-back through the full F1 calendar. Each round includes:
- **Qualifying** — determines grid positions
- **Race** — points awarded (25-18-15-12-10-8-6-4-2-1)
- **DNFs** — mechanical failures and driver errors based on car reliability and driver traits

Driver ratings update after every race based on results.

### 4. Off-Season Window

After the final race, players can make changes before starting a new season:

| Command | Description |
|---------|-------------|
| `transfer <your_id> <pilot_id> <amount>` | Sign a free agent or make an offer for another player's driver |
| `fire <your_id> pilot/principal <id>` | Release a driver or principal (partial refund) |
| `engine <your_id> <engine_id>` | Switch engine supplier |
| `change_principal <your_id> <principal_id> <amount>` | Hire a new Team Principal |
| `change <your_id> <opp_id> <your_pilot_id> <their_pilot_id> <amount>` | Swap drivers with another player |
| `start` | Build the car and begin the next season |

---

## 🗂️ Project Structure

```
f1/
├── cmd/
│   ├── sim/main.go           # Simulator entry point
│   └── data/seed.go          # DB reset & seed utility
├── internal/
│   ├── cli/ui.go             # Terminal UI & game loop
│   ├── engine/
│   │   ├── sim.go            # Race & qualifying simulation core
│   │   └── hooks.go          # Post-race and post-season rating updates
│   ├── models/models.go      # Domain models
│   └── storage/
│       ├── storage.go        # Repository interface
│       └── sqlite.go         # SQLite implementation
├── initial_data/
│   ├── pilot.csv             # 28 drivers with full stats
│   ├── pilot_track.csv       # Per-driver track familiarity levels
│   ├── base.csv              # 11 F1 teams
│   ├── track.csv             # 24-round calendar with track characteristics
│   ├── engine.csv            # Engine suppliers and power levels
│   └── team_principal.csv    # Team principal pool
├── f1_simulation.db          # Pre-seeded SQLite database ← ready to use
└── go.mod
```

---

## 🗄️ Database

The project uses **SQLite** via `go-sqlite3`. The included `f1_simulation.db` is already fully populated — no manual seeding needed.

| Table | Description |
|-------|-------------|
| `pilots_initial` | Master copy of all driver stats (source of truth) |
| `pilots` | Current in-session driver ratings |
| `pilots_track_initial` | Master copy of per-driver track familiarity |
| `pilots_track` | Current in-session track familiarity levels |
| `base_team` | Master copy of team data |
| `teams` | Current in-session team state |
| `tracks` | Track list with downforce, type, tyre, rain and difficulty data |
| `engine` | Engine suppliers and base power levels |
| `teams_principals` | Team principal pool with price and level |
| `players` | Player state — budget, tokens, team and principal assignment |
| `car` | Car component token allocation per player team |

### Resetting the database

To wipe and re-seed from scratch:

```bash
go run ./cmd/data/seed.go -d -c -s
```

Flags: `-d` drop tables, `-c` create tables, `-s` seed data.

---

## ⚙️ Simulation Mechanics

### Speed Factors (Qualifying & Race)

- **Driver rating** and **qualifying rating**
- **Car level** of the team
- **Car-to-track fit** — aerodynamics for high-downforce, engine for high-drag, chassis/tyres for medium
- **Track familiarity** per driver (0–20 scale)
- **Team Principal bonus** based on their level
- **Weather** — rain experts gain pace, poor wet-weather drivers lose it
- **Driving style** — Aggressive gives speed but risks time penalties; Smooth reduces error chance
- **Tyre management** vs track tyre demand
- **Random variance** scaled by driver stability

### Driver Development

After each race, drivers gain or lose rating based on:
- Win / podium / points finish
- Head-to-head result vs teammate
- Positions gained or lost vs qualifying
- Driver-error DNFs

After the full season, additional adjustments apply for: championship win, season-long teammate duel outcome, and championship position vs team expectations.

---

## 👥 Teams

| Team | Engine | Budget |
|------|--------|--------|
| Ferrari | Ferrari | 150M |
| Mercedes | Mercedes | 150M |
| Red Bull | RBPT | 150M |
| McLaren | Mercedes | 150M |
| Aston Martin | Mercedes | 150M |
| Alpine | Renault | 120M |
| Audi | Audi | 150M |
| Williams | Mercedes | 110M |
| Racing Bulls | RBPT | 130M |
| Haas | Ferrari | 100M |
| Cadillac | Cadillac | 150M |