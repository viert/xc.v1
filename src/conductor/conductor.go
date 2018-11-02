package conductor

import (
	"encoding/json"
	"fmt"
	"github.com/viert/sekwence"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"term"
	"time"
)

// ConductorConfig represents configuration for ConductorClass
type ConductorConfig struct {
	CacheTTL      time.Duration
	CacheDir      string
	WorkGroupList []string
	RemoteUrl     string
}

type Conductor struct {
	config *ConductorConfig
	data   *ExecuterRootData
	cache  *store
}

var (
	cGlobal        *Conductor
	exprWhiteSpace = regexp.MustCompile(`\s+`)
)

// NewConductor creates a new Conductor instance according to a
// given configuration
func NewConductor(config *ConductorConfig) *Conductor {
	cGlobal = &Conductor{config, &ExecuterRootData{}, newStore()}
	return cGlobal
}

func (c *Conductor) getCacheFilename() string {
	wglist := "all"
	if len(c.config.WorkGroupList) > 0 {
		wglist = strings.Join(c.config.WorkGroupList, "_")
	}
	cacheFilename := "cache_" + wglist + ".json"
	cacheFilename = path.Join(c.config.CacheDir, cacheFilename)
	return cacheFilename
}

func (c *Conductor) loadJSONCache() ([]byte, error) {
	_, err := os.Stat(c.config.CacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(c.config.CacheDir, 0755)
			if err != nil {
				term.Errorf("Can't create cache directory %s: %s\n", c.config.CacheDir, err)
				return nil, err
			}
		} else {
			term.Errorf("Error reading cache: %s\n", err)
			return nil, err
		}
	}

	cacheFilename := c.getCacheFilename()

	cf, err := os.Open(cacheFilename)
	if err != nil {
		if os.IsNotExist(err) {
			term.Warnf("No cache file %s found\n", cacheFilename)
			return nil, err
		} else {
			term.Errorf("Error opening cache file %s: %s\n", cacheFilename, err)
			return nil, err
		}
	}
	defer cf.Close()
	data, err := ioutil.ReadAll(cf)
	if err != nil {
		term.Errorf("Error reading cache file %s: %s\n", cacheFilename, err)
		return nil, err
	}
	return data, nil
}

func (c *Conductor) loadJSONHTTP() ([]byte, error) {
	term.Warnf("Reloading data from inventoree\n")
	wglist := strings.Join(c.config.WorkGroupList, ",")
	url := fmt.Sprintf("%s/api/v1/open/executer_data?work_groups=%s&recursive=true", c.config.RemoteUrl, wglist)
	resp, err := http.Get(url)
	if err != nil {
		term.Errorf("Error getting data by HTTP: %s\n", err)
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		term.Errorf("Error reading data: %s\n", err)
		return nil, err
	}
	return body, nil
}

func (c *Conductor) saveCache() error {
	if c.data == nil {
		return fmt.Errorf("Nothing to save to cache")
	}
	c.data.CreatedAt = time.Now()
	data, err := json.Marshal(c.data)
	if err != nil {
		term.Errorf("Error encoding cache data, this might be a bug: %s\n", err)
		return err
	}
	cacheFilename := c.getCacheFilename()
	err = ioutil.WriteFile(cacheFilename, data, 0644)
	if err != nil {
		term.Errorf("Error writing cachefile %s: %s\n", cacheFilename, err)
	}
	return err
}

func (c *Conductor) decodeJSON(jsonData []byte) error {
	return json.Unmarshal(jsonData, c.data)
}

// Load loads groups from cache or inventoree
func (c *Conductor) Load() error {
	return c.load(false)
}

func (c *Conductor) reload() error {
	data, err := c.loadJSONHTTP()
	if err != nil {
		return err
	}
	err = c.decodeJSON(data)
	if err != nil {
		// Something's wrong with backend, falling back to (expired) cache
		return err
	}
	c.saveCache()
	c.build()
	return nil
}

func (c *Conductor) load(expiredOk bool) error {
	var data []byte
	var err error

	cacheLoaded := false
	data, err = c.loadJSONCache()
	if err == nil {
		err = c.decodeJSON(data)
		if err == nil {
			expTime := c.data.CreatedAt.Add(c.config.CacheTTL)
			if expTime.After(time.Now()) || expiredOk {
				cacheLoaded = true
			}
		}
	}

	if !cacheLoaded {
		if expiredOk {
			// at this point something's wrong with cache
			// either it can't be loaded or decoded
			// and we've already tried http load and failed
			return fmt.Errorf("Can't load data neither from cache nor from http")
		}
		data, err = c.loadJSONHTTP()
		if err != nil {
			// Something's wrong with backend, falling back to (expired) cache
			return c.load(true)
		}
		err = c.decodeJSON(data)
		if err != nil {
			// Something's wrong with backend, falling back to (expired) cache
			return c.load(true)
		}
		c.saveCache()
	}
	c.build()
	return nil
}

func (c *Conductor) build() {
	for _, dc := range c.data.Data.Datacenters {
		c.cache.datacenters._id[dc.ID] = dc
		c.cache.datacenters.name[dc.Name] = dc
	}

	for _, dc := range c.data.Data.Datacenters {
		if pdc, found := c.cache.datacenters._id[dc.ParentID]; found {
			dc.Parent = pdc
		}
	}

	for _, workgroup := range c.data.Data.WorkGroups {
		workgroup.Groups = make([]*Group, 0)
		c.cache.workgroups._id[workgroup.ID] = workgroup
		c.cache.workgroups.name[workgroup.Name] = workgroup
	}

	for _, group := range c.data.Data.Groups {
		group.Hosts = make([]*Host, 0)
		c.cache.groups._id[group.ID] = group
		c.cache.groups.name[group.Name] = group
		wg := c.cache.workgroups._id[group.WorkGroupID]
		if wg != nil {
			wg.Groups = append(c.cache.workgroups._id[group.WorkGroupID].Groups, group)
		}
	}

	for _, host := range c.data.Data.Hosts {
		c.cache.hosts._id[host.ID] = host
		c.cache.hosts.fqdn[host.FQDN] = host
		if host.GroupID != "" {
			c.cache.groups._id[host.GroupID].Hosts = append(c.cache.groups._id[host.GroupID].Hosts, host)
		}
		if host.DatacenterID != "" {
			host.Datacenter = c.cache.datacenters._id[host.DatacenterID]
		}
	}
}

func sliceIndex(s []string, t string) int {
	for i := 0; i < len(s); i++ {
		if t == s[i] {
			return i
		}
	}
	return -1
}

func CompleteHost(line string) []string {
	res := make([]string, 0)
	for hostname := range cGlobal.cache.hosts.fqdn {
		if line == "" || strings.HasPrefix(hostname, line) {
			res = append(res, hostname[len(line):])
		}
	}
	sort.Strings(res)
	return res
}

func CompleteGroup(line string) []string {
	expr := line
	if strings.HasPrefix(expr, "%") {
		expr = expr[1:]
	}
	res := make([]string, 0)
	for groupname := range cGlobal.cache.groups.name {
		if expr == "" || strings.HasPrefix(groupname, expr) {
			res = append(res, groupname[len(expr):])
		}
	}
	sort.Strings(res)
	return res
}

func CompleteWorkGroup(line string) []string {
	expr := line
	if strings.HasPrefix(expr, "*") {
		expr = expr[1:]
	}
	res := make([]string, 0)
	for wgname := range cGlobal.cache.workgroups.name {
		if expr == "" || strings.HasPrefix(wgname, expr) {
			res = append(res, wgname[len(expr):])
		}
	}
	sort.Strings(res)
	return res
}

func CompleteDatacenter(line string) []string {
	expr := line
	if strings.HasPrefix(expr, "@") {
		expr = expr[1:]
	}
	res := make([]string, 0)
	for dcname := range cGlobal.cache.datacenters.name {
		if expr == "" || strings.HasPrefix(dcname, expr) {
			res = append(res, dcname[len(expr):])
		}
	}
	sort.Strings(res)
	return res
}

func HostList(expr []rune) ([]string, error) {
	tokens, err := ParseExpression(expr)
	if err != nil {
		return nil, err
	}

	hostlist := make([]string, 0)

	for _, token := range tokens {
		switch token.Type {
		case TTypeHost:

			hosts, err := sekwence.ExpandPattern(token.Value)
			if err != nil {
				hosts = []string{token.Value}
			}

			for _, host := range hosts {
				if len(token.TagsFilter) > 0 {
					invhost, found := cGlobal.cache.hosts.fqdn[host]
					if !found {
						continue
					}
					for _, tag := range token.TagsFilter {
						if !contains(invhost.AllTags, tag) {
							continue
						}
					}
				}
				if token.Exclude {
					hIdx := sliceIndex(hostlist, host)
					if hIdx >= 0 {
						hostlist = append(hostlist[:hIdx], hostlist[hIdx+1:]...)
					}
				} else {
					hostlist = append(hostlist, host)
				}
			}

		case TTypeGroup:
			if group, found := cGlobal.cache.groups.name[token.Value]; found {
				hosts := group.AllHosts()

			hostLoop1:
				for _, host := range hosts {
					if token.DatacenterFilter != "" {
						if host.Datacenter == nil {
							continue
						}
						if host.Datacenter.Name != token.DatacenterFilter {
							// TODO tree
							continue
						}
					}

					for _, tag := range token.TagsFilter {
						if !contains(host.AllTags, tag) {
							continue hostLoop1
						}
					}

					if token.Exclude {
						hIdx := sliceIndex(hostlist, host.FQDN)
						if hIdx >= 0 {
							hostlist = append(hostlist[:hIdx], hostlist[hIdx+1:]...)
						}
					} else {
						hostlist = append(hostlist, host.FQDN)
					}
				}
			}

		case TTypeWorkGroup:
			if wg, found := cGlobal.cache.workgroups.name[token.Value]; found {
				groups := wg.Groups
				hosts := make([]*Host, 0)
				for _, group := range groups {
					hosts = append(hosts, group.Hosts...)
				}

			hostLoop2:
				for _, host := range hosts {
					if token.DatacenterFilter != "" {
						if host.Datacenter == nil {
							continue
						}
						if host.Datacenter.Name != token.DatacenterFilter {
							// TODO tree
							continue
						}
					}
					for _, tag := range token.TagsFilter {
						if !contains(host.AllTags, tag) {
							continue hostLoop2
						}
					}

					if token.Exclude {
						hIdx := sliceIndex(hostlist, host.FQDN)
						if hIdx >= 0 {
							hostlist = append(hostlist[:hIdx], hostlist[hIdx+1:]...)
						}
					} else {
						hostlist = append(hostlist, host.FQDN)
					}
				}
			}
		}
	}

	return hostlist, nil
}

func contains(array []string, elem string) bool {
	for _, item := range array {
		if elem == item {
			return true
		}
	}
	return false
}

// Reload tries to load groups from inventoree only
func Reload() error {
	if cGlobal == nil {
		return fmt.Errorf("conductor object is not initialized")
	}
	return cGlobal.reload()
}
