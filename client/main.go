package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type Direction struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
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

type ExplorationTreeResponse struct {
	Explorations map[string]*Exploration `json:"explorations"`
	GlobalStats  struct {
		TotalExplorations  int  `json:"total_explorations"`
		ActiveExplorations int  `json:"active_explorations"`
		GoalFound          bool `json:"goal_found"`
		VisitedPositions   int  `json:"visited_positions_count"`
	} `json:"global_stats"`
}

var ServerURL string


func main() {
	// Command line flags
	host := flag.String("host", "localhost", "Server host")
	port := flag.String("port", "8079", "Server port")
	command := flag.String("c", "", "Execute single command and exit (non-interactive)")
	flag.Parse()

	ServerURL = fmt.Sprintf("http://%s:%s", *host, *port)

	// Non-interactive mode
	if *command != "" {
		fmt.Printf("🎮 Maze Game Client - %s\n", ServerURL)
		executeCommand(*command)
		return
	}

	// Interactive mode
	fmt.Printf("🎮 Maze Game Client - connecting to %s\n", ServerURL)
	fmt.Println("==================================================")
	fmt.Println("Available commands:")
	fmt.Println("  status <exploration>        - Check exploration's current position")
	fmt.Println("  status <x> <y>              - Check specific coordinates")
	fmt.Println("  move <exploration> <x> <y>  - Move exploration to position")
	fmt.Println("  tree                        - Show exploration tree")
	fmt.Println("  render                      - Generate and save maze image")
	fmt.Println("  help                        - Show this help")
	fmt.Println("  quit                        - Exit client")
	fmt.Println()
	fmt.Println("💡 Non-interactive mode: use -c \"command\" to run single command")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("maze> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		executeCommand(line)
	}
}

func executeCommand(line string) {
	args := strings.Fields(line)
	if len(args) == 0 {
		return
	}

	command := args[0]

	switch command {
	case "status":
		handleStatusCommand(args)
	case "move":
		handleMoveCommand(args)
	case "tree":
		handleTreeCommand()
	case "render":
		handleRenderCommand()
	case "help":
		showHelp()
	case "quit", "exit":
		fmt.Println("👋 Goodbye!")
		os.Exit(0)
	default:
		fmt.Printf("❌ Unknown command: %s. Type 'help' for available commands.\n", command)
	}
}

func handleStatusCommand(args []string) {
	if len(args) == 2 {
		// Single argument: could be exploration name or invalid
		explorationName := args[1]
		
		// Try to call exploration-status API first
		url := fmt.Sprintf("%s/exploration-status?name=%s", ServerURL, explorationName)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("❌ Error connecting to server: %v\n", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == 404 {
			fmt.Printf("❌ Exploration '%s' not found. Use 'tree' to see available explorations.\n", explorationName)
			return
		} else if resp.StatusCode != 200 {
			fmt.Printf("❌ Server error: %s\n", resp.Status)
			return
		}

		var status MazeStatusResponse
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			fmt.Printf("❌ Error parsing response: %v\n", err)
			return
		}

		fmt.Printf("🔍 Exploration '%s' current status:\n", explorationName)
		displayMazeStatus(status, -1, -1) // -1, -1 indicates exploration query
		return
		
	} else if len(args) == 3 {
		// Two arguments: coordinates query
		x, err1 := strconv.Atoi(args[1])
		y, err2 := strconv.Atoi(args[2])

		if err1 != nil || err2 != nil {
			fmt.Println("❌ Invalid coordinates. Use integers.")
			return
		}

		// Call maze-status API
		queryMazeStatus(x, y)
		return
		
	} else {
		fmt.Println("❌ Usage:")
		fmt.Println("   status <exploration_name>  - Check exploration's current position")
		fmt.Println("   status <x> <y>            - Check specific coordinates")
		fmt.Println("💡 Use 'tree' to see available explorations")
		return
	}
}

func queryMazeStatus(x, y int) {
	url := fmt.Sprintf("%s/maze-status?x=%d&y=%d", ServerURL, x, y)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("❌ Error connecting to server: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("❌ Server error: %s\n", resp.Status)
		return
	}

	var status MazeStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		fmt.Printf("❌ Error parsing response: %v\n", err)
		return
	}

	fmt.Printf("📍 Position (%d, %d):\n", x, y)
	displayMazeStatus(status, x, y)
}

func displayMazeStatus(status MazeStatusResponse, x, y int) {
	fmt.Printf("  🔍 Explored: %v\n", status.IsExplored)
	fmt.Printf("  🛤️  Junction: %v\n", status.IsJunction)
	fmt.Printf("  🎯 Goal: %v\n", status.IsGoal)
	fmt.Printf("  🏆 Any reached goal: %v\n", status.GoalReachedByAny)
	
	if len(status.AvailableDirections) == 0 {
		fmt.Printf("  ➡️  Available moves: None (blocked/wall)\n")
		if x == 0 && y == 0 {
			fmt.Printf("  💡 Hint: Start position is (1, 1). Try: status 1 1\n")
		}
	} else {
		fmt.Printf("  ➡️  Available moves (%d):\n", len(status.AvailableDirections))
		
		if x == -1 && y == -1 {
			// Exploration query - don't show coordinates since we don't know current position
			for i, dir := range status.AvailableDirections {
				dirName := getDirectionName(dir)
				fmt.Printf("    %d. %s\n", i+1, dirName)
			}
		} else {
			// Coordinate query - show target coordinates
			for i, dir := range status.AvailableDirections {
				dirName := getDirectionName(dir)
				targetX := x + dir.X
				targetY := y + dir.Y
				fmt.Printf("    %d. %s to (%d, %d)\n", i+1, dirName, targetX, targetY)
			}
		}
		
		fmt.Printf("  💡 Use: move <exploration_name> <target_x> <target_y>\n")
		if x == 1 && y == 1 && !status.IsExplored {
			fmt.Printf("  🚀 Quick start: move root 1 1\n")
		}
	}
}

func handleMoveCommand(args []string) {
	if len(args) != 4 {
		fmt.Println("❌ Usage: move <exploration_name> <x> <y>")
		return
	}

	explorationName := args[1]
	x, err1 := strconv.Atoi(args[2])
	y, err2 := strconv.Atoi(args[3])

	if err1 != nil || err2 != nil {
		fmt.Println("❌ Invalid coordinates. Use integers.")
		return
	}

	moveReq := MoveRequest{
		ExplorationName: explorationName,
		NextPosition:    Position{x, y},
	}

	jsonData, err := json.Marshal(moveReq)
	if err != nil {
		fmt.Printf("❌ Error creating request: %v\n", err)
		return
	}

	resp, err := http.Post(fmt.Sprintf("%s/move", ServerURL), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("❌ Error connecting to server: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var moveResp MoveResponse
	if err := json.NewDecoder(resp.Body).Decode(&moveResp); err != nil {
		fmt.Printf("❌ Error parsing response: %v\n", err)
		return
	}

	if moveResp.Success {
		fmt.Printf("✅ %s\n", moveResp.Message)
		fmt.Printf("   📊 Status: %s\n", moveResp.NewStatus)
	} else {
		fmt.Printf("❌ Move failed: %s\n", moveResp.Message)
		fmt.Printf("   📊 Status: %s\n", moveResp.NewStatus)
	}
}

func handleTreeCommand() {
	resp, err := http.Get(fmt.Sprintf("%s/exploration-tree", ServerURL))
	if err != nil {
		fmt.Printf("❌ Error connecting to server: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("❌ Server error: %s\n", resp.Status)
		return
	}

	var tree ExplorationTreeResponse
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		fmt.Printf("❌ Error parsing response: %v\n", err)
		return
	}

	fmt.Printf("🌳 Exploration Tree:\n")
	fmt.Printf("   📊 Total: %d | Active: %d | Goal: %v | Visited: %d\n\n", 
		tree.GlobalStats.TotalExplorations,
		tree.GlobalStats.ActiveExplorations,
		tree.GlobalStats.GoalFound,
		tree.GlobalStats.VisitedPositions)

	if len(tree.Explorations) == 0 {
		fmt.Println("   (No explorations yet)")
		return
	}

	for id, exp := range tree.Explorations {
		status := getExplorationStatus(exp)
		parentInfo := "root"
		if exp.ParentID != nil {
			parentInfo = *exp.ParentID
		}
		
		fmt.Printf("🔍 %s [%s] (gen: %d, parent: %s)\n", id, status, exp.Generation, parentInfo)
		fmt.Printf("   📍 Current: (%d, %d)\n", exp.CurrentPosition.X, exp.CurrentPosition.Y)
		fmt.Printf("   🛤️  Path length: %d steps\n", len(exp.PathPositions))
		if len(exp.ChildIDs) > 0 {
			fmt.Printf("   👶 Children: %v\n", exp.ChildIDs)
		}
		fmt.Println()
	}
}

func getExplorationStatus(exp *Exploration) string {
	if exp.FoundGoal {
		return "🎯 GOAL"
	}
	if exp.IsDead {
		return "💀 DEAD"
	}
	if exp.IsActive {
		return "🚀 ACTIVE"
	}
	if exp.IsComplete {
		return "✅ COMPLETE"
	}
	return "❓ UNKNOWN"
}

func getDirectionName(dir Direction) string {
	switch {
	case dir.X == 0 && dir.Y == -1:
		return "UP"
	case dir.X == 0 && dir.Y == 1:
		return "DOWN"
	case dir.X == -1 && dir.Y == 0:
		return "LEFT"
	case dir.X == 1 && dir.Y == 0:
		return "RIGHT"
	default:
		return "UNKNOWN"
	}
}

func handleRenderCommand() {
	resp, err := http.Post(fmt.Sprintf("%s/render", ServerURL), "application/json", strings.NewReader("{}"))
	if err != nil {
		fmt.Printf("❌ Error connecting to server: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("❌ Server error: %s\n", resp.Status)
		return
	}

	// Read PNG content from response
	pngContent, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ Error reading response: %v\n", err)
		return
	}

	// Save to local file
	filename := "maze.png"
	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("❌ Error creating file: %v\n", err)
		return
	}
	defer file.Close()

	_, err = file.Write(pngContent)
	if err != nil {
		fmt.Printf("❌ Error writing file: %v\n", err)
		return
	}

	fmt.Printf("✅ Maze rendered successfully!\n")
	fmt.Printf("   📁 File: %s\n", filename)
	fmt.Printf("   📂 Size: %d bytes\n", len(pngContent))
}

func showHelp() {
	fmt.Println("\n📖 Command Help:")
	fmt.Println("==================")
	fmt.Println("🔍 status <exploration_name>")
	fmt.Println("   Check an exploration's current position and available moves")
	fmt.Println("   Example: status root")
	fmt.Println()
	fmt.Println("🔍 status <x> <y>")
	fmt.Println("   Get information about a specific maze position")
	fmt.Println("   Example: status 5 10")
	fmt.Println()
	fmt.Println("🚀 move <exploration_name> <x> <y>")
	fmt.Println("   Move an exploration to a new position")
	fmt.Println("   Creates new exploration if it doesn't exist")
	fmt.Println("   Example: move root 2 1")
	fmt.Println()
	fmt.Println("🌳 tree")
	fmt.Println("   Display the complete exploration tree")
	fmt.Println("   Shows all explorations, their status, and relationships")
	fmt.Println()
	fmt.Println("🖼️ render")
	fmt.Println("   Generate and save a PNG image of current maze state")
	fmt.Println("   Image is saved locally as 'maze.png'")
	fmt.Println("   Example: render")
	fmt.Println()
	fmt.Println("💡 Tips:")
	fmt.Println("   - Start by creating root exploration: move root 1 1")
	fmt.Println("   - Check exploration status with: status root")
	fmt.Println("   - Use 'tree' to see all explorations")
	fmt.Println("   - At junctions, create multiple explorations to branch")
	fmt.Println("   - Use 'render' to save visualization snapshots")
	fmt.Println()
}