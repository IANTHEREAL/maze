package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
)

type Direction struct {
	X, Y int
}

var (
	UP    = Direction{0, -1}
	DOWN  = Direction{0, 1}
	LEFT  = Direction{-1, 0}
	RIGHT = Direction{1, 0}
)

type CellType int

const (
	WALL CellType = iota
	PATH
	START
	GOAL
)

type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func (p Position) Add(d Direction) Position {
	return Position{p.X + d.X, p.Y + d.Y}
}

type Exploration struct {
	ID               string     `json:"id"`
	StartPosition    Position   `json:"start_position"`
	CurrentPosition  Position   `json:"current_position"`
	PathPositions    []Position `json:"path_positions"`
	ParentID         *string    `json:"parent_id"`
	ChildIDs         []string   `json:"child_ids"`
	IsActive         bool       `json:"is_active"`
	IsComplete       bool       `json:"is_complete"`
	IsDead           bool       `json:"is_dead"`
	FoundGoal        bool       `json:"found_goal"`
	Generation       int        `json:"generation"`
}

func NewExploration(id string, startPos, currentPos Position, parentID *string, generation int) *Exploration {
	pathPositions := []Position{startPos}
	if currentPos != startPos {
		pathPositions = append(pathPositions, currentPos)
	}

	return &Exploration{
		ID:              id,
		StartPosition:   startPos,
		CurrentPosition: currentPos,
		PathPositions:   pathPositions,
		ParentID:        parentID,
		ChildIDs:        []string{},
		IsActive:        true,
		IsComplete:      false,
		IsDead:          false,
		FoundGoal:       false,
		Generation:      generation,
	}
}

type Game struct {
	Maze                     [][]CellType
	Width, Height            int
	Start, Goal              Position
	Explorations             map[string]*Exploration
	GlobalVisitedPositions   map[Position]bool
	GoalFound                bool
	WinningExploration       *string
	NextExplorationID        int
}

func NewGame(width, height int, seed int64) *Game {
	rand.Seed(seed)
	
	game := &Game{
		Width:                   width,
		Height:                  height,
		Explorations:            make(map[string]*Exploration),
		GlobalVisitedPositions:  make(map[Position]bool),
		GoalFound:               false,
		NextExplorationID:       0,
	}

	game.generateMaze()
	return game
}

type MazeStatusResponse struct {
	IsExplored           bool        `json:"is_explored"`
	IsJunction           bool        `json:"is_junction"`
	AvailableDirections  []Direction `json:"available_directions"`
	IsGoal               bool        `json:"is_goal"`
	GoalReachedByAny     bool        `json:"goal_reached_by_any"`
}

type MoveRequest struct {
	ExplorationName string   `json:"exploration_name"`
	NextPosition    Position `json:"next_position"`
}

type MoveResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	NewStatus string `json:"new_status"`
}

type ExplorationTreeResponse struct {
	Explorations map[string]*Exploration `json:"explorations"`
	GlobalStats  struct {
		TotalExplorations  int  `json:"total_explorations"`
		ActiveExplorations int  `json:"active_explorations"`
		GoalFound          bool `json:"goal_found"`
		VisitedPositions   int  `json:"visited_positions_count"`
	} `json:"global_stats"`
}

var game *Game

func main() {
	// Command line flags
	host := flag.String("host", "localhost", "Server host")
	port := flag.String("port", "8079", "Server port")
	flag.Parse()

	game = NewGame(31, 31, 42)

	http.HandleFunc("/maze-status", handleMazeStatus)
	http.HandleFunc("/move", handleMove)
	http.HandleFunc("/exploration-tree", handleExplorationTree)
	http.HandleFunc("/web", handleWebView)
	http.HandleFunc("/render", handleRender)

	addr := *host + ":" + *port
	fmt.Printf("🎮 Maze Game Server starting on %s\n", addr)
	fmt.Printf("📐 Maze size: %dx%d\n", game.Width, game.Height)
	fmt.Printf("📍 Start: (%d, %d)\n", game.Start.X, game.Start.Y)
	fmt.Printf("🎯 Goal: (%d, %d)\n", game.Goal.X, game.Goal.Y)
	fmt.Println("🖼️  Render: SVG images returned to client")
	fmt.Println("🚀 Ready for exploration commands!")
	fmt.Printf("🌐 Web viewer available at: http://%s/web\n", addr)

	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleMazeStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	x, _ := strconv.Atoi(r.URL.Query().Get("x"))
	y, _ := strconv.Atoi(r.URL.Query().Get("y"))
	pos := Position{x, y}

	response := game.getMazeStatus(pos)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	response := game.moveExploration(req.ExplorationName, req.NextPosition)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleExplorationTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := game.getExplorationTree()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (g *Game) generateMaze() {
	if g.Width%2 == 0 {
		g.Width++
	}
	if g.Height%2 == 0 {
		g.Height++
	}

	g.Maze = make([][]CellType, g.Height)
	for y := 0; y < g.Height; y++ {
		g.Maze[y] = make([]CellType, g.Width)
		for x := 0; x < g.Width; x++ {
			g.Maze[y][x] = WALL
		}
	}

	for y := 1; y < g.Height-1; y += 2 {
		for x := 1; x < g.Width-1; x += 2 {
			g.Maze[y][x] = PATH
		}
	}

	stack := []Position{{1, 1}}
	visited := map[Position]bool{{1, 1}: true}
	directions := []Direction{{0, -2}, {2, 0}, {0, 2}, {-2, 0}}

	for len(stack) > 0 {
		current := stack[len(stack)-1]

		var neighbors []Position
		for _, dir := range directions {
			next := Position{current.X + dir.X, current.Y + dir.Y}
			if next.X >= 1 && next.X < g.Width-1 && 
			   next.Y >= 1 && next.Y < g.Height-1 && 
			   !visited[next] {
				neighbors = append(neighbors, next)
			}
		}

		if len(neighbors) > 0 {
			next := neighbors[rand.Intn(len(neighbors))]
			visited[next] = true

			wallX := current.X + (next.X-current.X)/2
			wallY := current.Y + (next.Y-current.Y)/2
			g.Maze[wallY][wallX] = PATH

			stack = append(stack, next)
		} else {
			stack = stack[:len(stack)-1]
		}
	}

	for i := 0; i < g.Width*g.Height/30; i++ {
		x := 2 + rand.Intn((g.Width-4)/2)*2
		y := 2 + rand.Intn((g.Height-4)/2)*2

		for _, dir := range []Direction{{0, 1}, {1, 0}, {0, -1}, {-1, 0}} {
			nx, ny := x+dir.X, y+dir.Y
			if nx >= 0 && nx < g.Width && ny >= 0 && ny < g.Height && 
			   g.Maze[ny][nx] == PATH {
				g.Maze[y][x] = PATH
				break
			}
		}
	}

	g.Start = Position{1, 1}
	g.Maze[1][1] = START

	maxDist := 0
	bestGoal := Position{g.Width - 2, g.Height - 2}
	for y := 1; y < g.Height-1; y += 2 {
		for x := 1; x < g.Width-1; x += 2 {
			if g.Maze[y][x] == PATH {
				dist := abs(x-1) + abs(y-1)
				if dist > maxDist {
					maxDist = dist
					bestGoal = Position{x, y}
				}
			}
		}
	}

	g.Goal = bestGoal
	g.Maze[bestGoal.Y][bestGoal.X] = GOAL
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (g *Game) isWalkable(pos Position) bool {
	if pos.X < 0 || pos.X >= g.Width || pos.Y < 0 || pos.Y >= g.Height {
		return false
	}
	return g.Maze[pos.Y][pos.X] != WALL
}

func (g *Game) isCollision(pos Position) bool {
	return g.GlobalVisitedPositions[pos]
}

func (g *Game) getValidDirections(pos Position) []Direction {
	var valid []Direction
	directions := []Direction{UP, DOWN, LEFT, RIGHT}

	for _, dir := range directions {
		newPos := pos.Add(dir)
		if g.isWalkable(newPos) && !g.isCollision(newPos) {
			valid = append(valid, dir)
		}
	}
	return valid
}

func (g *Game) isAtGoal(pos Position) bool {
	return pos.X == g.Goal.X && pos.Y == g.Goal.Y
}

func (g *Game) getMazeStatus(pos Position) MazeStatusResponse {
	return MazeStatusResponse{
		IsExplored:          g.GlobalVisitedPositions[pos],
		IsJunction:          len(g.getValidDirections(pos)) > 1,
		AvailableDirections: g.getValidDirections(pos),
		IsGoal:              g.isAtGoal(pos),
		GoalReachedByAny:    g.GoalFound,
	}
}

func (g *Game) moveExploration(explorationName string, nextPos Position) MoveResponse {
	if !g.isWalkable(nextPos) {
		return MoveResponse{
			Success: false,
			Message: "Position is not walkable",
			NewStatus: "blocked",
		}
	}

	if g.isCollision(nextPos) {
		return MoveResponse{
			Success: false,
			Message: "Position already explored (collision)",
			NewStatus: "collision",
		}
	}

	exploration, exists := g.Explorations[explorationName]
	if !exists {
		// Create new exploration starting at nextPos
		exploration = NewExploration(explorationName, nextPos, nextPos, nil, 0)
		g.Explorations[explorationName] = exploration
	}

	// Check if this is the very first move to (1,1)
	if explorationName == "root" && nextPos.X == 1 && nextPos.Y == 1 && len(exploration.PathPositions) == 1 {
		// Root exploration starting - already at start position
		g.GlobalVisitedPositions[nextPos] = true
		return MoveResponse{
			Success: true,
			Message: "Root exploration started at start position",
			NewStatus: "continue",
		}
	}

	exploration.CurrentPosition = nextPos
	exploration.PathPositions = append(exploration.PathPositions, nextPos)
	g.GlobalVisitedPositions[nextPos] = true

	if g.isAtGoal(nextPos) {
		exploration.FoundGoal = true
		exploration.IsActive = false
		exploration.IsComplete = true
		g.GoalFound = true
		winnerName := explorationName
		g.WinningExploration = &winnerName
		return MoveResponse{
			Success: true,
			Message: fmt.Sprintf("Goal reached by %s!", explorationName),
			NewStatus: "goal_reached",
		}
	}

	validMoves := g.getValidDirections(nextPos)
	if len(validMoves) == 0 {
		exploration.IsDead = true
		exploration.IsActive = false
		exploration.IsComplete = true
		return MoveResponse{
			Success: true,
			Message: "Dead end reached",
			NewStatus: "dead_end",
		}
	}

	if len(validMoves) > 1 {
		return MoveResponse{
			Success: true,
			Message: "Junction reached - can branch explorations",
			NewStatus: "junction",
		}
	}

	return MoveResponse{
		Success: true,
		Message: "Moved successfully",
		NewStatus: "continue",
	}
}

func (g *Game) getExplorationTree() ExplorationTreeResponse {
	activeCount := 0
	for _, exp := range g.Explorations {
		if exp.IsActive {
			activeCount++
		}
	}

	return ExplorationTreeResponse{
		Explorations: g.Explorations,
		GlobalStats: struct {
			TotalExplorations  int  `json:"total_explorations"`
			ActiveExplorations int  `json:"active_explorations"`
			GoalFound          bool `json:"goal_found"`
			VisitedPositions   int  `json:"visited_positions_count"`
		}{
			TotalExplorations:  len(g.Explorations),
			ActiveExplorations: activeCount,
			GoalFound:          g.GoalFound,
			VisitedPositions:   len(g.GlobalVisitedPositions),
		},
	}
}

func handleWebView(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	
	html := `<!DOCTYPE html>
<html>
<head>
    <title>🎮 Maze Game - Real-time Viewer</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, sans-serif;
            margin: 0;
            padding: 20px;
            background: #f5f5f5;
        }
        .header {
            text-align: center;
            margin-bottom: 20px;
        }
        .container {
            display: flex;
            gap: 20px;
            justify-content: center;
            flex-wrap: wrap;
        }
        .canvas-container {
            background: white;
            border-radius: 8px;
            padding: 20px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        .stats {
            background: white;
            border-radius: 8px;
            padding: 20px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            min-width: 300px;
            max-width: 400px;
        }
        canvas {
            border: 1px solid #ddd;
            border-radius: 4px;
        }
        .stat-item {
            display: flex;
            justify-content: space-between;
            padding: 8px 0;
            border-bottom: 1px solid #eee;
        }
        .stat-item:last-child {
            border-bottom: none;
        }
        .exploration-list {
            max-height: 400px;
            overflow-y: auto;
            margin-top: 15px;
        }
        .exploration-item {
            background: #f8f9fa;
            margin: 5px 0;
            padding: 10px;
            border-radius: 4px;
            border-left: 4px solid #007bff;
        }
        .exploration-item.active {
            border-left-color: #28a745;
        }
        .exploration-item.dead {
            border-left-color: #dc3545;
            opacity: 0.7;
        }
        .exploration-item.goal {
            border-left-color: #ffc107;
            background: #fff8dc;
        }
        .refresh-controls {
            text-align: center;
            margin-bottom: 20px;
        }
        button {
            background: #007bff;
            color: white;
            border: none;
            padding: 10px 20px;
            border-radius: 4px;
            cursor: pointer;
            margin: 0 5px;
        }
        button:hover {
            background: #0056b3;
        }
        button:disabled {
            background: #6c757d;
            cursor: not-allowed;
        }
        .auto-refresh {
            color: #28a745;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>🎮 Maze Game - Real-time Viewer</h1>
        <div class="refresh-controls">
            <button onclick="toggleAutoRefresh()" id="autoBtn">🔄 Start Auto Refresh</button>
            <button onclick="refreshOnce()">↻ Refresh Once</button>
            <span id="status">Ready</span>
        </div>
    </div>
    
    <div class="container">
        <div class="canvas-container">
            <canvas id="mazeCanvas" width="600" height="600"></canvas>
        </div>
        
        <div class="stats">
            <h3>📊 Game Statistics</h3>
            <div id="stats-container">
                <div class="stat-item">
                    <span>Total Explorations:</span>
                    <span id="total-explorations">0</span>
                </div>
                <div class="stat-item">
                    <span>Active Explorations:</span>
                    <span id="active-explorations">0</span>
                </div>
                <div class="stat-item">
                    <span>Goal Found:</span>
                    <span id="goal-found">❌</span>
                </div>
                <div class="stat-item">
                    <span>Visited Positions:</span>
                    <span id="visited-positions">0</span>
                </div>
            </div>
            
            <h3>🔍 Explorations</h3>
            <div id="explorations-list" class="exploration-list">
                <!-- Explorations will be populated here -->
            </div>
        </div>
    </div>

    <script>
        const canvas = document.getElementById('mazeCanvas');
        const ctx = canvas.getContext('2d');
        let autoRefresh = false;
        let refreshInterval = null;
        let gameData = null;
        
        // Color palette matching Python implementation
        const COLORS = {
            background: '#FFFFFF',
            wall: '#E0E0E0',
            path: '#FAFAFA',
            start: '#4CAF50',
            goal: '#F44336',
            segmentColors: [
                '#2196F3',  // Blue
                '#9C27B0',  // Purple
                '#FF5722',  // Deep Orange
                '#8BC34A',  // Light Green
                '#00BCD4',  // Cyan
                '#E91E63'   // Pink
            ],
            winnerColor: '#FF6D00',  // Gold
            deadColor: '#9E9E9E',    // Gray
            robot: '#FF9800'         // Orange
        };
        
        function toggleAutoRefresh() {
            const btn = document.getElementById('autoBtn');
            const status = document.getElementById('status');
            
            if (autoRefresh) {
                autoRefresh = false;
                clearInterval(refreshInterval);
                btn.textContent = '🔄 Start Auto Refresh';
                status.textContent = 'Manual mode';
                status.className = '';
            } else {
                autoRefresh = true;
                refreshInterval = setInterval(fetchGameState, 1000);
                btn.textContent = '⏹ Stop Auto Refresh';
                status.textContent = 'Auto refreshing...';
                status.className = 'auto-refresh';
                fetchGameState(); // Immediate refresh
            }
        }
        
        function refreshOnce() {
            fetchGameState();
        }
        
        async function fetchGameState() {
            try {
                const response = await fetch('/exploration-tree');
                const data = await response.json();
                gameData = data;
                updateDisplay();
                document.getElementById('status').textContent = autoRefresh ? 
                    'Auto refreshing... (Last: ' + new Date().toLocaleTimeString() + ')' : 
                    'Last updated: ' + new Date().toLocaleTimeString();
            } catch (error) {
                console.error('Failed to fetch game state:', error);
                document.getElementById('status').textContent = 'Error fetching data';
            }
        }
        
        function updateDisplay() {
            if (!gameData) return;
            
            updateStats();
            updateExplorationsList();
            drawMaze();
        }
        
        function updateStats() {
            document.getElementById('total-explorations').textContent = gameData.global_stats.total_explorations;
            document.getElementById('active-explorations').textContent = gameData.global_stats.active_explorations;
            document.getElementById('goal-found').textContent = gameData.global_stats.goal_found ? '✅' : '❌';
            document.getElementById('visited-positions').textContent = gameData.global_stats.visited_positions_count;
        }
        
        function updateExplorationsList() {
            const container = document.getElementById('explorations-list');
            container.innerHTML = '';
            
            Object.entries(gameData.explorations).forEach(([id, exp]) => {
                const div = document.createElement('div');
                div.className = 'exploration-item';
                
                if (exp.found_goal) div.className += ' goal';
                else if (exp.is_dead) div.className += ' dead';
                else if (exp.is_active) div.className += ' active';
                
                const status = exp.found_goal ? '🎯' : exp.is_dead ? '💀' : exp.is_active ? '🚀' : '✅';
                const parent = exp.parent_id ? exp.parent_id : 'root';
                
                div.innerHTML = ` + "`" + `
                    <strong>${id}</strong> ${status}<br>
                    <small>Position: (${exp.current_position.x}, ${exp.current_position.y})</small><br>
                    <small>Path: ${exp.path_positions.length} steps | Parent: ${parent}</small>
                ` + "`" + `;
                
                container.appendChild(div);
            });
        }
        
        async function fetchMazeStructure() {
            // Fetch maze structure by querying multiple positions
            const mazeStructure = [];
            for (let y = 0; y < 31; y++) {
                mazeStructure[y] = [];
                for (let x = 0; x < 31; x++) {
                    try {
                        const response = await fetch(` + "`" + `/maze-status?x=${x}&y=${y}` + "`" + `);
                        const status = await response.json();
                        // Determine cell type based on status
                        let cellType = 'wall';
                        if (x === 1 && y === 1) cellType = 'start';
                        else if (status.is_goal) cellType = 'goal';
                        else if (status.available_directions.length > 0) cellType = 'path';
                        
                        mazeStructure[y][x] = cellType;
                    } catch (error) {
                        mazeStructure[y][x] = 'wall';
                    }
                }
            }
            return mazeStructure;
        }
        
        let mazeStructure = null;
        
        async function drawMaze() {
            if (!gameData) return;
            
            // Only fetch maze structure once
            if (!mazeStructure) {
                mazeStructure = await fetchMazeStructure();
            }
            
            const cellSize = canvas.width / 31; // 31x31 maze
            ctx.clearRect(0, 0, canvas.width, canvas.height);
            
            // Draw maze structure
            for (let y = 0; y < 31; y++) {
                for (let x = 0; x < 31; x++) {
                    const cellType = mazeStructure[y][x];
                    let color = COLORS.wall;
                    
                    if (cellType === 'path') color = COLORS.path;
                    else if (cellType === 'start') color = COLORS.start;
                    else if (cellType === 'goal') color = COLORS.goal;
                    
                    ctx.fillStyle = color;
                    ctx.fillRect(x * cellSize, y * cellSize, cellSize, cellSize);
                    
                    if (cellType === 'start' || cellType === 'goal') {
                        ctx.strokeStyle = 'white';
                        ctx.lineWidth = 2;
                        ctx.strokeRect(x * cellSize + 1, y * cellSize + 1, cellSize - 2, cellSize - 2);
                    }
                }
            }
            
            // Draw explorations
            Object.entries(gameData.explorations).forEach(([id, exp]) => {
                drawExploration(exp, cellSize);
            });
        }
        
        function drawExploration(exp, cellSize) {
            if (exp.path_positions.length < 2) return;
            
            // Get color for this exploration
            let color = COLORS.segmentColors[0];
            if (exp.found_goal) {
                color = COLORS.winnerColor;
            } else if (exp.is_dead) {
                color = COLORS.deadColor;
            } else {
                const colorIndex = parseInt(exp.id.replace('s', '')) % COLORS.segmentColors.length;
                color = COLORS.segmentColors[colorIndex] || COLORS.segmentColors[0];
            }
            
            // Draw path
            ctx.strokeStyle = color;
            ctx.lineWidth = exp.found_goal ? 4 : exp.is_active ? 3 : 2;
            ctx.lineCap = 'round';
            ctx.lineJoin = 'round';
            
            ctx.beginPath();
            const firstPos = exp.path_positions[0];
            ctx.moveTo(firstPos.x * cellSize + cellSize/2, firstPos.y * cellSize + cellSize/2);
            
            for (let i = 1; i < exp.path_positions.length; i++) {
                const pos = exp.path_positions[i];
                ctx.lineTo(pos.x * cellSize + cellSize/2, pos.y * cellSize + cellSize/2);
            }
            ctx.stroke();
            
            // Draw robot marker for active explorations
            if (exp.is_active) {
                const pos = exp.current_position;
                const centerX = pos.x * cellSize + cellSize/2;
                const centerY = pos.y * cellSize + cellSize/2;
                
                // Draw diamond shape robot marker
                ctx.fillStyle = color;
                ctx.beginPath();
                const size = cellSize * 0.3;
                ctx.moveTo(centerX, centerY - size);
                ctx.lineTo(centerX + size, centerY);
                ctx.lineTo(centerX, centerY + size);
                ctx.lineTo(centerX - size, centerY);
                ctx.closePath();
                ctx.fill();
                
                // White border
                ctx.strokeStyle = 'white';
                ctx.lineWidth = 2;
                ctx.stroke();
            }
        }
        
        // Initialize
        fetchGameState();
    </script>
</body>
</html>`

	fmt.Fprintf(w, html)
}

func handleRender(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate SVG content
	svgContent := generateMazeSVG()

	// Return SVG content directly
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Content-Disposition", "attachment; filename=\"maze.svg\"")
	fmt.Fprint(w, svgContent)
}

func generateMazeSVG() string {
	cellSize := 20
	width := game.Width * cellSize
	height := game.Height * cellSize

	var svg strings.Builder
	
	// SVG header
	svg.WriteString(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">
<rect width="%d" height="%d" fill="#FAFAFA"/>
`, width, height, width, height))

	// Draw maze structure
	for y := 0; y < game.Height; y++ {
		for x := 0; x < game.Width; x++ {
			cellType := game.Maze[y][x]
			color := "#E0E0E0" // wall color
			
			if cellType == PATH {
				color = "#FAFAFA" // path color
			} else if cellType == START {
				color = "#4CAF50" // start color
			} else if cellType == GOAL {
				color = "#F44336" // goal color
			}

			if cellType != PATH || cellType == START || cellType == GOAL {
				svg.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>
`, x*cellSize, y*cellSize, cellSize, cellSize, color))
			}
		}
	}

	// Color palette for explorations
	segmentColors := []string{
		"#2196F3", // Blue
		"#9C27B0", // Purple  
		"#FF5722", // Deep Orange
		"#8BC34A", // Light Green
		"#00BCD4", // Cyan
		"#E91E63", // Pink
	}
	winnerColor := "#FF6D00" // Gold
	deadColor := "#9E9E9E"   // Gray

	// Draw exploration paths
	for _, exp := range game.Explorations {
		if len(exp.PathPositions) < 2 {
			continue
		}

		// Determine color
		color := segmentColors[0]
		if exp.FoundGoal {
			color = winnerColor
		} else if exp.IsDead {
			color = deadColor
		} else {
			// Extract numeric ID for consistent coloring
			idStr := strings.TrimPrefix(exp.ID, "s")
			if idStr == "" || exp.ID == "root" {
				idStr = "0"
			}
			if id, err := strconv.Atoi(idStr); err == nil {
				color = segmentColors[id%len(segmentColors)]
			}
		}

		strokeWidth := 2
		if exp.FoundGoal {
			strokeWidth = 4
		} else if exp.IsActive {
			strokeWidth = 3
		}

		// Create path string
		var pathData strings.Builder
		first := exp.PathPositions[0]
		pathData.WriteString(fmt.Sprintf("M %f %f", 
			float64(first.X*cellSize)+float64(cellSize)/2,
			float64(first.Y*cellSize)+float64(cellSize)/2))

		for i := 1; i < len(exp.PathPositions); i++ {
			pos := exp.PathPositions[i]
			pathData.WriteString(fmt.Sprintf(" L %f %f",
				float64(pos.X*cellSize)+float64(cellSize)/2,
				float64(pos.Y*cellSize)+float64(cellSize)/2))
		}

		svg.WriteString(fmt.Sprintf(`<path d="%s" stroke="%s" stroke-width="%d" fill="none" stroke-linecap="round" stroke-linejoin="round"/>
`, pathData.String(), color, strokeWidth))

		// Draw robot marker for active explorations
		if exp.IsActive {
			pos := exp.CurrentPosition
			centerX := float64(pos.X*cellSize) + float64(cellSize)/2
			centerY := float64(pos.Y*cellSize) + float64(cellSize)/2
			size := float64(cellSize) * 0.3

			// Diamond shape
			svg.WriteString(fmt.Sprintf(`<polygon points="%f,%f %f,%f %f,%f %f,%f" fill="%s" stroke="white" stroke-width="2"/>
`,
				centerX, centerY-size,
				centerX+size, centerY,
				centerX, centerY+size,
				centerX-size, centerY,
				color))
		}
	}

	// Add title and stats
	stats := game.getExplorationTree()
	title := fmt.Sprintf("Maze Exploration - %d explorations, %d visited positions", 
		stats.GlobalStats.TotalExplorations, 
		stats.GlobalStats.VisitedPositions)
	
	svg.WriteString(fmt.Sprintf(`<text x="%d" y="20" font-family="Arial, sans-serif" font-size="14" font-weight="bold" fill="#424242">%s</text>
`, width/2-len(title)*4, title))

	if stats.GlobalStats.GoalFound {
		svg.WriteString(fmt.Sprintf(`<text x="%d" y="40" font-family="Arial, sans-serif" font-size="12" fill="#FF6D00">🎯 GOAL REACHED!</text>
`, width/2-60))
	}

	svg.WriteString("</svg>")
	return svg.String()
}