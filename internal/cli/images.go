package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/atomic"
	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/templates"
)

func newImagesCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "images",
		Short: "Manage the images that make new VMs start in seconds",
		Long: `New VMs are copies of an image: a VM that has already booted, installed
every update, Docker and its packs' tools, and then shut down. Copying it
is an APFS clone, so "limabean up", "run" and "rebuild" take
seconds instead of minutes.

There's an image for each environment and set of packs, built from the
current templates. The first VM of a kind is made the slow way, and its
image is built in the background for the next one. An image more than a
week old is still used, and replaced in the background. When the templates
or packs change, the next VM is made the slow way and a new image is built.

Images are hidden from "limabean ls". Each takes a few GiB, shared with the
VMs copied from it until they change. IMAGES="no" in the config turns them
off.`,
		Example: `  limabean images ls

  # Build one now, so even the first VM of a kind is fast
  limabean images build --with rust

  limabean images prune
  limabean images rm --all`,
	}
	cmd.AddCommand(newImagesLsCmd(a), newImagesBuildCmd(a), newImagesRmCmd(a), newImagesPruneCmd(a))
	return cmd
}

const (
	imagePrefix = "limabean-img-"
	// imageMaxAge is how old an image gets before it's replaced: VMs copied
	// from it have the updates it had then.
	imageMaxAge = 7 * 24 * time.Hour
	// imageFormat changes when what goes into an image changes, so images
	// from older limabean versions aren't used.
	imageFormat = "1"
)

// isImage reports whether a Lima instance is one of limabean's images.
func isImage(name string) bool { return strings.HasPrefix(name, imagePrefix) }

// imageLock is the lock file for an image key (or the start of one, as an
// image's name has it) and a purpose: "build" while one is built, "use"
// while one is copied.
func (a *App) imageLock(key, purpose string) string {
	return filepath.Join(a.imagesDir(), key[:8]+"."+purpose)
}

// image is what limabean keeps about an image, in images/<name>.json.
type image struct {
	Name      string    `json:"name"`
	Key       string    `json:"key"`       // what it was built for: see imageKey
	Templates string    `json:"templates"` // hash of the templates it was built from
	Env       string    `json:"env"`
	Packs     []string  `json:"packs"`
	Built     time.Time `json:"built"`
	Used      time.Time `json:"used"` // last copied
}

func (im image) stale() bool { return time.Since(im.Built) > imageMaxAge }

func (a *App) imagesDir() string { return filepath.Join(a.stateDir, "images") }

// templatesHash hashes every file under the templates directory, so any
// change to a template, script or pack means a new image.
func templatesHash(root string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00", filepath.ToSlash(rel), len(b))
		h.Write(b)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil)), err
}

// imageKey names what an image holds: the templates, the environment and
// the packs. VMs with the same key can be copied from the same image.
func imageKey(templates, env string, packs []string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{imageFormat, templates, env, strings.Join(packs, ",")}, "\x00")))
	return hex.EncodeToString(sum[:])
}

// images returns the images that exist, newest first. Records of images
// that were deleted behind limabean's back are removed.
func (a *App) images(ctx context.Context) ([]image, error) {
	list, err := a.Lima.List(ctx)
	if err != nil {
		return nil, err
	}
	exists := map[string]bool{}
	for _, inst := range list {
		exists[inst.Name] = isImage(inst.Name)
	}
	files, _ := filepath.Glob(filepath.Join(a.imagesDir(), imagePrefix+"*.json"))
	var out []image
	for _, f := range files {
		var im image
		b, err := os.ReadFile(f)
		if err != nil || json.Unmarshal(b, &im) != nil {
			continue
		}
		if !exists[im.Name] {
			_ = os.Remove(f)
			continue
		}
		out = append(out, im)
	}
	slices.SortFunc(out, func(x, y image) int { return y.Built.Compare(x.Built) })
	return out, nil
}

// imageFor returns the newest image for key, if there is one.
func (a *App) imageFor(ctx context.Context, key string) (image, bool) {
	list, err := a.images(ctx)
	if err != nil {
		return image{}, false
	}
	for _, im := range list {
		if im.Key == key {
			return im, true
		}
	}
	return image{}, false
}

func (a *App) saveImage(im image) error {
	if err := os.MkdirAll(a.imagesDir(), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(im, "", "  ")
	return atomic.WriteFile(filepath.Join(a.imagesDir(), im.Name+".json"), b, 0o644)
}

// imageCleanScript runs in a new image before it's shut down. An empty
// machine-id gives each copy its own on first boot, as cloud images do.
const imageCleanScript = `sudo apt-get clean
sudo truncate -s 0 /etc/machine-id
rm -f ~/.zsh_history ~/.bash_history
sudo journalctl --rotate --vacuum-time=1s >/dev/null 2>&1 || true`

// buildImage builds an image of env with packs: a VM provisioned as any
// other, with no secrets, code disk or network policy, cleaned and shut
// down. Older images for the same key are deleted once it's ready.
func (a *App) buildImage(ctx context.Context, root, env string, packs []string) (image, error) {
	tmpl, err := templates.Path(root, env)
	if err != nil {
		return image{}, err
	}
	hash, err := templatesHash(root)
	if err != nil {
		return image{}, err
	}
	sets, err := packProvision(root, packs)
	if err != nil {
		return image{}, err
	}
	// Provisioning an image downloads as much as a VM does.
	sets = append(sets, a.caCertSets(ctx)...)
	key := imageKey(hash, env, packs)
	now := time.Now()
	im := image{Name: imagePrefix + key[:8] + "-" + strconv.FormatInt(now.UnixMilli(), 36), Key: key, Templates: hash, Env: env, Packs: packs, Built: now, Used: now}
	open := filepath.Join(a.imagesDir(), "open-network")
	if err := os.MkdirAll(open, 0o755); err != nil {
		return image{}, err
	}
	opts := lima.StartOptions{
		Name:     im.Name,
		Template: tmpl,
		Params:   map[string]string{"gitConfigDir": a.Cfg.GitConfigDir, "codeDir": a.Cfg.CodeDir, "networkDir": open},
		Sets:     sets,
	}
	desc := env
	if len(packs) > 0 {
		desc += " + " + strings.Join(packs, ", ")
	}
	err = a.limaStep(ctx, "Building an image of "+desc+". This takes a few minutes…", func(ctx context.Context) ([]byte, error) {
		if out, err := a.Lima.Create(ctx, opts); err != nil {
			return out, err
		}
		inst, _, err := a.Lima.Get(ctx, im.Name)
		if err != nil {
			return nil, err
		}
		if out, err := a.Lima.Run(ctx, inst, strings.NewReader(imageCleanScript), "bash -s"); err != nil {
			return out, err
		}
		return a.Lima.Stop(ctx, im.Name)
	})
	if err != nil {
		_, _ = a.Lima.Delete(context.WithoutCancel(ctx), im.Name)
		return image{}, err
	}
	if err := a.saveImage(im); err != nil {
		return image{}, err
	}
	a.UI.OK("Built image %s", im.Name)
	// Copies in progress hold the key's lock shared; wait for them.
	unlock, err := lockFile(a.imageLock(key, "use"), true, false)
	if err != nil {
		return im, nil
	}
	defer unlock()
	list, _ := a.images(ctx)
	for _, old := range list {
		if old.Key == key && old.Name != im.Name {
			a.removeImage(ctx, old.Name)
		}
	}
	return im, nil
}

// removeImage deletes an image and its record.
func (a *App) removeImage(ctx context.Context, name string) {
	if out, err := a.Lima.Delete(ctx, name); err != nil {
		a.UI.Warn("Couldn't delete image %s: %v %s", name, err, strings.TrimSpace(string(out)))
		return
	}
	_ = os.Remove(filepath.Join(a.imagesDir(), name+".json"))
	a.UI.OK("Deleted image %s", name)
}

// buildInBackground starts "limabean images build" for env and packs as a
// process of its own, which carries on after this one exits. Its output
// goes to the images log.
func (a *App) buildInBackground(env string, packs []string) {
	args := []string{"images", "build", "--plain", "--if-needed", "--env", env, "--packs", strings.Join(packs, ",")}
	if a.configPath != "" {
		if p, err := filepath.Abs(a.configPath); err == nil {
			args = append(args, "--config", p)
		}
	}
	if dir := firstNonEmpty(a.templatesDir, os.Getenv("LIMABEAN_TEMPLATES")); dir != "" {
		if p, err := filepath.Abs(dir); err == nil {
			args = append(args, "--templates", p)
		}
	}
	if err := a.spawn(args...); err != nil {
		a.UI.Warn("Couldn't start building an image: %v", err)
		return
	}
	a.UI.Hint("Building an image in the background, so the next VM like this starts in seconds", "Log: "+shortPath(imagesLog()))
}

// imagesLog is where background image builds write.
func imagesLog() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Logs", "limabean-images.log")
}

// spawn runs this limabean with args in the background, detached from the
// terminal.
func spawn(args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(imagesLog()), 0o755); err != nil {
		return err
	}
	log, err := os.OpenFile(imagesLog(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = log.Close() }()
	_, _ = fmt.Fprintf(log, "\n%s limabean %s\n", time.Now().Format(time.RFC3339), strings.Join(args, " "))
	cmd := exec.Command(exe, args...)
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// errLocked means another limabean holds a lock.
var errLocked = errors.New("locked")

// lockFile takes an flock on path, exclusive or shared, waiting for it or
// failing with errLocked. The lock goes when unlock is called or the
// process ends.
func lockFile(path string, exclusive, wait bool) (unlock func(), err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	how := syscall.LOCK_SH
	if exclusive {
		how = syscall.LOCK_EX
	}
	if !wait {
		how |= syscall.LOCK_NB
	}
	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errLocked
		}
		return nil, err
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
