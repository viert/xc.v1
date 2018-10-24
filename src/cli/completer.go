package cli

import (
	"conductor"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type completeFunc func([]rune) ([][]rune, int)

type xcCompleter struct {
	commands   []string
	completers map[string]completeFunc
}

func newXcCompleter(commands []string) *xcCompleter {
	x := &xcCompleter{commands, make(map[string]completeFunc)}
	x.completers["mode"] = staticCompleter([]string{"collapse", "serial", "parallel"})
	x.completers["progressbar"] = staticCompleter([]string{"on", "off"})
	x.completers["raise"] = staticCompleter([]string{"none", "su", "sudo"})
	x.completers["exec"] = x.completeExec
	x.completers["s_exec"] = x.completeExec
	x.completers["c_exec"] = x.completeExec
	x.completers["p_exec"] = x.completeExec
	x.completers["ssh"] = x.completeExec
	x.completers["hostlist"] = x.completeExec
	x.completers["cd"] = x.completeFiles
	return x
}

func wsSplit(line []rune) ([]rune, []rune) {
	sline := string(line)
	tokens := exprWhiteSpace.Split(sline, 2)
	if len(tokens) < 2 {
		return []rune(tokens[0]), nil
	}
	return []rune(tokens[0]), []rune(tokens[1])
}

func runeIndex(line []rune, sym rune) int {
	for i := 0; i < len(line); i++ {
		if line[i] == sym {
			return i
		}
	}
	return -1
}

func toRunes(src []string) [][]rune {
	dst := make([][]rune, len(src))
	for i := 0; i < len(src); i++ {
		dst[i] = []rune(src[i])
	}
	return dst
}

func staticCompleter(variants []string) completeFunc {
	sort.Strings(variants)
	return func(line []rune) (newLine [][]rune, length int) {
		ll := len(line)
		sr := make([]string, 0)
		for _, variant := range variants {
			if strings.HasPrefix(variant, string(line)) {
				sr = append(sr, variant[ll:])
			}
		}
		return toRunes(sr), ll
	}
}

func (x *xcCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	postfix := line[pos:]
	result, length := x.complete(line[:pos])
	if len(postfix) > 0 {
		for i := 0; i < len(result); i++ {
			result[i] = append(result[i], postfix...)
		}
	}
	return result, length
}

func (x *xcCompleter) complete(line []rune) (newLine [][]rune, length int) {
	cmd, args := wsSplit(line)
	if args == nil {
		return x.completeCommand(cmd)
	}

	if handler, found := x.completers[string(cmd)]; found {
		return handler(args)
	}

	return [][]rune{}, 0
}

func (x *xcCompleter) completeCommand(line []rune) (newLine [][]rune, length int) {
	sr := make([]string, 0)
	for _, cmd := range x.commands {
		if strings.HasPrefix(cmd, string(line)) {
			sr = append(sr, cmd[len(line):]+" ")
		}
	}
	sort.Strings(sr)
	return toRunes(sr), len(line)
}

func (x *xcCompleter) completeFiles(line []rune) (newLine [][]rune, length int) {
	ll := len(line)
	path := string(line)
	files, err := filepath.Glob(path + "*")
	if err != nil {
		return [][]rune{}, len(line)
	}

	results := make([][]rune, len(files))
	for i := 0; i < len(files); i++ {
		filename := files[i]
		if st, err := os.Stat(filename); err == nil {
			if st.IsDir() {
				filename += "/"
			}
		}
		results[i] = []rune(filename[ll:])
	}

	return results, ll
}

func (x *xcCompleter) completeExec(line []rune) (newLine [][]rune, length int) {
	_, shellCmd := wsSplit(line)
	if shellCmd != nil {
		return [][]rune{}, 0
	}

	// are we in complex pattern? look for comma
	ci := runeIndex(line, ',')
	if ci >= 0 {
		return x.completeExec(line[ci+1:])
	}

	// here we are exactly in the beginning of the last expression
	if len(line) > 0 && line[0] == '-' {
		// exclusion is excluded from completion
		return x.completeExec(line[1:])
	}

	if len(line) > 0 && line[0] == '%' {
		return x.completeGroup(line)
	}

	if len(line) > 0 && line[0] == '*' {
		return x.completeWorkGroup(line)
	}

	return x.completeHost(line)
}

func (x *xcCompleter) completeGroup(line []rune) (newLine [][]rune, length int) {
	ai := runeIndex(line, '@')
	if ai >= 0 {
		return x.completeDatacenter(line[ai:])
	}
	groups := conductor.CompleteGroup(string(line))
	return toRunes(groups), len(line)
}

func (x *xcCompleter) completeWorkGroup(line []rune) (newLine [][]rune, length int) {
	ai := runeIndex(line, '@')
	if ai >= 0 {
		return x.completeDatacenter(line[ai:])
	}
	wgroups := conductor.CompleteWorkGroup(string(line))
	return toRunes(wgroups), len(line)
}

func (x *xcCompleter) completeHost(line []rune) (newLine [][]rune, length int) {
	hosts := conductor.CompleteHost(string(line))
	return toRunes(hosts), len(line)
}

func (x *xcCompleter) completeDatacenter(line []rune) (newLine [][]rune, length int) {
	dcs := conductor.CompleteDatacenter(string(line))
	return toRunes(dcs), len(line)
}
