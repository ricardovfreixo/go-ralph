package manifest

import (
	"fmt"
	"sort"
)

type DependencyGraph struct {
	nodes map[string]bool
	edges map[string][]string // feature -> dependencies
}

func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes: make(map[string]bool),
		edges: make(map[string][]string),
	}
}

func (g *DependencyGraph) AddNode(id string) {
	g.nodes[id] = true
}

func (g *DependencyGraph) AddEdge(from, to string) {
	g.edges[from] = append(g.edges[from], to)
}

func (g *DependencyGraph) GetDependencies(id string) []string {
	return g.edges[id]
}

func (m *Manifest) BuildDependencyGraph() *DependencyGraph {
	m.mu.RLock()
	defer m.mu.RUnlock()

	graph := NewDependencyGraph()

	for _, feature := range m.Features {
		graph.AddNode(feature.ID)
	}

	for _, feature := range m.Features {
		for _, depID := range feature.DependsOn {
			graph.AddEdge(feature.ID, depID)
		}
	}

	return graph
}

type CircularDependencyError struct {
	Cycle []string
}

func (e *CircularDependencyError) Error() string {
	return fmt.Sprintf("circular dependency detected: %v", e.Cycle)
}

func (g *DependencyGraph) DetectCycles() *CircularDependencyError {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	path := make([]string, 0)

	var dfs func(node string) *CircularDependencyError
	dfs = func(node string) *CircularDependencyError {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		for _, dep := range g.edges[node] {
			if !g.nodes[dep] {
				continue
			}
			if !visited[dep] {
				if err := dfs(dep); err != nil {
					return err
				}
			} else if recStack[dep] {
				cycleStart := -1
				for i, n := range path {
					if n == dep {
						cycleStart = i
						break
					}
				}
				cycle := append(path[cycleStart:], dep)
				return &CircularDependencyError{Cycle: cycle}
			}
		}

		path = path[:len(path)-1]
		recStack[node] = false
		return nil
	}

	for node := range g.nodes {
		if !visited[node] {
			if err := dfs(node); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *Manifest) ValidateDependencies() (warnings []string, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	featureIDs := make(map[string]bool)
	for _, f := range m.Features {
		featureIDs[f.ID] = true
	}

	for _, feature := range m.Features {
		for _, depID := range feature.DependsOn {
			if !featureIDs[depID] {
				warnings = append(warnings,
					fmt.Sprintf("feature %s (%s) depends on unknown feature %q, treating as no dependency",
						feature.ID, feature.Title, depID))
			}
		}
	}

	graph := &DependencyGraph{
		nodes: make(map[string]bool),
		edges: make(map[string][]string),
	}
	for _, f := range m.Features {
		graph.nodes[f.ID] = true
	}
	for _, f := range m.Features {
		for _, depID := range f.DependsOn {
			if featureIDs[depID] {
				graph.edges[f.ID] = append(graph.edges[f.ID], depID)
			}
		}
	}

	if cycleErr := graph.DetectCycles(); cycleErr != nil {
		return warnings, cycleErr
	}

	return warnings, nil
}

func (m *Manifest) GetNextRunnableFeature() *ManifestFeature {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := range m.Features {
		feature := &m.Features[i]
		if feature.Status != "pending" {
			continue
		}
		if m.isDependencySatisfiedUnlocked(feature.ID) {
			return feature
		}
	}
	return nil
}

func (m *Manifest) GetAllRunnableFeatures() []ManifestFeature {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var runnable []ManifestFeature
	for _, feature := range m.Features {
		if feature.Status != "pending" {
			continue
		}
		if m.isDependencySatisfiedUnlocked(feature.ID) {
			runnable = append(runnable, feature)
		}
	}
	return runnable
}

func (m *Manifest) GetBlockedFeatures() []ManifestFeature {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var blocked []ManifestFeature
	for _, feature := range m.Features {
		if feature.Status != "pending" {
			continue
		}
		if !m.isDependencySatisfiedUnlocked(feature.ID) {
			blocked = append(blocked, feature)
		}
	}
	return blocked
}

func (m *Manifest) GetTopologicalOrder() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	graph := &DependencyGraph{
		nodes: make(map[string]bool),
		edges: make(map[string][]string),
	}

	featureIDs := make(map[string]bool)
	for _, f := range m.Features {
		featureIDs[f.ID] = true
		graph.nodes[f.ID] = true
	}

	for _, f := range m.Features {
		for _, depID := range f.DependsOn {
			if featureIDs[depID] {
				graph.edges[f.ID] = append(graph.edges[f.ID], depID)
			}
		}
	}

	if cycleErr := graph.DetectCycles(); cycleErr != nil {
		return nil, cycleErr
	}

	inDegree := make(map[string]int)
	for id := range graph.nodes {
		inDegree[id] = 0
	}

	reverseEdges := make(map[string][]string)
	for from, deps := range graph.edges {
		for _, to := range deps {
			reverseEdges[to] = append(reverseEdges[to], from)
			inDegree[from]++
		}
	}

	var queue []string
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	var result []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		var nextNodes []string
		for _, dependent := range reverseEdges[node] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				nextNodes = append(nextNodes, dependent)
			}
		}
		sort.Strings(nextNodes)
		queue = append(queue, nextNodes...)
	}

	return result, nil
}

func (m *Manifest) RemoveMissingDependencies() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	featureIDs := make(map[string]bool)
	for _, f := range m.Features {
		featureIDs[f.ID] = true
	}

	var removed []string
	for i := range m.Features {
		var validDeps []string
		for _, depID := range m.Features[i].DependsOn {
			if featureIDs[depID] {
				validDeps = append(validDeps, depID)
			} else {
				removed = append(removed,
					fmt.Sprintf("feature %s: removed invalid dependency %q",
						m.Features[i].ID, depID))
			}
		}
		m.Features[i].DependsOn = validDeps
	}

	return removed
}
