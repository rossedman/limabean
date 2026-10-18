package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/project"
	"github.com/rossedman/limabean/internal/secrets"
)

// upChanges is what it takes to bring an existing VM in line with its
// project file.
type upChanges struct {
	lines []string // what changes, for the plan

	recreate bool     // packs or environment differ: rebuild, keeping ~/code
	env      string   // the file's environment
	packs    []string // the file's packs
	tools    projectTools

	cpus, memory, disk int // a new size, where it differs (0: as it is)

	policy   *netPolicy // a tighter network policy to apply, if any
	widening []string   // commands for network changes the file can't make

	secrets []secrets.Secret // the file's secrets the VM doesn't have yet

	start bool // start commands that differ from the ones last approved

	vars bool // variables that differ from the ones last applied
}

// empty reports whether the VM already matches the file.
func (c upChanges) empty() bool { return len(c.lines) == 0 }

// upDiff compares a VM with its project file. It reads the VM's saved
// packs, environment and network policy and Lima's view of its size, so it
// works for VMs made before up kept track of anything.
func (a *App) upDiff(inst lima.Instance, f *project.File, own []secrets.Secret) (upChanges, error) {
	c := upChanges{env: firstNonEmpty(f.Env, "scratch")}
	packs, err := a.resolvePacks(nil, f.Packs, nil)
	if err != nil {
		return c, err
	}
	c.packs = packs
	have, known := a.packsOf(inst.Name)
	if !known || !slices.Equal(have, packs) {
		c.recreate = true
		c.lines = append(c.lines, fmt.Sprintf("packs: %s → %s (rebuild, keeping ~/code)", packList(have, known), packList(packs, true)))
	}
	c.tools = toolsOfFile(f)
	if lines := toolsChanges(a.toolsOf(inst.Name), c.tools); len(lines) > 0 {
		c.recreate = true
		c.lines = append(c.lines, lines...)
	}
	if env := firstNonEmpty(a.envOf(inst.Name), "scratch"); env != c.env {
		c.recreate = true
		c.lines = append(c.lines, fmt.Sprintf("env: %s → %s (rebuild, keeping ~/code)", env, c.env))
	}

	if f.CPUs > 0 && f.CPUs != inst.CPUs {
		c.cpus = f.CPUs
		c.lines = append(c.lines, fmt.Sprintf("cpus: %d → %d", inst.CPUs, f.CPUs))
	}
	if f.Memory > 0 && int64(f.Memory) != inst.MemoryGiB() {
		c.memory = f.Memory
		c.lines = append(c.lines, fmt.Sprintf("memory: %d → %d GiB", inst.MemoryGiB(), f.Memory))
	}
	if disk := int(inst.Disk >> 30); f.Disk > disk {
		c.disk = f.Disk
		c.lines = append(c.lines, fmt.Sprintf("disk: %d → %d GiB", disk, f.Disk))
	} else if f.Disk > 0 && f.Disk < disk {
		a.UI.Warn("The file asks for a %d GiB disk, but %s's is %d GiB and Lima can't shrink one", f.Disk, inst.Name, disk)
	}
	if c.recreate && (c.cpus > 0 || c.memory > 0 || c.disk > 0) {
		c.lines = append(c.lines, "(the rebuild takes the new size)")
	}

	if err := a.diffNetwork(&c, inst, f); err != nil {
		return c, err
	}

	if lines := varsChanges(a.appliedVars(inst.Name), f.Vars); len(lines) > 0 {
		c.vars = true
		c.lines = append(c.lines, lines...)
	}

	if f.Start != a.approvedStart(inst.Name) {
		c.start = true
		if f.Start == "" {
			c.lines = append(c.lines, "start: none")
		} else {
			c.lines = append(c.lines, startPlan(f.Start)...)
		}
	}

	mine, _ := secrets.Load(a.Cfg.Dir(), inst.Name)
	for _, s := range own {
		if !slices.ContainsFunc(mine, func(m secrets.Secret) bool { return m.Name == s.Name }) {
			c.secrets = append(c.secrets, s)
			c.lines = append(c.lines, "secret: add "+s.Name)
		}
	}
	return c, nil
}

// diffNetwork compares the VM's network policy with the file's. A file may
// tighten a policy (fewer domains, locked) but never loosen one: domains
// it adds and a lock it drops are left to the user's own network commands.
func (a *App) diffNetwork(c *upChanges, inst lima.Instance, f *project.File) error {
	want, err := expandAllow(f.Network.Allow)
	if err != nil {
		return err
	}
	slices.Sort(want)
	want = slices.Compact(want)
	if !inst.NetworkPolicy {
		if len(want) > 0 || f.Network.Locked {
			c.widening = append(c.widening, "limabean rebuild "+inst.Name+"   (made before network policies)")
		}
		return nil
	}
	p, err := a.loadPolicy(inst.Name)
	if err != nil {
		return err
	}
	next := p
	switch {
	case len(want) == 0 && len(p.Allow) > 0:
		c.widening = append(c.widening, "limabean network open "+inst.Name)
	case len(want) > 0 && len(p.Allow) == 0:
		next.Allow = want
		c.lines = append(c.lines, "network: only "+strings.Join(f.Network.Allow, ", "))
	case len(want) > 0:
		var keep, drop, add []string
		for _, d := range p.Allow {
			if slices.Contains(want, d) {
				keep = append(keep, d)
			} else {
				drop = append(drop, d)
			}
		}
		for _, d := range want {
			if !slices.Contains(p.Allow, d) {
				add = append(add, d)
			}
		}
		if len(add) > 0 {
			c.widening = append(c.widening, "limabean network allow "+inst.Name+" "+strings.Join(add, " "))
		}
		switch {
		case len(drop) == 0:
		case len(keep) == 0:
			// Removing every domain would open the network.
			c.widening = append(c.widening, "limabean network rm "+inst.Name+" "+strings.Join(drop, " ")+"   (after the allow above)")
		default:
			next.Allow = keep
			c.lines = append(c.lines, "network: stop allowing "+strings.Join(drop, ", "))
		}
	}
	switch {
	case f.Network.Locked && !p.Locked:
		next.Locked = true
		c.lines = append(c.lines, "network: lock (no sudo or Docker for the VM's user)")
	case !f.Network.Locked && p.Locked:
		c.widening = append(c.widening, "limabean network unlock "+inst.Name)
	}
	if next.Locked != p.Locked || !slices.Equal(next.Allow, p.Allow) {
		c.policy = &next
	}
	return nil
}

// packList shows packs for the plan.
func packList(packs []string, known bool) string {
	if !known {
		return "unknown"
	}
	return strings.Join(append([]string{"core"}, packs...), ", ")
}

// upExisting brings an existing project VM in line with its file: it shows
// what differs, asks, and applies it, then starts the VM if it's stopped.
func (a *App) upExisting(ctx context.Context, inst lima.Instance, f *project.File, dir string, opt upOptions) error {
	own, missing := a.projectSecrets(f.Secrets)
	c, err := a.upDiff(inst, f, own)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		a.UI.Warn("Not set up on this Mac, so the VM won't get them: %s", strings.Join(missing, ", "))
	}
	if !c.empty() {
		a.UI.Section(f.Name, "changes from "+shortPath(filepath.Join(dir, project.FileName)))
		a.UI.Hint(c.lines...)
	}
	if opt.dryRun {
		if len(c.widening) > 0 {
			a.UI.Warn("The file allows more than %s does now, which only you can change:", f.Name)
			a.UI.Hint(c.widening...)
		}
		if c.empty() {
			a.UI.OK("%s is up to date", a.UI.Code(f.Name))
		} else {
			a.UI.Info("Nothing changed (--preview)")
		}
		return nil
	}
	if !c.empty() {
		if !opt.yes {
			if !a.UI.Prompting {
				return usageErr("check the changes above, then run again with --yes to apply them")
			}
			ok, err := a.UI.Confirm("Apply these changes to "+f.Name+"?", true)
			if err != nil {
				return err
			}
			if !ok {
				a.UI.Info("Left %s as it is", f.Name)
				c = upChanges{widening: c.widening}
			}
		}
	}
	if inst, err = a.upApply(ctx, inst, f, c); err != nil {
		return err
	}
	if c.start {
		if err := a.approveStart(inst.Name, f.Start); err != nil {
			return err
		}
	}
	// A rebuild starts from a fresh system disk, so it needs the file again.
	if c.vars || (c.recreate && len(f.Vars) > 0) {
		if err := a.writeVars(ctx, inst, f.Vars); err != nil {
			return err
		}
		if err := a.saveVars(inst.Name, f.Vars); err != nil {
			return err
		}
	}
	if len(c.widening) > 0 {
		a.UI.Warn("The file allows more than %s does now, which only you can change:", f.Name)
		a.UI.Hint(c.widening...)
	}
	a.upClaude(ctx, inst, f.Claude)
	clone := ""
	if opt.setup {
		repo := a.projectRepo(ctx, f, dir)
		if repo == "" {
			return errors.New("no repository to run setup in: the checkout has no origin, and the file names no repo")
		}
		if clone, err = a.cloneForRun(ctx, inst, repo); err != nil {
			return err
		}
		if f.Setup == "" {
			a.UI.Info("The file has no setup to run")
		} else if err := a.upSetup(inst, f, clone); err != nil {
			return err
		}
	}
	if f.Start != "" {
		if f.Start != a.approvedStart(inst.Name) {
			a.UI.Warn("start changed and wasn't approved, so it didn't run")
		} else {
			if clone == "" {
				clone = a.projectDir(ctx, inst, f, dir)
			}
			if err := a.runStart(ctx, inst, f, clone); err != nil {
				return err
			}
		}
	}
	if c.empty() {
		a.UI.OK("%s is up to date", a.UI.Code(f.Name))
	}
	a.upBox(f, clone)
	return nil
}

// upApply makes the changes c lists and returns the VM, running.
func (a *App) upApply(ctx context.Context, inst lima.Instance, f *project.File, c upChanges) (lima.Instance, error) {
	if c.recreate && a.noRecreate {
		return inst, fmt.Errorf("the file's packs, tools or environment changed, which rebuilds %s (keeping ~/code); the user can run limabean up, or start the MCP server with --allow-destructive", inst.Name)
	}
	if c.recreate {
		keepsCode := a.hasCodeDisk(ctx, inst.Name)
		if !keepsCode {
			clean, err := a.checkUnsaved(ctx, inst)
			if err != nil {
				return inst, err
			}
			if !clean {
				return inst, fmt.Errorf("%s has no separate ~/code disk, so a rebuild would lose the work above; push it, or rebuild by hand: limabean rebuild %s", inst.Name, inst.Name)
			}
		}
		if f.CPUs > 0 {
			a.Cfg.CPUs = f.CPUs
		}
		if f.Memory > 0 {
			a.Cfg.Memory = fmt.Sprint(f.Memory)
		}
		if f.Disk > 0 {
			a.Cfg.Disk = f.Disk
		}
		if c.policy != nil {
			if err := a.savePolicy(inst.Name, *c.policy); err != nil {
				return inst, err
			}
		}
		if err := a.putSecrets(inst.Name, c.secrets); err != nil {
			return inst, err
		}
		if err := a.recreate(ctx, inst, c.env, c.packs, c.tools, keepsCode); err != nil {
			return inst, err
		}
		return a.vm(ctx, inst.Name)
	}

	if c.cpus > 0 || c.memory > 0 || c.disk > 0 {
		if inst.Running() {
			if err := a.limaStep(ctx, "Stopping "+inst.Name+" to resize it", func(ctx context.Context) ([]byte, error) {
				return a.Lima.Stop(ctx, inst.Name)
			}); err != nil {
				return inst, err
			}
		}
		if err := a.limaStep(ctx, "Resizing "+inst.Name, func(ctx context.Context) ([]byte, error) {
			return a.Lima.Edit(ctx, inst.Name, c.cpus, c.memory, c.disk)
		}); err != nil {
			return inst, err
		}
		inst.Status = "Stopped"
	}
	inst, err := a.ensureRunning(ctx, inst)
	if err != nil {
		return inst, err
	}
	if c.policy != nil {
		if err := a.savePolicy(inst.Name, *c.policy); err != nil {
			return inst, err
		}
		if err := a.applyPolicy(ctx, inst); err != nil {
			return inst, err
		}
		a.UI.OK("%s's network: %s", a.UI.Code(inst.Name), c.policy.summary())
	}
	if len(c.secrets) > 0 {
		if err := a.putSecrets(inst.Name, c.secrets); err != nil {
			return inst, err
		}
		if err := a.syncSecrets(ctx, inst.Name); err != nil {
			return inst, err
		}
	}
	if !c.empty() {
		a.UI.OK("Applied the changes to %s", a.UI.Code(inst.Name))
	}
	return inst, nil
}

// putSecrets adds secrets to a VM's own secrets file.
func (a *App) putSecrets(vm string, list []secrets.Secret) error {
	for _, s := range list {
		if err := secrets.Put(secrets.VMFile(a.Cfg.Dir(), vm), s.Name, s.Ref); err != nil {
			return err
		}
	}
	return nil
}
