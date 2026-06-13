# F1 Simulation DB Schema

<details>
<summary><b>engine</b></summary>

- id INTEGER PK
- manufacturer TEXT
- price INTEGER
- power INTEGER
</details>

<details>
<summary><b>base_team</b></summary>

- id INTEGER PK
- name TEXT
- car_lvl INTEGER
- ice INTEGER
- base_lvl INTEGER
- engineer INTEGER
- tube INTEGER
- sim INTEGER
- update_rtg INTEGER
- is_manufacturer INTEGER
</details>

<details>
<summary><b>teams</b></summary>

- id INTEGER PK
- name TEXT
- car_lvl INTEGER
- ice INTEGER
- base_lvl INTEGER
- engineer INTEGER
- tube INTEGER
- sim INTEGER
- update_rtg INTEGER
- is_manufacturer INTEGER
</details>

<details>
<summary><b>tracks</b></summary>

- id INTEGER PK
- name TEXT
- downforce INTEGER
- type INTEGER
- difficulity INTEGER
- quali_impact INTEGER
- rain INTEGER
- tyre INTEGER
</details>

<details>
<summary><b>teams_principals</b></summary>

- id INTEGER PK
- name TEXT
- price INTEGER
- level INTEGER
</details>

<details>
<summary><b>pilots_initial</b></summary>

- id INTEGER PK
- name TEXT
- rating INTEGER
- quali_rating INTEGER
- style INTEGER
- expirince INTEGER
- adaptiveness INTEGER
- emotions INTEGER
- stability INTEGER
- rain INTEGER
- settings_angle INTEGER
- starting INTEGER
- tyre_management INTEGER
- mistake_possibility INTEGER
- price INTEGER
- sponsors INTEGER
</details>

<details>
<summary><b>pilots</b></summary>

- id INTEGER PK
- name TEXT
- rating INTEGER
- quali_rating INTEGER
- style INTEGER
- expirince INTEGER
- adaptiveness INTEGER
- emotions INTEGER
- stability INTEGER
- rain INTEGER
- settings_angle INTEGER
- starting INTEGER
- tyre_management INTEGER
- mistake_possibility INTEGER
- price INTEGER
- sponsors INTEGER
</details>

<details>
<summary><b>pilots_track_initial</b></summary>

- id INTEGER PK
- pilot_id INTEGER (FK → pilots_initial.id)
- track_id INTEGER (FK → tracks.id)
- level INTEGER
</details>

<details>
<summary><b>pilots_track</b></summary>

- id INTEGER PK
- pilot_id INTEGER (FK → pilots_initial.id)
- track_id INTEGER (FK → tracks.id)
- level INTEGER
</details>

<details>
<summary><b>players</b></summary>

- id INTEGER PK
- name TEXT
- team_id INTEGER (FK → teams.id)
- pilot1_id INTEGER (FK → pilots.id)
- pilot2_id INTEGER (FK → pilots.id)
- principal_id INTEGER (FK → teams_principals.id)
</details>