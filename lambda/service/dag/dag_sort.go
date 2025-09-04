package dag

import (
	"errors"
	"sort"
)

// Kahn’s algorithm does a topological sort of a DAG and returns the execution order grouped by levels.
// Each inner slice is a batch of nodes that can run in parallel.
func TopologicalSortLevels(graph map[string][]string) ([][]string, error) {
	// 1) Collect all nodes
	nodes := make(map[string]struct{})
	for n, deps := range graph {
		nodes[n] = struct{}{}
		for _, d := range deps {
			nodes[d] = struct{}{}
		}
	}

	// 2) Build adjacency list: dependency -> dependents
	adj := make(map[string][]string, len(nodes))
	for n := range nodes {
		adj[n] = []string{}
	}
	for node, deps := range graph {
		for _, dep := range deps {
			adj[dep] = append(adj[dep], node)
		}
	}

	// 3) Compute indegree
	indegree := make(map[string]int, len(nodes))
	for n := range nodes {
		indegree[n] = 0
	}
	for node, deps := range graph {
		indegree[node] = len(deps)
	}

	// 4) Initialize queue with nodes having indegree 0
	queue := []string{}
	for n, deg := range indegree {
		if deg == 0 {
			queue = append(queue, n)
		}
	}
	sort.Strings(queue)

	var levels [][]string

	// 5) Kahn’s loop by levels
	for len(queue) > 0 {
		level := append([]string(nil), queue...) // current batch
		levels = append(levels, level)
		nextQueue := []string{}

		for _, curr := range queue {
			for _, dep := range adj[curr] {
				indegree[dep]--
				if indegree[dep] == 0 {
					nextQueue = append(nextQueue, dep)
				}
			}
		}

		sort.Strings(nextQueue)
		queue = nextQueue
	}

	// 6) Detect cycles
	totalNodes := 0
	for _, level := range levels {
		totalNodes += len(level)
	}
	if totalNodes != len(nodes) {
		return nil, errors.New("graph contains a cycle — topological sort not possible")
	}

	return levels, nil
}
