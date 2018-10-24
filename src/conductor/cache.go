package conductor

type dcstore struct {
	_id  map[string]*Datacenter
	name map[string]*Datacenter
}

type groupstore struct {
	_id  map[string]*Group
	name map[string]*Group
}

type hoststore struct {
	_id  map[string]*Host
	fqdn map[string]*Host
}

type wgstore struct {
	_id  map[string]*WorkGroup
	name map[string]*WorkGroup
}

type store struct {
	datacenters *dcstore
	groups      *groupstore
	hosts       *hoststore
	workgroups  *wgstore
}

func newStore() *store {
	s := new(store)
	s.datacenters = new(dcstore)
	s.datacenters._id = make(map[string]*Datacenter)
	s.datacenters.name = make(map[string]*Datacenter)
	s.groups = new(groupstore)
	s.groups._id = make(map[string]*Group)
	s.groups.name = make(map[string]*Group)
	s.hosts = new(hoststore)
	s.hosts._id = make(map[string]*Host)
	s.hosts.fqdn = make(map[string]*Host)
	s.workgroups = new(wgstore)
	s.workgroups._id = make(map[string]*WorkGroup)
	s.workgroups.name = make(map[string]*WorkGroup)
	return s
}
