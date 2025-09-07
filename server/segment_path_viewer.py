#!/usr/bin/env python3
"""
Segment Path Viewer - Visualize path from root to any segment
Based on PathSegment tree structure from pantheon_explorer.py
"""

import random
import time
import os
import shutil
import json
import sys
from collections import deque
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Set, Tuple
from enum import Enum
import matplotlib.pyplot as plt
import matplotlib.patches as patches
import numpy as np


class NumpyEncoder(json.JSONEncoder):
    """Custom JSON encoder to handle numpy data types"""
    def default(self, obj):
        if isinstance(obj, np.integer):
            return int(obj)
        if isinstance(obj, np.floating):
            return float(obj)
        if isinstance(obj, np.ndarray):
            return obj.tolist()
        return super().default(obj)

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


class SegmentPathViewer:
    """Viewer for specific segment paths loaded from JSON"""
    
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
        'target_color': '#FF1744',  # Bright red for target segment
    }
    
    def __init__(self, json_file: str = "pathsegment_tree.json"):
        self.json_file = json_file
        self.maze = None
        self.width = 0
        self.height = 0
        self.start = None
        self.goal = None
        self.segments = {}
        
        # Setup matplotlib
        plt.style.use('default')
        self.fig, self.ax = plt.subplots(1, 1, figsize=(14, 14))
        self.setup_plot()
    
    def setup_plot(self):
        """Setup clean plot styling"""
        self.ax.set_aspect('equal')
        self.ax.axis('off')
        self.fig.patch.set_facecolor(self.COLORS['background'])
        
        plt.tight_layout()
        plt.subplots_adjust(top=0.90, bottom=0.05, left=0.05, right=0.95)
    
    def load_tree_from_json(self) -> bool:
        """Load PathSegment tree from JSON file"""
        try:
            with open(self.json_file, 'r', encoding='utf-8') as f:
                tree_data = json.load(f)
            
            # Restore metadata
            metadata = tree_data["metadata"]
            self.width = metadata["width"]
            self.height = metadata["height"]
            self.start = Position.from_dict(metadata["start"])
            self.goal = Position.from_dict(metadata["goal"])
            
            # Restore maze
            self.maze = np.array(tree_data["maze"])
            
            # Restore segments
            self.segments = {}
            for seg_id, seg_data in tree_data["segments"].items():
                self.segments[seg_id] = PathSegment.from_dict(seg_data)
            
            print(f"📂 PathSegment tree loaded from '{self.json_file}'")
            print(f"   📊 {len(self.segments)} segments loaded")
            return True
            
        except FileNotFoundError:
            print(f"❌ Error: File '{self.json_file}' not found")
            return False
        except Exception as e:
            print(f"❌ Error loading file: {e}")
            return False
    
    def find_segment_path(self, target_segment_id: str) -> List[PathSegment]:
        """Find path from root to target segment"""
        if target_segment_id not in self.segments:
            print(f"❌ Error: Segment '{target_segment_id}' not found")
            return []
        
        path_segments = []
        current_segment_id = target_segment_id
        
        # Trace back from target segment to root
        while current_segment_id:
            if current_segment_id in self.segments:
                segment = self.segments[current_segment_id]
                path_segments.append(segment)
                current_segment_id = segment.parent_id
            else:
                break
        
        # Reverse to get path from root to target
        path_segments.reverse()
        
        # print(f"🛤️  Path from root to '{target_segment_id}': {len(path_segments)} segments")
        # for i, seg in enumerate(path_segments):
        #     print(f"   {i+1}. {seg.id} (gen: {seg.generation})")
        
        return path_segments
    
    def list_all_segments(self):
        """List all available segments with basic info"""
        print(f"\n📋 Available segments in '{self.json_file}':")
        print("=" * 60)
        
        # Group segments by generation for better display
        generations = {}
        for seg_id, segment in self.segments.items():
            gen = segment.generation
            if gen not in generations:
                generations[gen] = []
            generations[gen].append(segment)
        
        for gen in sorted(generations.keys()):
            segments = generations[gen]
            print(f"Generation {gen}: ({len(segments)} segments)")
            
            for segment in sorted(segments, key=lambda s: s.id):
                status_icons = []
                if segment.found_goal:
                    status_icons.append("🎯")
                if segment.is_dead:
                    status_icons.append("💀")
                if segment.is_active:
                    status_icons.append("🔄")
                if segment.is_complete:
                    status_icons.append("✅")
                
                status_str = " ".join(status_icons) if status_icons else "📍"
                parent_str = f" (parent: {segment.parent_id})" if segment.parent_id else ""
                child_str = f" (children: {len(segment.child_ids)})" if segment.child_ids else ""
                
                print(f"  {status_str} {segment.id}{parent_str}{child_str}")
            print()
    
    def draw_maze(self):
        """Draw the maze structure"""
        # Background
        bg = patches.Rectangle((0, 0), self.width, self.height,
                             facecolor=self.COLORS['maze_bg'], 
                             edgecolor='none')
        self.ax.add_patch(bg)
        
        # Draw walls
        for y in range(self.height):
            for x in range(self.width):
                if self.maze[y][x] == CellType.WALL.value:
                    wall = patches.Rectangle((x, y), 1, 1,
                                           facecolor=self.COLORS['wall'],
                                           edgecolor='none')
                    self.ax.add_patch(wall)
        
        # Draw start position
        start_circle = patches.Circle((self.start.x + 0.5, self.start.y + 0.5), 0.35,
                                    facecolor=self.COLORS['start'], 
                                    edgecolor='white', linewidth=2)
        self.ax.add_patch(start_circle)
        
        # Draw goal position
        goal_circle = patches.Circle((self.goal.x + 0.5, self.goal.y + 0.5), 0.35,
                                   facecolor=self.COLORS['goal'],
                                   edgecolor='white', linewidth=2)
        self.ax.add_patch(goal_circle)
        
        # Set limits
        self.ax.set_xlim(0, self.width)
        self.ax.set_ylim(0, self.height)
        self.ax.invert_yaxis()
    
    def draw_segment_path(self, path_segments: List[PathSegment], target_segment_id: str):
        """Draw the path segments with the target highlighted"""
        for i, segment in enumerate(path_segments):
            if len(segment.path_positions) < 2:
                continue
            
            # Segment path coordinates
            path_x = [pos.x + 0.5 for pos in segment.path_positions]
            path_y = [pos.y + 0.5 for pos in segment.path_positions]
            
            # Determine color and style
            if segment.id == target_segment_id:
                # Target segment - bright red
                color = self.COLORS['target_color']
                width = 6.0
                alpha = 1.0
                zorder = 15
            elif segment.found_goal:
                # Goal segment - gold
                color = self.COLORS['winner_color']
                width = 5.0
                alpha = 1.0
                zorder = 12
            else:
                # Regular segment - use fixed color with higher prominence
                base_color_index = segment.fixed_color_index % len(self.COLORS['segment_colors'])
                color = self.COLORS['segment_colors'][base_color_index]
                width = 4.0
                alpha = 0.9
                zorder = 10
            
            # Draw segment path
            self.ax.plot(path_x, path_y, 
                        color=color, linewidth=width, alpha=alpha,
                        solid_capstyle='round', solid_joinstyle='round',
                        zorder=zorder)
            
            # Add segment ID label at the end position
            # if segment.id != "root":  # Don't label root
            #     end_pos = segment.current_position
            #     self.ax.text(end_pos.x + 0.5, end_pos.y + 0.3, 
            #                segment.id, 
            #                fontsize=9, fontweight='bold',
            #                ha='center', va='bottom',
            #                color=color,
            #                bbox=dict(boxstyle='round,pad=0.2', 
            #                        facecolor='white', alpha=0.8, edgecolor=color),
            #                zorder=20)
    
    def draw_target_marker(self, target_segment: PathSegment):
        """Draw special marker for the target segment endpoint"""
        pos = target_segment.current_position
        
        # Large target marker - double ring
        outer_ring = patches.Circle((pos.x + 0.5, pos.y + 0.5), 0.4,
                                  facecolor='none', 
                                  edgecolor=self.COLORS['target_color'], 
                                  linewidth=4, zorder=25)
        self.ax.add_patch(outer_ring)
        
        inner_ring = patches.Circle((pos.x + 0.5, pos.y + 0.5), 0.25,
                                  facecolor=self.COLORS['target_color'], 
                                  edgecolor='white', 
                                  linewidth=2, alpha=0.8, zorder=26)
        self.ax.add_patch(inner_ring)
    
    def visualize_segment_path(self, target_segment_id: str, save_image: bool = True):
        """Visualize path from root to target segment"""
        # Clear previous plot
        self.ax.clear()
        
        # Find path segments
        path_segments = self.find_segment_path(target_segment_id)
        if not path_segments:
            return False
        
        target_segment = path_segments[-1]
        
        # Draw components
        self.draw_maze()
        self.draw_segment_path(path_segments, target_segment_id)
        self.draw_target_marker(target_segment)
        
        # Add title
        total_path_length = sum(len(seg.path_positions) - 1 for seg in path_segments) + 1
        
        if target_segment.found_goal:
            title = "PANTHEON MAZE SOLVED!"
            subtitle = f"Multi-Branch BFS Pathfinding | Winner: {target_segment_id} | Segments: {len(path_segments)} | Path length: {total_path_length} steps"
        elif target_segment.is_dead:
            title = "Dead End Reached"
            subtitle = f"Multi-Branch BFS Pathfinding | Segment: {target_segment_id} | Generation: {target_segment.generation} | Path length: {total_path_length} steps"
        else:
            title = "Multi-Branch BFS Pathfinding"
            subtitle = f"Segment Analysis | Target: {target_segment_id} | Generation: {target_segment.generation} | Path length: {total_path_length} steps"
        
        self.fig.suptitle(f"{title}\n{subtitle}",
                         fontsize=14, fontweight='400',
                         color=self.COLORS['text'], y=0.94)
        
        # Save image if requested
        if save_image:
            filename = f"segment_path_{target_segment_id}.png"
            self.fig.savefig(filename, dpi=150, bbox_inches='tight', 
                           facecolor=self.COLORS['background'], 
                           edgecolor='none')
            print(f"💾 Path visualization saved as '{filename}'")
        
        # Don't show plot, just save
        return True


def main():
    """Main function to handle command line interaction"""
    print("🔍 PathSegment Path Viewer")
    print("=" * 50)
    
    # Check for command line arguments
    json_file = "pathsegment_tree.json"
    segment_id = None
    
    if len(sys.argv) > 1:
        segment_id = sys.argv[1]
    if len(sys.argv) > 2:
        json_file = sys.argv[2]
    
    viewer = SegmentPathViewer(json_file)
    
    # Load tree data
    if not viewer.load_tree_from_json():
        return
    
    # If segment ID provided as argument, visualize directly
    if segment_id:
        print(f"\n🎨 Visualizing path to segment '{segment_id}'...")
        if viewer.visualize_segment_path(segment_id):
            print("✅ Visualization completed!")
        return
    
    # Otherwise ask for input
    print("\n📋 Available segments:")
    viewer.list_all_segments()
    
    print("\n" + "=" * 50)
    segment_id = input("Enter segment ID to visualize: ").strip()
    
    if segment_id:
        print(f"\n🎨 Visualizing path to segment '{segment_id}'...")
        if viewer.visualize_segment_path(segment_id):
            print("✅ Visualization completed!")
    else:
        print("❌ No segment ID provided")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\n⏹️ Interrupted by user")
    except Exception as e:
        print(f"❌ Error: {e}")
        import traceback
        traceback.print_exc()