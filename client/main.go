package main

import (
	"bytes"
	"encoding/json"
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

type AvailableMove struct {
	Direction      Direction `json:"direction"`
	TargetPosition Position  `json:"target_position"`
}

type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type MazeStatusResponse struct {
	IsExplored           bool            `json:"is_explored"`
	IsJunction           bool            `json:"is_junction"`
	AvailableDirections  []Direction     `json:"available_directions"`
	AvailableMoves       []AvailableMove `json:"available_moves"`
	IsGoal               bool            `json:"is_goal"`
	GoalReachedByAny     bool            `json:"goal_reached_by_any"`
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

const ConfigFile = ".maze_server"

func loadServerConfig() {
	// Try to load from config file
	data, err := os.ReadFile(ConfigFile)
	if err == nil {
		ServerURL = strings.TrimSpace(string(data))
	} else {
		// Default server connection
		ServerURL = "http://localhost:8079"
	}
}

func saveServerConfig(host, port string) error {
	ServerURL = fmt.Sprintf("http://%s:%s", host, port)
	return os.WriteFile(ConfigFile, []byte(ServerURL), 0644)
}

func handleSetCommand(host, port string) {
	fmt.Printf("🔧 Setting server to %s:%s...\n", host, port)
	
	if err := saveServerConfig(host, port); err != nil {
		fmt.Printf("❌ Error saving config: %v\n", err)
		return
	}
	
	fmt.Printf("✅ Server set to %s\n", ServerURL)
	fmt.Println("💡 Configuration saved to .maze_server")
}

func main() {
	// Load server configuration
	loadServerConfig()

	// Parse command line arguments
	args := os.Args[1:] // Skip program name

	if len(args) == 0 {
		// No arguments: reset game
		resetGame()
		return
	}

	command := args[0]

	switch command {
	case "set":
		if len(args) != 3 {
			fmt.Println("❌ Usage: maze_client set <host> <port>")
			fmt.Println("   Example: maze_client set 34.169.25.230 8079")
			return
		}
		handleSetCommand(args[1], args[2])
		
	case "start":
		if len(args) != 4 {
			fmt.Println("❌ Usage: maze_client start <exploration_name> <x> <y>")
			fmt.Println("   Example: maze_client start root 1 1")
			return
		}
		handleStartCommand(args[1], args[2], args[3])
		
	case "status":
		if len(args) != 2 {
			fmt.Println("❌ Usage: maze_client status <exploration_name>")
			fmt.Println("   Example: maze_client status root")
			return
		}
		handleStatusCommand(args[1])
		
	case "move":
		if len(args) != 4 {
			fmt.Println("❌ Usage: maze_client move <exploration_name> <x> <y>")
			fmt.Println("   Example: maze_client move root 2 1")
			return
		}
		handleMoveCommand(args[1], args[2], args[3])
		
	case "render":
		handleRenderCommand()
		
	case "tree":
		handleTreeCommand()
		
	default:
		showUsage()
	}
}

func resetGame() {
	fmt.Printf("🔄 Resetting game on %s...\n", ServerURL)
	
	resp, err := http.Post(fmt.Sprintf("%s/reset", ServerURL), "application/json", strings.NewReader("{}"))
	if err != nil {
		fmt.Printf("❌ Error connecting to server: %v\n", err)
		fmt.Println("💡 Make sure server is running and accessible")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("❌ Server error: %s\n", resp.Status)
		return
	}

	fmt.Println("✅ Game reset successfully")
	fmt.Println("💡 Start a new exploration with: maze_client start root 1 1")
}

func handleStartCommand(name, xStr, yStr string) {
	x, err1 := strconv.Atoi(xStr)
	y, err2 := strconv.Atoi(yStr)

	if err1 != nil || err2 != nil {
		fmt.Println("❌ Invalid coordinates. Use integers.")
		return
	}

	fmt.Printf("🚀 Starting exploration '%s' at (%d, %d)...\n", name, x, y)
	
	moveReq := MoveRequest{
		ExplorationName: name,
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
		fmt.Printf("💡 Check status with: maze_client status %s\n", name)
	} else {
		fmt.Printf("❌ Start failed: %s\n", moveResp.Message)
	}
}

func showUsage() {
	fmt.Println("🎮 Maze Game Client")
	fmt.Println("==================")
	fmt.Printf("Current server: %s\n", ServerURL)
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  maze_client                           - Reset game (clear all explorations)")
	fmt.Println("  maze_client set <host> <port>         - Set server address")
	fmt.Println("  maze_client start <name> <x> <y>      - Start new exploration")
	fmt.Println("  maze_client status <name>             - Check exploration status")
	fmt.Println("  maze_client move <name> <x> <y>       - Move exploration")
	fmt.Println("  maze_client render                    - Generate maze image")
	fmt.Println("  maze_client tree                      - Show exploration tree")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  maze_client set 34.169.25.230 8079")
	fmt.Println("  maze_client start root 1 1")
	fmt.Println("  maze_client status root")
	fmt.Println("  maze_client move root 2 1")
	fmt.Println()
}

func handleStatusCommand(explorationName string) {
	fmt.Printf("🔍 Checking status of exploration '%s'...\n", explorationName)
	
	url := fmt.Sprintf("%s/exploration-status?name=%s", ServerURL, explorationName)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("❌ Error connecting to server: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		fmt.Printf("❌ Exploration '%s' not found\n", explorationName)
		fmt.Println("💡 Use: maze_client tree (to see all explorations)")
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

	fmt.Printf("📍 Exploration '%s' status:\n", explorationName)
	displayMazeStatus(status)
}


func displayMazeStatus(status MazeStatusResponse) {
	fmt.Printf("  🔍 Explored: %v\n", status.IsExplored)
	fmt.Printf("  🛤️  Junction: %v\n", status.IsJunction)
	fmt.Printf("  🎯 Goal: %v\n", status.IsGoal)
	fmt.Printf("  🏆 Any reached goal: %v\n", status.GoalReachedByAny)
	
	if len(status.AvailableMoves) == 0 {
		fmt.Printf("  ➡️  Available moves: None (blocked/wall)\n")
	} else {
		fmt.Printf("  ➡️  Available moves (%d):\n", len(status.AvailableMoves))
		for i, move := range status.AvailableMoves {
			dirName := getDirectionName(move.Direction)
			fmt.Printf("    %d. %s to (%d, %d)\n", i+1, dirName, move.TargetPosition.X, move.TargetPosition.Y)
		}
		fmt.Printf("  💡 Use: maze_client move <exploration_name> <target_x> <target_y>\n")
	}
}

func handleMoveCommand(explorationName, xStr, yStr string) {
	x, err1 := strconv.Atoi(xStr)
	y, err2 := strconv.Atoi(yStr)

	if err1 != nil || err2 != nil {
		fmt.Println("❌ Invalid coordinates. Use integers.")
		return
	}

	fmt.Printf("🚀 Moving exploration '%s' to (%d, %d)...\n", explorationName, x, y)
	
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

