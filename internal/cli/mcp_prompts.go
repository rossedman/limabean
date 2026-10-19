package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpPrompt is a workflow the MCP server offers as a prompt: Claude Code
// shows each as a slash command (/mcp__limabean__<name>), Claude Desktop
// in its attachments menu. A prompt is only text for Claude: it runs
// nothing itself, so every tool's guards still apply, and each one asks
// Claude to keep them (show plans, never widen a network, leave deleting
// and rolling back to the user).
type mcpPrompt struct {
	name, title, description string
	args                     []*mcp.PromptArgument
	text                     func(args map[string]string) string
}

// promptRules ends every prompt: the guards Claude should keep however the
// workflow goes.
const promptRules = `

Keep to these throughout:
- Everything runs inside a VM, through limabean's tools; nothing runs on my Mac.
- Show me a plan before anything that creates or changes a VM, and wait for my yes.
- Never try to widen a VM's network. If something is blocked, network_log shows what; tell me, and give me the limabean network allow command to run if I agree.
- Don't delete VMs or restore snapshots yourself; tell me the command instead.
- A VM's clone is separate from my checkout on the Mac: say so when you change files in it.`

var mcpPrompts = []mcpPrompt{
	{
		name:        "test_in_sandbox",
		title:       "Test a checkout in a sandbox",
		description: "Bring up a checkout's VM, writing its .limabean.yaml first if it has none, and run its tests there.",
		args: []*mcp.PromptArgument{
			{Name: "dir", Description: "absolute path of the checkout on the Mac", Required: true},
		},
		text: func(a map[string]string) string {
			return fmt.Sprintf(`Run the tests of the checkout at %s in a limabean VM.

1. If it has no .limabean.yaml, draft one with project_init and show it to me. Suggest a network locked to what the project needs (github, plus its package registries), and write the file only once I agree.
2. Show me project_plan's output, then bring the VM up with project_up.
3. Work out the test command from the repository (its Makefile, package.json, go.mod, pyproject.toml and the like) and run it with project_run.
4. Report what passed and what failed, with the failures' output. If setup or a test failed because something was blocked, say what.`, a["dir"])
		},
	},
	{
		name:        "triage_repo",
		title:       "Triage an unfamiliar repository",
		description: "Clone a repository into a new locked VM, install it and run its tests, and report what works.",
		args: []*mcp.PromptArgument{
			{Name: "repo", Description: "owner/repo, host/owner/repo or a URL", Required: true},
			{Name: "allow", Description: "network presets or domains the VM may reach; default github and the registries the project needs"},
		},
		text: func(a map[string]string) string {
			allow := a["allow"]
			if strings.TrimSpace(allow) == "" {
				allow = "github, plus the package registries the project needs (tell me which before you create the VM)"
			}
			return fmt.Sprintf(`Triage the repository %s in a new limabean VM.

1. Create a VM named after the repository with create_vm, locked, allowing only: %s. Give it the packs its language needs.
2. Save a snapshot named fresh with snapshot_save, then clone the repository with clone_repo.
3. Install its dependencies and run its tests with run_in_vm, in ~/code/<host>/<owner>/<repo>.
4. Report: what the project is, whether it builds, what the tests say, and anything that needed a host the VM couldn't reach. Leave the VM running for me to look at.`, a["repo"], allow)
		},
	},
	{
		name:        "try_a_change",
		title:       "Try a change safely",
		description: "Snapshot a checkout's VM, make a change in its clone, run the tests and show the diff.",
		args: []*mcp.PromptArgument{
			{Name: "dir", Description: "absolute path of the checkout on the Mac; it needs a .limabean.yaml", Required: true},
			{Name: "change", Description: "what to change, e.g. upgrade to Go 1.27", Required: true},
		},
		text: func(a map[string]string) string {
			return fmt.Sprintf(`Try this change to the checkout at %s, in its limabean VM: %s

1. Make sure its VM is up (project_status; project_up if it isn't, showing me project_plan first).
2. Save a snapshot with snapshot_save before changing anything.
3. Make the change in the VM's clone with project_run, then run the tests there.
4. Show me git diff from the clone, and the test results.
5. If it didn't work, explain why, and give me the limabean snapshot restore command to roll the VM back.`, a["dir"], a["change"])
		},
	},
	{
		name:        "run_untrusted",
		title:       "Run something you don't trust",
		description: "Run a script, tool or repository in a throwaway locked VM that deletes itself, and report what it did.",
		args: []*mcp.PromptArgument{
			{Name: "what", Description: "what to run: a repository, a URL to a script, or a package and its command", Required: true},
		},
		text: func(a map[string]string) string {
			return fmt.Sprintf(`Run this in a throwaway limabean VM and tell me what it does: %s

1. Create a new VM with create_vm: locked, allowing only what's needed to fetch it (ask me if that's unclear), and a ttl of 2h so it deletes itself.
2. Fetch it inside the VM and read it before running it: tell me anything that looks risky. A new VM gets my shared secrets (limabean secrets ls lists them), so remind me of that, and wait for my yes before running it.
3. Run it with run_in_vm.
4. Report what it did, what it printed, and every host it tried to reach, from network_log, including the blocked ones.`, a["what"])
		},
	},
	{
		name:        "tidy_vms",
		title:       "Tidy up VMs",
		description: "Review your VMs, stop the ones you're not using, and list the ones to delete.",
		text: func(map[string]string) string {
			return `Help me tidy up my limabean VMs.

1. List them with list_vms.
2. Suggest which running ones to stop and which to delete, and why (stopped for a long time, expiring, scratch names).
3. Stop the ones I agree to with stop_vm.
4. For the ones to delete, give me the limabean rm commands: rm checks each for unpushed work before it deletes.`
		},
	},
}

// addPrompts offers the workflows above.
func addPrompts(s *mcp.Server) {
	for _, p := range mcpPrompts {
		p := p
		s.AddPrompt(&mcp.Prompt{Name: p.name, Title: p.title, Description: p.description, Arguments: p.args},
			func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				args := req.Params.Arguments
				for _, a := range p.args {
					if a.Required && strings.TrimSpace(args[a.Name]) == "" {
						return nil, fmt.Errorf("%s needs %s: %s", p.name, a.Name, a.Description)
					}
				}
				return &mcp.GetPromptResult{
					Description: p.description,
					Messages: []*mcp.PromptMessage{{
						Role:    "user",
						Content: &mcp.TextContent{Text: p.text(args) + promptRules},
					}},
				}, nil
			})
	}
}
