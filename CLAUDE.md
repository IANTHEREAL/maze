# Maze Exploration Guidelines

## Recursive Maze Exploration Loop

This is a recursive operation for maze exploration. Follow these steps for each exploration:

### Step 1: Check Current Status
Execute `./maze_client status ${exploration_name}` to check the current status and get information about next possible moves.

### Step 2: Handle Three Possible Scenarios

Based on the execution result, choose the next step according to one of three scenarios:

#### Scenario 1: Single Available Move
**Condition:** Output shows `Available moves (1)`

**Actions:**
1. Execute `./maze_client move ${exploration_name} available_move` to move using the current exploration_name
2. Execute `./maze_client render ${exploration_name}` to render the current state
3. Call `parallel_exploration` with:
   - `parallel_num = 1`
   - `max_results = 1` 
   - `prompt = ['The exploration branch name is ${exploration_name}, please follow CLAUDE.md requirements for maze exploration']`

#### Scenario 2: Junction Reached
**Condition:** Output shows `Junction reached - please start new exploration to explore different directions`

**Actions:**
1. Generate unique random names for each new exploration branch using the first 8 characters of a UUID as suffix
2. Start all new explorations by executing `./maze_client start ${new_random_exploration_name} position_x position_y` for each branch direction
3. After all new exploration branches are started, execute `./maze_client render ${new_random_exploration_name1} ${new_random_exploration_name2} ...` to render all new branches together
4. Use `parallel_exploration` for each new exploration branch with:
   - `parallel_num = 1`
   - `max_results = 1`
   - `prompt = ['The exploration branch name is ${new_random_exploration_name}, please follow CLAUDE.md requirements for maze exploration']`

#### Scenario 3: No Available Moves
**Condition:** Output shows `Available moves: None (blocked/wall)`

**Actions:**
1. Execute `./maze_client render ${exploration_name}` to render the final state
2. Summarize the exploration results
3. Exit the exploration loop

### Important Notes
- This is a recursive process - each successful move or new exploration branch should continue following these same guidelines
- Always use the exact exploration names provided or generated
- Each branch exploration operates independently following the same recursive pattern
- Generate unique names using UUID prefixes to avoid conflicts between parallel explorations