package lima

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// FuzzParseList feeds parseList arbitrary `limactl list --json` output,
// seeded with the real lines from the package's tests.
func FuzzParseList(f *testing.F) {
	f.Add([]byte(listJSON))
	for _, line := range strings.Split(listJSON, "\n") {
		f.Add([]byte(line))
	}
	f.Add([]byte(`{"name":"code-abc123","size":53687091200,"format":"raw","dir":"/h/.lima/_disks/code-abc123"}` + "\r\n{\"name\":\"data\",\"size\":1}\n"))
	f.Add([]byte("{not json\n"))
	f.Fuzz(func(t *testing.T, out []byte) {
		list, err := parseList(out)
		if err != nil {
			if list != nil {
				t.Fatalf("parseList returned %d instances with an error: %v", len(list), err)
			}
			return
		}
		// One instance per line that starts with {, in order, each the JSON
		// on its line and flagged from that line alone.
		var lines [][]byte
		for _, line := range bytes.Split(out, []byte("\n")) {
			if line = bytes.TrimSpace(line); len(line) > 0 && line[0] == '{' {
				lines = append(lines, line)
			}
		}
		if len(list) != len(lines) {
			t.Fatalf("parseList returned %d instances for %d JSON lines", len(list), len(lines))
		}
		for i, inst := range list {
			var want Instance
			if err := json.Unmarshal(lines[i], &want); err != nil {
				t.Fatalf("line %d accepted by parseList but not by json: %v", i, err)
			}
			want.Limabean = bytes.Contains(lines[i], []byte("/etc/limabean/mise/"))
			want.NetworkPolicy = bytes.Contains(lines[i], []byte(`"/mnt/host/limabean"`))
			if inst != want {
				t.Fatalf("instance %d = %+v, want %+v", i, inst, want)
			}
		}
	})
}
