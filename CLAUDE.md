# Maze Exploration Guidelines

## Two-Phase Recursive Maze Exploration

This document defines a two-phase recursive maze exploration system compatible with Pantheon's parallel exploration framework.

### Environment Variables
- `exploration_name`: Current exploration name (provided by caller when invoking parallel_explore)
- `source_snap_id`: Snap ID from the completed previous phase, automatically provided by Pantheon

## Phase 1: Action Execution Phase

Execute `./maze_client status ${exploration_name}` to check current status and available coordinates, then handle one of three scenarios:

### Scenario 1: Single Available Move
**Condition:** Status output shows one available coordinate

**Actions:**
1. Execute `./maze_client move ${exploration_name} x y` using the available coordinates
2. Execute `./maze_client render ${exploration_name}` to visualize the new position

### Scenario 2: Junction (Multiple Available Moves)
**Condition:** Status output shows multiple available coordinates

**Actions:**
1. Parse all available coordinates from status output: `[(x1,y1), (x2,y2), (x3,y3), ...]`
2. For each coordinate, generate unique branch names using the format: `${exploration_name}_${direction}_${uuid_prefix}`
   - Example: `main_north_a1b2c3d4`, `main_south_e5f6g7h8`
3. Execute `./maze_client start ${branch_name} x y` for each available coordinate
4. Execute `./maze_client render ${branch1} ${branch2} ${branch3} ...` to render all new branches

### Scenario 3: Dead End
**Condition:** Status output shows no available coordinates (blocked/wall)

**Actions:**
1. Execute `./maze_client render ${exploration_name}` to render final state
2. Mark this exploration branch as completed

## Phase 2: Decision and Recursion Phase

Based on Phase 1 scenario results, call `parallel_explore` with appropriate parameters:

### After Scenario 1 (Single Move)
Continue exploring the current branch using the same exploration_name:
```python
parallel_explore(
    source_snap=source_snap_id (fetch from env),
    parallels_num=1,
    max_results=1,
    shared_prompt_sequence=[
        "exploration_name=${exploration_name}; Follow @CLAUDE.md to execute phase 1, determine next scenario and execute corresponding actions",
        "exploration_name=${exploration_name}; query Follow @CLAUDE.md to execute phase 2, spawn correct recursive branches"
    ]
)
```

### After Scenario 2 (Junction)  
Create parallel exploration for each branch created in Phase 1:
```python
# For each branch_name generated in Phase 1:
parallel_explore(
    source_snap=source_snap_id (fetch from env),
    parallels_num=1,
    max_results=1,
    shared_prompt_sequence=[
        "exploration_name=${branch_name}; Follow @CLAUDE.md to execute phase 1, determine next scenario and execute corresponding actions",
        "exploration_name=${branch_name}; Follow @CLAUDE.md to execute phase 2, spawn correct recursive branches"
    ]
)
```

### After Scenario 3 (Dead End)
No recursive action needed - this exploration branch is complete.

## Unified Recursion Pattern

**Key Insight:** All `parallel_explore` calls use identical structure:
- `parallels_num=1` (mandatory)
- `max_results=1` (mandatory)
- `source_snap=source_snap_id` (from environment)
- **Same 2-prompt sequence for all scenarios**

## Pantheon Integration Notes
- **Branch Naming**: Manual generation of unique branch names using `${exploration_name}_${direction}_${uuid_prefix}` format
- **Environment Management**: `source_snap_id` automatically provided by Pantheon; `exploration_name` set by caller
- **Snap Management**: `source_snap_id` is automatically set from the completed previous phase
