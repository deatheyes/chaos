package config

// Agent is a standalone controller at a node
type Agent struct {
	Role     string            // role of node
	Address  string            // agent's IP
	Version  string            // agent's version for compatibility
	Selector map[string]string // selector for the agent
}

// Selector is a filter for agent
type Selector map[string]string

// Topology is a view of the whole cluster
type Topology map[string]*Agent

// Select get a or some agent according to selector
func (t Topology) Select(selector Selector) []*Agent {
	ret := []*Agent{}
	for _, a := range t {
		flag := true
		for k, v := range selector {
			if value, ok := a.Selector[k]; !ok {
				flag = false
				break
			} else if value != v {
				flag = false
				break
			}
		}

		if flag {
			ret = append(ret, a)
		}
	}
	return ret
}
