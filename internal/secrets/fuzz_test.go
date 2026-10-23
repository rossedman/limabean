package secrets

import (
	"regexp"
	"strings"
	"testing"
)

// secretsSample is a reference file from TestParse: good lines, quoted
// references, a blank value and lines Parse must refuse.
const secretsSample = `# a comment

OPENAI_API_KEY=op://Private/OpenAI/key
export NPM_TOKEN = keychain:npm-token
QUOTED="cmd:echo hi"
SINGLE='env:HOME'
REGION=us-east-1
EMPTY=
bad line
1BAD=x
`

// The only errors Parse may return name the file and a line number, or the
// scanner's own failure: never a line's contents, which could be a value
// pasted by mistake.
var parseErrRE = regexp.MustCompile(`^secrets\.env (line [0-9]+: expected NAME=reference|: bufio\.Scanner: token too long)$`)

// FuzzParse feeds Parse arbitrary reference files.
func FuzzParse(f *testing.F) {
	f.Add(secretsSample)
	for _, line := range strings.Split(secretsSample, "\n") {
		f.Add(line)
	}
	f.Add("GITHUB_TOKEN=sk-live-0123456789\nnot a line sk-live-0123456789\n")
	f.Add("export \t A_B = \"'\"\n=value\n\x00=x\n")
	f.Fuzz(func(t *testing.T, in string) {
		got, errs := Parse(strings.NewReader(in), "secrets.env")
		for _, s := range got {
			if s.Name == "" || !ValidName(s.Name) {
				t.Fatalf("returned secret with name %q", s.Name)
			}
			if s.Source != "secrets.env" {
				t.Fatalf("source %q", s.Source)
			}
			if !strings.Contains(in, s.Name) || !strings.Contains(in, s.Ref) {
				t.Fatalf("secret %+v isn't from the input", s)
			}
		}
		for _, err := range errs {
			if err == nil || !parseErrRE.MatchString(err.Error()) {
				t.Fatalf("error %q could carry a line's contents", err)
			}
		}
	})
}

// FuzzUnquote checks unquote takes off one pair of matching quotes and
// nothing else.
func FuzzUnquote(f *testing.F) {
	for _, s := range []string{`"cmd:echo hi"`, `'env:HOME'`, `"`, `''`, `"mismatched'`, "plain", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := unquote(s)
		switch {
		case got == s:
			if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
				t.Fatalf("unquote(%q) kept its quotes", s)
			}
		case len(got) == len(s)-2 && s[1:len(s)-1] == got:
			if s[0] != s[len(s)-1] || (s[0] != '"' && s[0] != '\'') {
				t.Fatalf("unquote(%q) = %q took off quotes that don't match", s, got)
			}
		default:
			t.Fatalf("unquote(%q) = %q", s, got)
		}
	})
}
