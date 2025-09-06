#!/usr/bin/env python3
"""
Pantheon Maze Explorer - PathSegment Tree Based BFS
Complete rewrite using PathSegment as the sole exploration unit.
Each PathSegment represents exploration from one junction to another.
"""

import random
import time
import os
import shutil
import json
from collections import deque
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Set, Tuple
from enum import Enum
import matplotlib.pyplot as plt
import matplotlib.patches as patches
import numpy as np

# Optional imports for GIF creation
try:
    from PIL import Image
    PIL_AVAILABLE = True
except ImportError:
    PIL_AVAILABLE = False
    print("⚠️  PIL not available - install Pillow for GIF generation")


class Direction(Enum):
    UP = (0, -1)
    DOWN = (0, 1)
    LEFT = (-1, 0)
    RIGHT = (1, 0)


class CellType(Enum):
    WALL = 0
    PATH = 1
    START = 2
    GOAL = 3


@dataclass
class Position:
    x: int
    y: int
    
    def __hash__(self):
        return hash((self.x, self.y))
    
    def __eq__(self, other):
        return isinstance(other, Position) and self.x == other.x and self.y == other.y
    
    def __add__(self, direction: Direction):
        dx, dy = direction.value
        return Position(self.x + dx, self.y + dy)
    
    def to_dict(self) -> Dict:
        """Serialize Position to dictionary"""
        return {"x": self.x, "y": self.y}
    
    @classmethod
    def from_dict(cls, data: Dict) -> 'Position':
        """Deserialize Position from dictionary"""
        return cls(data["x"], data["y"])


@dataclass
class PathSegment:
    """A single path segment - the ONLY exploration unit"""
    id: str
    start_position: Position
    current_position: Position
    path_positions: List[Position] = field(default_factory=list)
    
    # Tree structure
    parent_id: Optional[str] = None
    child_ids: List[str] = field(default_factory=list)
    
    # State
    is_active: bool = True           # Currently exploring
    is_complete: bool = False        # Reached junction, goal, or dead end
    is_dead: bool = False           # Hit dead end or collision
    found_goal: bool = False        # Reached goal
    
    # Visual properties
    fixed_color_index: int = 0      # Fixed color assigned at creation
    generation: int = 0             # Distance from root in tree
    
    def __post_init__(self):
        if not self.path_positions:
            self.path_positions = [self.start_position]
        if self.current_position not in self.path_positions:
            self.path_positions.append(self.current_position)
    
    def to_dict(self) -> Dict:
        """Serialize PathSegment to dictionary"""
        return {
            "id": self.id,
            "start_position": self.start_position.to_dict(),
            "current_position": self.current_position.to_dict(),
            "path_positions": [pos.to_dict() for pos in self.path_positions],
            "parent_id": self.parent_id,
            "child_ids": self.child_ids.copy(),
            "is_active": self.is_active,
            "is_complete": self.is_complete,
            "is_dead": self.is_dead,
            "found_goal": self.found_goal,
            "fixed_color_index": self.fixed_color_index,
            "generation": self.generation
        }
    
    @classmethod
    def from_dict(cls, data: Dict) -> 'PathSegment':
        """Deserialize PathSegment from dictionary"""
        return cls(
            id=data["id"],
            start_position=Position.from_dict(data["start_position"]),
            current_position=Position.from_dict(data["current_position"]),
            path_positions=[Position.from_dict(pos) for pos in data["path_positions"]],
            parent_id=data["parent_id"],
            child_ids=data["child_ids"].copy(),
            is_active=data["is_active"],
            is_complete=data["is_complete"],
            is_dead=data["is_dead"],
            found_goal=data["found_goal"],
            fixed_color_index=data["fixed_color_index"],
            generation=data["generation"]
        )


class MazeGenerator:
    """Generate complex mazes using recursive backtracking"""
    
    @staticmethod
    def generate(width: int, height: int, seed: Optional[int] = None) -> np.ndarray:
        if seed is not None:
            random.seed(seed)
        
        # Ensure odd dimensions
        if width % 2 == 0:
            width += 1
        if height % 2 == 0:
            height += 1
        
        maze = np.zeros((height, width), dtype=int)
        
        # Initialize paths at odd coordinates
        for y in range(1, height - 1, 2):
            for x in range(1, width - 1, 2):
                maze[y][x] = CellType.PATH.value
        
        # Recursive backtracking algorithm
        stack = [(1, 1)]
        visited = {(1, 1)}
        directions = [(0, -2), (2, 0), (0, 2), (-2, 0)]
        
        while stack:
            current_x, current_y = stack[-1]
            
            # Find unvisited neighbors
            neighbors = []
            for dx, dy in directions:
                nx, ny = current_x + dx, current_y + dy
                if (1 <= nx < width - 1 and 1 <= ny < height - 1 and 
                    (nx, ny) not in visited):
                    neighbors.append((nx, ny))
            
            if neighbors:
                # Choose random neighbor
                next_x, next_y = random.choice(neighbors)
                visited.add((next_x, next_y))
                
                # Remove wall between current and next
                wall_x = current_x + (next_x - current_x) // 2
                wall_y = current_y + (next_y - current_y) // 2
                maze[wall_y][wall_x] = CellType.PATH.value
                
                stack.append((next_x, next_y))
            else:
                stack.pop()
        
        # Add some extra connections for complexity
        for _ in range(width * height // 30):
            x = random.randrange(2, width - 2, 2)
            y = random.randrange(2, height - 2, 2)
            
            for dx, dy in [(0, 1), (1, 0), (0, -1), (-1, 0)]:
                nx, ny = x + dx, y + dy
                if (0 <= nx < width and 0 <= ny < height and 
                    maze[ny][nx] == CellType.PATH.value):
                    maze[y][x] = CellType.PATH.value
                    break
        
        # Set start and goal
        maze[1][1] = CellType.START.value
        
        # Find optimal goal position (furthest from start)
        max_dist = 0
        best_goal = (width - 2, height - 2)
        for y in range(1, height - 1, 2):
            for x in range(1, width - 1, 2):
                if maze[y][x] == CellType.PATH.value:
                    dist = abs(x - 1) + abs(y - 1)
                    if dist > max_dist:
                        max_dist = dist
                        best_goal = (x, y)
        
        maze[best_goal[1]][best_goal[0]] = CellType.GOAL.value
        return maze


class PathSegmentEngine:
    """Pure PathSegment-based exploration engine"""
    
    def __init__(self, width: int = 31, height: int = 31, seed: Optional[int] = None):
        self.maze = MazeGenerator.generate(width, height, seed)
        self.height, self.width = self.maze.shape
        
        # Find start and goal positions
        start_pos = np.where(self.maze == CellType.START.value)
        self.start = Position(start_pos[1][0], start_pos[0][0])
        
        goal_pos = np.where(self.maze == CellType.GOAL.value)
        self.goal = Position(goal_pos[1][0], goal_pos[0][0])
        
        # PathSegment tree management
        self.segments: Dict[str, PathSegment] = {}
        self.next_id = 0
        
        # Global exploration state - CRITICAL for collision detection
        self.global_visited_positions: Set[Position] = set()
        
        # Goal tracking
        self.goal_found = False
        self.winning_segment = None
        self.show_only_winner = False
        
        # Statistics
        self.total_steps = 0
        self.max_concurrent_segments = 0
        
        # Create root segment
        self._create_root_segment()
    
    def _create_root_segment(self):
        """Create the initial root segment"""
        root = PathSegment(
            id="root",
            start_position=self.start,
            current_position=self.start,
            fixed_color_index=0,
            generation=0
        )
        self.segments["root"] = root
        self.global_visited_positions.add(self.start)
    
    def _generate_id(self) -> str:
        """Generate unique segment ID"""
        segment_id = f"s{self.next_id}"
        self.next_id += 1
        return segment_id
    
    def _is_walkable(self, pos: Position) -> bool:
        """Check if position is walkable (not wall)"""
        if not (0 <= pos.x < self.width and 0 <= pos.y < self.height):
            return False
        return self.maze[pos.y][pos.x] != CellType.WALL.value
    
    def _is_collision(self, pos: Position) -> bool:
        """Check if position collides with already explored territory"""
        return pos in self.global_visited_positions
    
    def _get_valid_directions(self, pos: Position) -> List[Tuple[Direction, Position]]:
        """Get valid movement directions that don't cause collisions"""
        valid = []
        for direction in Direction:
            new_pos = pos + direction
            if self._is_walkable(new_pos) and not self._is_collision(new_pos):
                valid.append((direction, new_pos))
        return valid
    
    def _is_at_goal(self, pos: Position) -> bool:
        """Check if position is the goal"""
        return pos.x == self.goal.x and pos.y == self.goal.y
    
    def get_child_segments(self, segment_id: str) -> List[PathSegment]:
        """Get direct child segments"""
        segment = self.segments.get(segment_id)
        if not segment:
            return []
        return [self.segments[child_id] for child_id in segment.child_ids 
                if child_id in self.segments]
    
    def step(self) -> bool:
        """Execute one exploration step for all active segments"""
        active_segments = [s for s in self.segments.values() if s.is_active]
        
        if not active_segments or self.goal_found:
            return False
        
        self.total_steps += 1
        self.max_concurrent_segments = max(self.max_concurrent_segments, len(active_segments))
        
        # Process all active segments
        new_segments_to_create = []
        
        for segment in list(active_segments):
            if not segment.is_active or self.goal_found:
                continue
            
            result = self._process_segment(segment)
            if result:
                new_segments_to_create.extend(result)
        
        # Create new segments after processing current level (true BFS)
        for segment_data in new_segments_to_create:
            self._create_child_segment(segment_data)
        
        return True
    
    def _process_segment(self, segment: PathSegment) -> Optional[List[Dict]]:
        """Process a single segment according to BFS rules"""
        current_pos = segment.current_position
        valid_moves = self._get_valid_directions(current_pos)
        
        if len(valid_moves) == 0:
            # Dead end - no valid moves (collision or wall)
            segment.is_active = False
            segment.is_dead = True
            segment.is_complete = True
            return None
            
        elif len(valid_moves) == 1:
            # Single path - continue with this segment
            direction, new_pos = valid_moves[0]
            self._move_segment_to_position(segment, new_pos)
            return None
            
        else:
            # Multiple paths - junction reached, complete segment and create children
            segment.is_active = False
            segment.is_complete = True
            
            children_to_create = []
            for direction, new_pos in valid_moves:
                children_to_create.append({
                    'parent_segment': segment,
                    'new_position': new_pos,
                    'direction': direction
                })
            
            return children_to_create
    
    def _move_segment_to_position(self, segment: PathSegment, new_pos: Position):
        """Move segment to new position"""
        segment.current_position = new_pos
        segment.path_positions.append(new_pos)
        self.global_visited_positions.add(new_pos)
        
        # Check if reached goal
        if self._is_at_goal(new_pos):
            segment.found_goal = True
            segment.is_active = False
            segment.is_complete = True
            self.goal_found = True
            self.winning_segment = segment.id
            print(f"🎯 GOAL REACHED by segment {segment.id} in {len(segment.path_positions)} steps!")
    
    def _create_child_segment(self, segment_data: Dict):
        """Create a child segment from parent segment data"""
        parent = segment_data['parent_segment']
        new_pos = segment_data['new_position']
        
        child_id = self._generate_id()
        
        # Assign color based on creation order
        color_index = self.next_id % 6  # 6 colors excluding gold/gray
        
        child = PathSegment(
            id=child_id,
            start_position=parent.current_position,  # Start from parent's junction
            current_position=new_pos,
            parent_id=parent.id,
            fixed_color_index=color_index,
            generation=parent.generation + 1
        )
        
        # Move to new position and mark as visited
        self.global_visited_positions.add(new_pos)
        
        # Link parent to child
        parent.child_ids.append(child_id)
        
        # Check if reached goal
        if self._is_at_goal(new_pos):
            child.found_goal = True
            child.is_active = False
            child.is_complete = True
            self.goal_found = True
            self.winning_segment = child_id
            print(f"🎯 GOAL REACHED by segment {child_id} in {len(child.path_positions)} steps!")
        
        self.segments[child_id] = child
    
    def get_statistics(self) -> Dict:
        """Get current exploration statistics"""
        active_count = len([s for s in self.segments.values() if s.is_active])
        complete_count = len([s for s in self.segments.values() if s.is_complete])
        dead_count = len([s for s in self.segments.values() if s.is_dead])
        goal_count = len([s for s in self.segments.values() if s.found_goal])
        
        return {
            'total_segments': len(self.segments),
            'active_segments': active_count,
            'complete_segments': complete_count,
            'dead_segments': dead_count,
            'successful_segments': goal_count,
            'total_steps': self.total_steps,
            'max_concurrent': self.max_concurrent_segments,
            'goal_found': self.goal_found,
            'winning_segment': self.winning_segment,
            'show_only_winner': self.show_only_winner,
            'visited_positions': len(self.global_visited_positions)
        }
    
    def enable_winner_only_mode(self):
        """Enable winner-only display mode"""
        if self.goal_found and self.winning_segment:
            self.show_only_winner = True
            print(f"🏆 Switched to winner-only mode - showing only winning path (segment: {self.winning_segment})")
        else:
            print("⚠️  Cannot enable winner-only mode - no winner found yet")
    
    def save_tree_to_json(self, filename: str = "pathsegment_tree.json"):
        """Save the complete PathSegment tree to JSON file"""
        tree_data = {
            "metadata": {
                "width": self.width,
                "height": self.height,
                "start": self.start.to_dict(),
                "goal": self.goal.to_dict(),
                "goal_found": self.goal_found,
                "winning_segment": self.winning_segment,
                "show_only_winner": self.show_only_winner,
                "total_steps": self.total_steps,
                "max_concurrent_segments": self.max_concurrent_segments,
                "next_id": self.next_id
            },
            "maze": self.maze.tolist(),  # Convert numpy array to list for JSON serialization
            "segments": {seg_id: segment.to_dict() for seg_id, segment in self.segments.items()},
            "global_visited_positions": [pos.to_dict() for pos in self.global_visited_positions]
        }
        
        with open(filename, 'w', encoding='utf-8') as f:
            json.dump(tree_data, f, indent=2, ensure_ascii=False)
        
        print(f"💾 PathSegment tree saved to '{filename}'")
        print(f"   📊 {len(self.segments)} segments, {len(self.global_visited_positions)} visited positions")
    
    def load_tree_from_json(self, filename: str = "pathsegment_tree.json"):
        """Load PathSegment tree from JSON file"""
        with open(filename, 'r', encoding='utf-8') as f:
            tree_data = json.load(f)
        
        # Restore metadata
        metadata = tree_data["metadata"]
        self.width = metadata["width"]
        self.height = metadata["height"]
        self.start = Position.from_dict(metadata["start"])
        self.goal = Position.from_dict(metadata["goal"])
        self.goal_found = metadata["goal_found"]
        self.winning_segment = metadata["winning_segment"]
        self.show_only_winner = metadata["show_only_winner"]
        self.total_steps = metadata["total_steps"]
        self.max_concurrent_segments = metadata["max_concurrent_segments"]
        self.next_id = metadata["next_id"]
        
        # Restore maze
        self.maze = np.array(tree_data["maze"])
        
        # Restore segments
        self.segments = {}
        for seg_id, seg_data in tree_data["segments"].items():
            self.segments[seg_id] = PathSegment.from_dict(seg_data)
        
        # Restore global visited positions
        self.global_visited_positions = {Position.from_dict(pos_data) 
                                       for pos_data in tree_data["global_visited_positions"]}
        
        print(f"📂 PathSegment tree loaded from '{filename}'")
        print(f"   📊 {len(self.segments)} segments, {len(self.global_visited_positions)} visited positions")


class PathSegmentVisualizer:
    """Visualizer for PathSegment-based exploration with robot markers"""
    
    # Clean color palette
    COLORS = {
        'background': '#FFFFFF',
        'maze_bg': '#FAFAFA', 
        'wall': '#E0E0E0',
        'start': '#4CAF50',
        'goal': '#F44336',
        'text': '#424242',
        'robot': '#FF9800',  # Orange for robot markers
        
        # Fixed segment colors (excluding gold/gray reserved for win/death)
        'segment_colors': [
            '#2196F3',  # Blue
            '#9C27B0',  # Purple  
            '#FF5722',  # Deep Orange
            '#8BC34A',  # Light Green
            '#00BCD4',  # Cyan
            '#E91E63',  # Pink
        ],
        
        # Lighter versions for inactive states
        'segment_colors_light': [
            '#64B5F6',  # Light Blue
            '#BA68C8',  # Light Purple
            '#FF8A65',  # Light Deep Orange
            '#AED581',  # Light Green
            '#4DD0E1',  # Light Cyan
            '#F06292',  # Light Pink
        ],
        
        'winner_color': '#FF6D00',  # Gold - ONLY for victory
        'dead_color': '#9E9E9E',    # Gray - ONLY for death
    }
    
    def __init__(self, engine: PathSegmentEngine, output_dir: str = "frames"):
        self.engine = engine
        self.output_dir = output_dir
        self.frame_count = 0
        
        # Create output directory
        if os.path.exists(output_dir):
            shutil.rmtree(output_dir)
        os.makedirs(output_dir)
        
        # Setup matplotlib for high-quality rendering
        plt.style.use('default')
        self.fig, self.ax = plt.subplots(1, 1, figsize=(12, 12))
        self.setup_plot()
    
    def setup_plot(self):
        """Setup clean plot styling"""
        self.ax.set_aspect('equal')
        self.ax.axis('off')
        self.fig.patch.set_facecolor(self.COLORS['background'])
        
        plt.tight_layout()
        plt.subplots_adjust(top=0.92, bottom=0.05, left=0.05, right=0.95)
    
    def save_frame(self):
        """Save current state as a frame"""
        self.ax.clear()
        self.draw_complete_state()
        
        # Save frame
        filename = os.path.join(self.output_dir, f"frame_{self.frame_count:04d}.png")
        self.fig.savefig(filename, dpi=100, bbox_inches='tight', 
                        facecolor=self.COLORS['background'], 
                        edgecolor='none')
        
        self.frame_count += 1
        
        # Print progress
        if self.frame_count % 10 == 0:
            stats = self.engine.get_statistics()
            print(f"📸 Frame {self.frame_count:4d} | Active: {stats['active_segments']:3d} | Total: {stats['total_segments']:4d}")
    
    def draw_complete_state(self):
        """Draw the complete current state"""
        self.draw_maze()
        self.draw_all_segments()
        self.draw_robot_markers()  # New: draw robot markers for active segments
        self.draw_title()
    
    def draw_maze(self):
        """Draw the maze structure"""
        # Background
        bg = patches.Rectangle((0, 0), self.engine.width, self.engine.height,
                             facecolor=self.COLORS['maze_bg'], 
                             edgecolor='none')
        self.ax.add_patch(bg)
        
        maze = self.engine.maze
        
        # Draw walls
        for y in range(self.engine.height):
            for x in range(self.engine.width):
                if maze[y][x] == CellType.WALL.value:
                    wall = patches.Rectangle((x, y), 1, 1,
                                           facecolor=self.COLORS['wall'],
                                           edgecolor='none')
                    self.ax.add_patch(wall)
        
        # Draw start position
        start_circle = patches.Circle((self.engine.start.x + 0.5, self.engine.start.y + 0.5), 0.35,
                                    facecolor=self.COLORS['start'], 
                                    edgecolor='white', linewidth=2)
        self.ax.add_patch(start_circle)
        
        # Draw goal position
        goal_circle = patches.Circle((self.engine.goal.x + 0.5, self.engine.goal.y + 0.5), 0.35,
                                   facecolor=self.COLORS['goal'],
                                   edgecolor='white', linewidth=2)
        self.ax.add_patch(goal_circle)
        
        # Set limits
        self.ax.set_xlim(0, self.engine.width)
        self.ax.set_ylim(0, self.engine.height)
        self.ax.invert_yaxis()
    
    def draw_all_segments(self):
        """Draw all path segments with parent-child color logic"""
        if self.engine.show_only_winner and self.engine.winning_segment:
            # Winner-only mode: show only segments leading to the winner
            segments_to_draw = self._get_winning_path_segments()
        else:
            # Normal mode: show all segments
            segments_to_draw = list(self.engine.segments.values())
        
        # Draw segment paths
        for segment in segments_to_draw:
            self._draw_segment_path(segment)
    
    def _get_winning_path_segments(self) -> List[PathSegment]:
        """Get all segments that are part of the winning path"""
        if not self.engine.winning_segment:
            return []
        
        winning_segments = []
        current_segment_id = self.engine.winning_segment
        
        # Trace back from winning segment to root
        while current_segment_id:
            if current_segment_id in self.engine.segments:
                segment = self.engine.segments[current_segment_id]
                winning_segments.append(segment)
                current_segment_id = segment.parent_id
            else:
                break
        
        return winning_segments
    
    def _get_segment_display_color_and_style(self, segment: PathSegment) -> Tuple[str, float, float, int]:
        """Get segment display color based on parent-child color logic"""
        # PRIORITY 1: Victory (GOLD)
        if segment.found_goal:
            return self.COLORS['winner_color'], 3.0, 1.0, 10
        
        # PRIORITY 2: Death (GRAY) - only if truly dead (no children and hit dead end)
        if segment.is_dead:
            child_segments = self.engine.get_child_segments(segment.id)
            if len(child_segments) == 0:  # Truly dead - no children
                return self.COLORS['dead_color'], 1.5, 0.5, 2
        
        # PRIORITY 3: Parent-child color logic
        child_segments = self.engine.get_child_segments(segment.id)
        
        if len(child_segments) == 0:
            # No children - use own fixed color
            base_color_index = segment.fixed_color_index % len(self.COLORS['segment_colors'])
            return self.COLORS['segment_colors'][base_color_index], 2.0, 0.9, 5
        else:
            # Has children - parent color determined by children
            child_colors = set()
            for child_segment in child_segments:
                if child_segment.found_goal:
                    child_colors.add('winner')
                elif child_segment.is_dead and len(self.engine.get_child_segments(child_segment.id)) == 0:
                    child_colors.add('dead')
                else:
                    child_color_index = child_segment.fixed_color_index % len(self.COLORS['segment_colors'])
                    child_colors.add(child_color_index)
            
            if len(child_colors) == 1:
                # All children same color - parent becomes that color
                single_color = list(child_colors)[0]
                if single_color == 'winner':
                    return self.COLORS['winner_color'], 3.0, 1.0, 10
                elif single_color == 'dead':
                    return self.COLORS['dead_color'], 1.5, 0.5, 2
                else:
                    # All children same segment color
                    return self.COLORS['segment_colors'][single_color], 2.0, 0.9, 5
            else:
                # Children have different colors - keep parent's original color
                base_color_index = segment.fixed_color_index % len(self.COLORS['segment_colors'])
                return self.COLORS['segment_colors'][base_color_index], 2.0, 0.9, 5
    
    def _draw_segment_path(self, segment: PathSegment):
        """Draw single path segment with parent-child color logic"""
        if len(segment.path_positions) < 2:
            return
        
        # Segment path coordinates - every position in this segment
        path_x = [pos.x + 0.5 for pos in segment.path_positions]
        path_y = [pos.y + 0.5 for pos in segment.path_positions]
        
        # Get color based on parent-child logic for segments
        color, width, alpha, zorder = self._get_segment_display_color_and_style(segment)
        
        # Draw segment path
        self.ax.plot(path_x, path_y, 
                    color=color, linewidth=width, alpha=alpha,
                    solid_capstyle='round', solid_joinstyle='round',
                    zorder=zorder)
    
    def draw_robot_markers(self):
        """Draw explorer markers for active segment heads with matching colors"""
        active_segments = [s for s in self.engine.segments.values() if s.is_active]
        
        for segment in active_segments:
            pos = segment.current_position
            
            # Skip if at start/goal (already drawn with special markers)
            if ((pos.x == self.engine.start.x and pos.y == self.engine.start.y) or
                (pos.x == self.engine.goal.x and pos.y == self.engine.goal.y)):
                continue
            
            # Get explorer color from segment's fixed color
            base_color_index = segment.fixed_color_index % len(self.COLORS['segment_colors'])
            explorer_color = self.COLORS['segment_colors'][base_color_index]
            
            # Draw large, prominent explorer marker (bigger diamond with pulsing effect)
            explorer_diamond = patches.RegularPolygon(
                (pos.x + 0.5, pos.y + 0.5), 4, radius=0.3,  # Much larger radius
                orientation=0, facecolor=explorer_color, 
                edgecolor='white', linewidth=3, zorder=20,  # Thicker border
                alpha=0.9
            )
            self.ax.add_patch(explorer_diamond)
            
            # Add inner highlight to make it more prominent
            inner_diamond = patches.RegularPolygon(
                (pos.x + 0.5, pos.y + 0.5), 4, radius=0.15,
                orientation=0, facecolor='white', 
                alpha=0.6, zorder=21
            )
            self.ax.add_patch(inner_diamond)
    
    def draw_title(self):
        """Draw informative title"""
        stats = self.engine.get_statistics()
        
        if stats['show_only_winner']:
            # Winner-only display mode
            title = "SHORTEST PATH FOUND!"
            subtitle = f"Multi-Branch BFS Pathfinding | Optimal Solution: {stats['total_segments']} segments explored"
        elif stats['goal_found']:
            title = "PANTHEON MAZE SOLVED!"
            subtitle = f"Multi-Branch BFS Pathfinding | Winner: {stats['winning_segment']} | Segments: {stats['total_segments']}"
        else:
            title = "Multi-Branch BFS Pathfinding"
            subtitle = f"Concurrent exploration spawning branches at junctions | Step: {stats['total_steps']} | Active: {stats['active_segments']} | Total: {stats['total_segments']}"
        
        self.fig.suptitle(f"{title}\n{subtitle}",
                         fontsize=14, fontweight='400',
                         color=self.COLORS['text'], y=0.96)


def create_gif_from_frames(frames_dir: str, output_filename: str = "pantheon_exploration.gif", fps: int = 4):
    """Create animated GIF from saved frames"""
    if not PIL_AVAILABLE:
        print("❌ PIL not available - cannot create GIF")
        return False
    
    # Get all frame files
    frame_files = [f for f in os.listdir(frames_dir) if f.startswith('frame_') and f.endswith('.png')]
    frame_files.sort()
    
    if not frame_files:
        print("❌ No frame files found")
        return False
    
    print(f"🎬 Creating GIF from {len(frame_files)} frames...")
    
    # Load and optimize frames
    frames = []
    for i, filename in enumerate(frame_files):
        filepath = os.path.join(frames_dir, filename)
        img = Image.open(filepath)
        
        # Resize for reasonable file size
        if img.width > 800:
            ratio = 800 / img.width
            new_size = (800, int(img.height * ratio))
            img = img.resize(new_size, Image.Resampling.LANCZOS)
        
        # Convert to palette mode for smaller GIF
        img = img.convert('P', palette=Image.ADAPTIVE, colors=256)
        frames.append(img)
        
        if (i + 1) % 50 == 0:
            print(f"  Processed {i + 1}/{len(frame_files)} frames...")
    
    # Save GIF
    duration = int(1000 / fps)  # Duration per frame in milliseconds
    frames[0].save(
        output_filename,
        save_all=True,
        append_images=frames[1:],
        duration=duration,
        loop=0,
        optimize=True
    )
    
    print(f"✅ GIF saved as '{output_filename}' ({len(frames)} frames, {fps} fps)")
    return True


def run_pantheon_exploration_with_recording():
    """Run complete Pantheon exploration with frame recording"""
    print("🎮 Pantheon Maze Explorer with PathSegment Tree")
    print("=" * 60)
    
    # Create engine and visualizer
    engine = PathSegmentEngine(width=31, height=31, seed=42)
    visualizer = PathSegmentVisualizer(engine)
    
    print(f"📐 Maze size: {engine.width}x{engine.height}")
    print(f"📍 Start: ({engine.start.x}, {engine.start.y})")
    print(f"🎯 Goal: ({engine.goal.x}, {engine.goal.y})")
    print(f"💾 Frames will be saved to '{visualizer.output_dir}' directory")
    print()
    print("🚀 Starting Pantheon exploration with frame recording...")
    print("-" * 40)
    
    # Save initial frame
    visualizer.save_frame()
    
    # Run exploration with frame recording
    step = 0
    max_steps = 500  # Safety limit
    
    while engine.step() and step < max_steps:
        step += 1
        
        # Save frame after each step
        visualizer.save_frame()
        
        # Check if goal found
        if engine.goal_found:
            print(f"\n🎉 Goal found in {step} steps!")
            
            # Switch to winner-only display mode
            engine.enable_winner_only_mode()
            
            # Save additional frames showing only the shortest path
            print("📸 Saving additional frames with clean shortest path display...")
            for extra_frame in range(12):  # Save 12 extra frames (3 seconds at 4fps)
                visualizer.save_frame()
                print(f"  Extra frame {extra_frame + 1}/12 saved")
            
            break
    
    # Final statistics
    print("\n" + "=" * 60)
    print("EXPLORATION COMPLETE")
    print("=" * 60)
    
    stats = engine.get_statistics()
    print(f"📊 Final Statistics:")
    print(f"   🛤️  Total path segments: {stats['total_segments']}")
    print(f"   ✅ Complete segments: {stats['complete_segments']}")
    print(f"   💀 Dead segments: {stats['dead_segments']}")
    print(f"   🎯 Goal reached: {stats['goal_found']}")
    print(f"   📍 Visited positions: {stats['visited_positions']}")
    print(f"   🔄 Total steps: {stats['total_steps']}")
    print(f"   📈 Max concurrent segments: {stats['max_concurrent']}")
    print(f"   📸 Total frames captured: {visualizer.frame_count}")
    
    if stats['goal_found']:
        winner = engine.segments[stats['winning_segment']]
        print(f"   🏆 Winner: {stats['winning_segment']}")
        print(f"   📏 Shortest path length: {len(winner.path_positions)}")
        print(f"   🧬 Winner generation: {winner.generation}")
    
    # Save PathSegment tree to JSON
    print(f"\n💾 Saving PathSegment tree structure...")
    engine.save_tree_to_json("pathsegment_tree.json")
    
    return visualizer.frame_count > 0


if __name__ == "__main__":
    try:
        # Run exploration with frame recording
        success = run_pantheon_exploration_with_recording()
        
        if success:
            print("\n🎬 Creating animated GIF with slower playback...")
            if create_gif_from_frames("frames", "pantheon_exploration.gif", fps=4):
                print("🎉 Complete! Check 'pantheon_exploration.gif' (4fps for clear viewing)")
            else:
                print("⚠️  GIF creation failed - frames are available in 'frames/' directory")
        else:
            print("❌ Exploration failed")
            
    except KeyboardInterrupt:
        print("\n⏹️ Exploration interrupted")
    except Exception as e:
        print(f"❌ Error: {e}")
        import traceback
        traceback.print_exc()