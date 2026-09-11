package tofu

import (
	"sort"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"
)

// Action of a node or resource in a plan, in increasing severity.
const (
	ActionNoop    = "no-op"
	ActionRead    = "read"
	ActionCreate  = "create"
	ActionUpdate  = "update"
	ActionReplace = "replace"
	ActionDelete  = "delete"
)

// Summary is a plan reduced to what the canvas needs: one action per node,
// and the resource-level detail behind it.
type Summary struct {
	Add     int                 `json:"add"`
	Change  int                 `json:"change"`
	Destroy int                 `json:"destroy"`
	Nodes   map[string]NodePlan `json:"nodes"`
	// Orphans are planned resources that map to no node (e.g. removed from
	// the diagram, still in state): they will be destroyed.
	Orphans []ResourceChange `json:"orphans,omitempty"`
}

// NodePlan is the aggregated plan for one diagram node.
type NodePlan struct {
	Action    string           `json:"action"`
	Resources []ResourceChange `json:"resources"`
}

// ResourceChange is one resource in the plan.
type ResourceChange struct {
	Address string   `json:"address"`
	Type    string   `json:"type"`
	Actions []string `json:"actions"`
	Action  string   `json:"action"`
}

// Summarize maps plan resource changes back to nodes. moduleToNode maps the
// sanitized module name used in generated Terraform to the node id.
func Summarize(p *tfjson.Plan, moduleToNode map[string]string) Summary {
	s := Summary{Nodes: map[string]NodePlan{}}
	for _, rc := range p.ResourceChanges {
		if rc.Change == nil {
			continue
		}
		change := ResourceChange{Address: rc.Address, Type: rc.Type}
		for _, a := range rc.Change.Actions {
			change.Actions = append(change.Actions, string(a))
		}
		change.Action = actionOf(rc.Change.Actions)
		switch change.Action {
		case ActionCreate:
			s.Add++
		case ActionUpdate:
			s.Change++
		case ActionDelete:
			s.Destroy++
		case ActionReplace:
			s.Add++
			s.Destroy++
		}
		node := ""
		if strings.HasPrefix(rc.Address, "module.") {
			rest := strings.TrimPrefix(rc.Address, "module.")
			mod, _, _ := strings.Cut(rest, ".")
			mod, _, _ = strings.Cut(mod, "[")
			node = moduleToNode[mod]
		}
		if node == "" {
			if change.Action != ActionNoop {
				s.Orphans = append(s.Orphans, change)
			}
			continue
		}
		np := s.Nodes[node]
		np.Resources = append(np.Resources, change)
		if severity(change.Action) > severity(np.Action) {
			np.Action = change.Action
		}
		s.Nodes[node] = np
	}
	for id, np := range s.Nodes {
		sort.Slice(np.Resources, func(i, j int) bool { return np.Resources[i].Address < np.Resources[j].Address })
		if np.Action == "" {
			np.Action = ActionNoop
		}
		s.Nodes[id] = np
	}
	return s
}

func actionOf(actions tfjson.Actions) string {
	switch {
	case actions.Replace():
		return ActionReplace
	case actions.Create():
		return ActionCreate
	case actions.Delete():
		return ActionDelete
	case actions.Update():
		return ActionUpdate
	case actions.Read():
		return ActionRead
	default:
		return ActionNoop
	}
}

func severity(a string) int {
	switch a {
	case ActionDelete:
		return 5
	case ActionReplace:
		return 4
	case ActionUpdate:
		return 3
	case ActionCreate:
		return 2
	case ActionRead:
		return 1
	}
	return 0
}
