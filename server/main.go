package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
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
	http.HandleFunc("/exploration-status", handleExplorationStatus)
	http.HandleFunc("/move", handleMove)
	http.HandleFunc("/exploration-tree", handleExplorationTree)
	http.HandleFunc("/reset", handleReset)
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

func handleExplorationStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	explorationName := r.URL.Query().Get("name")
	if explorationName == "" {
		http.Error(w, "Missing exploration name parameter", http.StatusBadRequest)
		return
	}

	exploration, exists := game.Explorations[explorationName]
	if !exists {
		http.Error(w, "Exploration not found", http.StatusNotFound)
		return
	}

	pos := exploration.CurrentPosition
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

func handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Reset game state
	game.Explorations = make(map[string]*Exploration)
	game.GlobalVisitedPositions = make(map[Position]bool)
	game.GoalFound = false
	game.WinningExploration = nil
	game.NextExplorationID = 0

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Game reset successfully",
	})
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	// Write HTML in parts to avoid Go string literal issues
	fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Maze Explorer</title>
    <style>
        body { font-family: monospace; margin: 0; padding: 20px; background: #FFFFFF; }
        .header { text-align: center; margin-bottom: 20px; }
        .container { display: flex; justify-content: center; }
        .canvas-container { background: white; border: 1px solid #ddd; }
        canvas { display: block; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Multi-Branch BFS Pathfinding</h1>
        <div>Step: <span id="step">0</span> | Active: <span id="active">0</span> | Total: <span id="total">0</span></div>
    </div>
    <div class="container">
        <div class="canvas-container">
            <canvas id="mazeCanvas" width="800" height="800"></canvas>
        </div>
    </div>
    <script>`)

	fmt.Fprint(w, `
        const canvas = document.getElementById('mazeCanvas');
        const ctx = canvas.getContext('2d');
        let gameData = null;
        
        const COLORS = {
            background: '#FFFFFF',
            wall: '#E0E0E0', 
            path: '#FAFAFA',
            start: '#4CAF50',
            goal: '#F44336',
            segmentColors: ['#2196F3', '#9C27B0', '#FF5722', '#8BC34A', '#00BCD4', '#E91E63'],
            winnerColor: '#FF6D00',
            deadColor: '#9E9E9E'
        };
        
        async function fetchGameState() {
            try {
                const response = await fetch('/exploration-tree');
                const data = await response.json();
                gameData = data;
                updateDisplay();
            } catch (error) {
                console.error('Error:', error);
            }
        }
        
        function updateDisplay() {
            if (!gameData) return;
            document.getElementById('active').textContent = gameData.global_stats.active_explorations;
            document.getElementById('total').textContent = gameData.global_stats.total_explorations;
            document.getElementById('step').textContent = Object.keys(gameData.explorations).length;
            drawMaze();
        }
        
        async function fetchMazeStructure() {
            const maze = [];
            for (let y = 0; y < 31; y++) {
                maze[y] = [];
                for (let x = 0; x < 31; x++) {
                    try {
                        const response = await fetch('/maze-status?x=' + x + '&y=' + y);
                        const status = await response.json();
                        let cellType = 'wall';
                        if (x === 1 && y === 1) cellType = 'start';
                        else if (status.is_goal) cellType = 'goal';
                        else if (status.available_directions.length > 0) cellType = 'path';
                        maze[y][x] = cellType;
                    } catch (error) {
                        maze[y][x] = 'wall';
                    }
                }
            }
            return maze;
        }
        
        let mazeStructure = null;
        
        async function drawMaze() {
            if (!gameData) return;
            if (!mazeStructure) mazeStructure = await fetchMazeStructure();
            
            const cellSize = canvas.width / 31;
            ctx.clearRect(0, 0, canvas.width, canvas.height);
            ctx.fillStyle = COLORS.background;
            ctx.fillRect(0, 0, canvas.width, canvas.height);
            
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
                        ctx.fillStyle = color;
                        ctx.beginPath();
                        ctx.arc(x * cellSize + cellSize/2, y * cellSize + cellSize/2, cellSize * 0.35, 0, 2 * Math.PI);
                        ctx.fill();
                        ctx.strokeStyle = 'white';
                        ctx.lineWidth = 2;
                        ctx.stroke();
                    }
                }
            }
            
            Object.entries(gameData.explorations).forEach(([id, exp]) => {
                drawExploration(exp, cellSize);
            });
        }
        
        function drawExploration(exp, cellSize) {
            if (exp.path_positions.length < 2) return;
            
            let color = COLORS.segmentColors[0];
            if (exp.found_goal) color = COLORS.winnerColor;
            else if (exp.is_dead) color = COLORS.deadColor;
            else {
                const colorIndex = parseInt(exp.id.replace('s', '')) % COLORS.segmentColors.length;
                color = COLORS.segmentColors[colorIndex] || COLORS.segmentColors[0];
            }
            
            ctx.strokeStyle = color;
            ctx.lineWidth = exp.found_goal ? 3 : 2;
            ctx.lineCap = 'round';
            ctx.lineJoin = 'round';
            ctx.globalAlpha = exp.found_goal ? 1.0 : 0.9;
            
            ctx.beginPath();
            const firstPos = exp.path_positions[0];
            ctx.moveTo(firstPos.x * cellSize + cellSize/2, firstPos.y * cellSize + cellSize/2);
            
            for (let i = 1; i < exp.path_positions.length; i++) {
                const pos = exp.path_positions[i];
                ctx.lineTo(pos.x * cellSize + cellSize/2, pos.y * cellSize + cellSize/2);
            }
            ctx.stroke();
            ctx.globalAlpha = 1.0;
            
            if (exp.is_active) {
                const pos = exp.current_position;
                const centerX = pos.x * cellSize + cellSize/2;
                const centerY = pos.y * cellSize + cellSize/2;
                const size = cellSize * 0.3;
                
                ctx.fillStyle = color;
                ctx.beginPath();
                ctx.moveTo(centerX, centerY - size);
                ctx.lineTo(centerX + size, centerY);
                ctx.lineTo(centerX, centerY + size);
                ctx.lineTo(centerX - size, centerY);
                ctx.closePath();
                ctx.fill();
                
                ctx.fillStyle = 'white';
                ctx.beginPath();
                ctx.moveTo(centerX, centerY - size * 0.5);
                ctx.lineTo(centerX + size * 0.5, centerY);
                ctx.lineTo(centerX, centerY + size * 0.5);
                ctx.lineTo(centerX - size * 0.5, centerY);
                ctx.closePath();
                ctx.fill();
            }
        }
        
        setInterval(fetchGameState, 1000);
        fetchGameState();
    </script>
</body>
</html>`)
}

func handleRender(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate PNG content
	pngData, err := generateMazePNG()
	if err != nil {
		http.Error(w, "Failed to generate maze image", http.StatusInternalServerError)
		return
	}

	// Return PNG content
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", "attachment; filename=\"maze.png\"")
	w.Write(pngData)
}

func generateMazePNG() ([]byte, error) {
	cellSize := 20
	mazeWidth := game.Width * cellSize
	mazeHeight := game.Height * cellSize
	
	// Add title area at top
	titleHeight := 60
	totalWidth := mazeWidth
	totalHeight := mazeHeight + titleHeight

	// Create image with title area
	img := image.NewRGBA(image.Rect(0, 0, totalWidth, totalHeight))

	// Colors
	colors := map[string]color.RGBA{
		"background": {255, 255, 255, 255}, // White
		"wall":       {224, 224, 224, 255}, // Light gray
		"path":       {250, 250, 250, 255}, // Very light gray
		"start":      {76, 175, 80, 255},   // Green
		"goal":       {244, 67, 54, 255},   // Red
		"winner":     {255, 109, 0, 255},   // Gold
		"dead":       {158, 158, 158, 255}, // Gray
	}

	segmentColors := []color.RGBA{
		{33, 150, 243, 255},  // Blue
		{156, 39, 176, 255},  // Purple
		{255, 87, 34, 255},   // Deep Orange
		{139, 195, 74, 255},  // Light Green
		{0, 188, 212, 255},   // Cyan
		{233, 30, 99, 255},   // Pink
	}

	// Fill background
	draw.Draw(img, img.Bounds(), &image.Uniform{colors["background"]}, image.ZP, draw.Src)

	// Draw title area background
	titleBg := color.RGBA{248, 249, 250, 255} // Light gray background for title
	for y := 0; y < titleHeight; y++ {
		for x := 0; x < totalWidth; x++ {
			img.Set(x, y, titleBg)
		}
	}
	
	// Draw title content
	drawTitle(img, totalWidth, titleHeight)

	// Draw maze structure (offset by title height)
	for y := 0; y < game.Height; y++ {
		for x := 0; x < game.Width; x++ {
			cellType := game.Maze[y][x]
			var cellColor color.RGBA

			switch cellType {
			case WALL:
				cellColor = colors["wall"]
			case PATH:
				cellColor = colors["path"]
			case START:
				cellColor = colors["start"]
			case GOAL:
				cellColor = colors["goal"]
			}

			// Fill cell (offset by title height)
			for py := y*cellSize + titleHeight; py < (y+1)*cellSize+titleHeight; py++ {
				for px := x * cellSize; px < (x+1)*cellSize; px++ {
					img.Set(px, py, cellColor)
				}
			}
		}
	}

	// Draw exploration paths
	for _, exp := range game.Explorations {
		if len(exp.PathPositions) < 2 {
			continue
		}

		// Determine color
		var pathColor color.RGBA
		if exp.FoundGoal {
			pathColor = colors["winner"]
		} else if exp.IsDead {
			pathColor = colors["dead"]
		} else {
			// Extract numeric ID for consistent coloring
			idStr := strings.TrimPrefix(exp.ID, "s")
			if idStr == "" || exp.ID == "root" {
				idStr = "0"
			}
			if id, err := strconv.Atoi(idStr); err == nil {
				pathColor = segmentColors[id%len(segmentColors)]
			} else {
				pathColor = segmentColors[0]
			}
		}

		// Draw path (simple line drawing) - offset by title height
		for i := 1; i < len(exp.PathPositions); i++ {
			prev := exp.PathPositions[i-1]
			curr := exp.PathPositions[i]
			
			x1 := prev.X*cellSize + cellSize/2
			y1 := prev.Y*cellSize + cellSize/2 + titleHeight
			x2 := curr.X*cellSize + cellSize/2
			y2 := curr.Y*cellSize + cellSize/2 + titleHeight

			drawLine(img, x1, y1, x2, y2, pathColor, 2)
		}

		// Draw robot marker for active explorations
		if exp.IsActive {
			pos := exp.CurrentPosition
			centerX := pos.X*cellSize + cellSize/2
			centerY := pos.Y*cellSize + cellSize/2 + titleHeight
			size := cellSize / 4

			// Draw diamond shape
			drawDiamond(img, centerX, centerY, size, pathColor)
		}
	}

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Helper function to draw title
func drawTitle(img *image.RGBA, width, height int) {
	// Get current statistics
	stats := game.getExplorationTree()
	
	// Colors
	bgColor := color.RGBA{248, 249, 250, 255}     // Light background
	activeColor := color.RGBA{33, 150, 243, 255} // Blue for active
	goalColor := color.RGBA{76, 175, 80, 255}    // Green for goal
	deadColor := color.RGBA{158, 158, 158, 255}  // Gray for dead
	
	// Clear title area with light background
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, bgColor)
		}
	}
	
	// Draw status indicators as colored bars
	barHeight := 8
	barY := height/2 - barHeight/2
	
	// Active explorations (blue bars)
	activeWidth := min(stats.GlobalStats.ActiveExplorations*15, width-100)
	for y := barY; y < barY+barHeight; y++ {
		for x := 20; x < 20+activeWidth; x++ {
			img.Set(x, y, activeColor)
		}
	}
	
	// Total explorations count (smaller gray bars above active)
	totalWidth := min(stats.GlobalStats.TotalExplorations*3, width-100)
	for y := barY-12; y < barY-8; y++ {
		for x := 20; x < 20+totalWidth; x++ {
			img.Set(x, y, deadColor)
		}
	}
	
	// Goal indicator (big green square if found)
	if stats.GlobalStats.GoalFound {
		goalSize := 20
		goalX := width - 40
		goalY := height/2 - goalSize/2
		for y := goalY; y < goalY+goalSize; y++ {
			for x := goalX; x < goalX+goalSize; x++ {
				img.Set(x, y, goalColor)
			}
		}
		// White checkmark inside
		for i := 0; i < 8; i++ {
			img.Set(goalX+6+i, goalY+10+i/2, color.RGBA{255, 255, 255, 255})
			if i < 4 {
				img.Set(goalX+6-i, goalY+10+i, color.RGBA{255, 255, 255, 255})
			}
		}
	}
}

// Helper function to draw simple pixel text (simplified version)
func drawPixelText(img *image.RGBA, text string, startX, startY int, textColor color.RGBA) {
	// This is a very basic pixel font implementation
	// For now, just draw simple blocks to represent text
	for i, char := range text {
		if char == ' ' {
			continue
		}
		
		// Draw a simple block for each character
		for dy := 0; dy < 5; dy++ {
			for dx := 0; dx < 3; dx++ {
				x := startX + i*4 + dx
				y := startY + dy
				if x < img.Bounds().Max.X && y < img.Bounds().Max.Y {
					img.Set(x, y, textColor)
				}
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Helper function to draw line
func drawLine(img *image.RGBA, x0, y0, x1, y1 int, color color.RGBA, width int) {
	dx := abs(x1 - x0)
	dy := abs(y1 - y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy

	x, y := x0, y0
	for {
		// Draw thick line by drawing multiple pixels
		for i := -width/2; i <= width/2; i++ {
			for j := -width/2; j <= width/2; j++ {
				if x+i >= 0 && y+j >= 0 && x+i < img.Bounds().Max.X && y+j < img.Bounds().Max.Y {
					img.Set(x+i, y+j, color)
				}
			}
		}

		if x == x1 && y == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x += sx
		}
		if e2 < dx {
			err += dx
			y += sy
		}
	}
}

// Helper function to draw diamond
func drawDiamond(img *image.RGBA, centerX, centerY, size int, color color.RGBA) {
	for dy := -size; dy <= size; dy++ {
		width := size - abs(dy)
		for dx := -width; dx <= width; dx++ {
			x := centerX + dx
			y := centerY + dy
			if x >= 0 && y >= 0 && x < img.Bounds().Max.X && y < img.Bounds().Max.Y {
				img.Set(x, y, color)
			}
		}
	}
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