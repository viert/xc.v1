package conductor

import (
	"time"
)

type Datacenter struct {
	ID          string   `json:"_id"`
	ChildIds    []string `json:"child_ids"`
	Description string   `json:"description"`
	Name        string   `json:"name"`
	ParentID    string   `json:"parent_id"`
	RootID      string   `json:"root_id"`
	Parent      *Datacenter
}

type Group struct {
	ID          string   `json:"_id"`
	ChildIds    []string `json:"child_ids"`
	ParentIds   []string `json:"parent_ids"`
	AllTags     []string `json:"tags"`
	Description string   `json:"description"`
	Name        string   `json:"name"`
	WorkGroupID string   `json:"work_group_id"`
	Hosts       []*Host
}

type Host struct {
	ID           string   `json:"_id"`
	Aliases      []string `json:"aliases"`
	AllTags      []string `json:"tags"`
	FQDN         string   `json:"fqdn"`
	GroupID      string   `json:"group_id"`
	DatacenterID string   `json:"datacenter_id"`
	Datacenter   *Datacenter
}

type WorkGroup struct {
	ID          string `json:"_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Groups      []*Group
}

type ExecuterData struct {
	Datacenters []*Datacenter `json:"datacenters"`
	Groups      []*Group      `json:"groups"`
	Hosts       []*Host       `json:"hosts"`
	WorkGroups  []*WorkGroup  `json:"work_groups"`
}

type ExecuterRootData struct {
	Data      ExecuterData `json:"data"`
	CreatedAt time.Time    `json:"created_at"`
}

func (g *Group) Children() []*Group {
	children := make([]*Group, 0)
	if len(g.ChildIds) > 0 {
		for i := 0; i < len(g.ChildIds); i++ {
			if child, found := cGlobal.cache.groups._id[g.ChildIds[i]]; found {
				children = append(children, child)
			}
		}
		lower := make([]*Group, 0)
		for _, child := range children {
			lower = append(lower, child.Children()...)
		}
		children = append(children, lower...)
	}
	return children
}

func (g *Group) AllHosts() []*Host {
	all_groups := g.Children()
	all_groups = append(all_groups, g)
	hosts := make([]*Host, 0)
	for _, group := range all_groups {
		hosts = append(hosts, group.Hosts...)
	}
	return hosts
}
