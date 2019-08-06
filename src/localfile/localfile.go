package localfile

import (
	"config"
	"encoding/json"
	"io/ioutil"
	"os"
	"parser"
	"sort"
	"strings"
	"regexp"

	"github.com/go-ini/ini"
)

type Hosts []string
type LocalFileData map[string]Hosts
type LocalFile struct {
	backend string
	path    string
	data    *LocalFileData
}

func (f *LocalFile) Load() error {
	switch f.backend {
	case "localini":
		cfg, err := ini.LoadSources(ini.LoadOptions{
			AllowBooleanKeys: true,
		}, f.path)
		if err != nil {
			return err
		}

		fData := LocalFileData{}
		for i, group := range cfg.Sections() {
			// no need DEFAULT section from github.com/go-ini/ini
			if i == 0 {
				continue
			}
			fData[group.Name()] = group.KeyStrings()
		}
		f.data = &fData
	case "localjson":
		jsonFile, err := os.Open(f.path)
		defer jsonFile.Close()
		if err != nil {
			return err
		}

		bData, err := ioutil.ReadAll(jsonFile)
		if err != nil {
			return err
		}

		err = json.Unmarshal(bData, &f.data)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *LocalFile) HostList(x []rune) ([]string, error) {
	tokens, err := parser.ParseExpression(x)
	if err != nil {
		return nil, err
	}
	hostlist := make([]string, 0)
	for _, token := range tokens {
		group := *f.data
		hosts := group[token.Value]
		switch token.Type {
		case parser.TTypeHostRegexp:
			for _, host := range f.MatchHost(token.RegexpFilter) {
				parser.MaybeAddHost(&hostlist, host, token.Exclude)
			}
		case parser.TTypeHost:
			if len(hosts) == 0 {
				hosts = []string{token.Value}
			}
			for _, host := range hosts {
				parser.MaybeAddHost(&hostlist, host, token.Exclude)
			}
		case parser.TTypeGroup:
			for _, host := range hosts {
				parser.MaybeAddHost(&hostlist, host, token.Exclude)
			}
		default:
			continue
		}
	}
	return hostlist, nil
}

func (f *LocalFile) Reload() error {
	return f.Load()
}

func (f *LocalFile) MatchHost(pattern *regexp.Regexp) []string {
	res := make([]string, 0)
	for _, group := range *f.data {
		for _, hostname := range group {
			if pattern.MatchString(hostname) {
				res = append(res, hostname)
			}
		}
	}
	sort.Strings(res)
	return res

}

func (f *LocalFile) CompleteHost(line string) []string {
	res := make([]string, 0)
	keys := make(map[string]bool)
	for _, group := range *f.data {
		for _, hostname := range group {
			if line == "" || strings.HasPrefix(hostname, line) {
				host := hostname[len(line):]
				if _, ok := keys[host]; !ok {
					keys[host] = true
					res = append(res, host)
				}
			}
		}
	}
	sort.Strings(res)
	return res
}

func (f *LocalFile) CompleteGroup(line string) []string {
	expr := line
	if strings.HasPrefix(expr, "%") {
		expr = expr[1:]
	}
	res := make([]string, 0)
	for group := range *f.data {
		if expr == "" || strings.HasPrefix(group, expr) {
			res = append(res, group[len(expr):])
		}
	}
	sort.Strings(res)
	return res
}

func (f *LocalFile) CompleteWorkGroup(line string) []string {
	// not realized in json struct now
	return nil
}

func (f *LocalFile) CompleteDatacenter(line string) []string {
	// not realized in json struct now
	return nil
}

func NewFromFile(config *config.XcConfig) *LocalFile {
	return &LocalFile{config.BackendType, config.LocalFile, &LocalFileData{}}
}
