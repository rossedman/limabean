package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rossedman/limabean/internal/claude"
)

// imageNames returns the images limabean lists.
func imageNames(t *testing.T, h *harness) []imageEntry {
	t.Helper()
	var list []imageEntry
	if err := json.Unmarshal([]byte(h.must("", "images", "ls", "--json")), &list); err != nil {
		t.Fatal(err)
	}
	return list
}

func TestImagesMakeNewVMsCopies(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_IMAGES", "yes")

	// No image yet: the VM is made the slow way, and one is built in the background.
	out := h.must("", "new", "dev")
	if !h.world.called("limactl start --name=dev") || !h.world.called("spawn images build --plain --if-needed --env scratch --packs containers,db,go,infra,kubernetes,node,python") {
		t.Fatalf("first VM: %v", h.world.calls)
	}
	if !strings.Contains(out, "Building an image in the background") {
		t.Errorf("out:\n%s", out)
	}

	// The build the background process runs.
	h.world.calls = nil
	out = h.must("", "images", "build", "--if-needed", "--env", "scratch", "--packs", "containers,db,go,infra,kubernetes,node,python")
	calls := strings.Join(h.world.calls, "\n")
	if !strings.Contains(calls, "limactl start --name="+imagePrefix) || strings.Contains(calls, "codeDisk") || !strings.Contains(calls, "50-node.toml") || !strings.Contains(calls, "limactl stop "+imagePrefix) {
		t.Fatalf("build: %s", calls)
	}
	if !strings.Contains(calls, "open-network") || strings.Contains(calls, "network/dev") {
		t.Errorf("an image has an open network of its own:\n%s", calls)
	}
	imgs := imageNames(t, h)
	if len(imgs) != 1 || imgs[0].Status != "current" || imgs[0].Env != "scratch" || !strings.Contains(out, "Built image") {
		t.Fatalf("images: %+v\n%s", imgs, out)
	}
	img := imgs[0].Name
	if out := h.must("", "images", "build", "--if-needed", "--packs", "containers,db,go,infra,kubernetes,node,python"); !strings.Contains(out, "up to date") {
		t.Errorf("--if-needed with a fresh image:\n%s", out)
	}

	// Images are hidden, and their names reserved.
	if out := h.must("", "ls"); strings.Contains(out, imagePrefix) {
		t.Errorf("ls shows the image:\n%s", out)
	}
	if _, err := h.run("", "new", imagePrefix+"x"); err == nil {
		t.Error("image names are reserved")
	}

	// The next VM is a copy.
	h.world.calls = nil
	h.world.writeFile(filepath.Join("_disks", codeDiskName("api"), "datadisk"), "")
	out = h.must("", "new", "api", "--cpus", "6", "--memory", "12")
	calls = strings.Join(h.world.calls, "\n")
	for _, want := range []string{
		"limactl clone --tty=false",
		`.param.networkDir = "` + filepath.Join(h.dir, "network", "api") + `"`,
		`.param.codeDisk = "` + codeDiskName("api") + `"`,
		`.additionalDisks += [{"name": "` + codeDiskName("api") + `"`,
		".cpus = 6",
		`.memory = "12GiB"`,
		img + " api",
		"limactl start --tty=false api",
	} {
		if !strings.Contains(calls, want) {
			t.Errorf("copy missing %q:\n%s", want, calls)
		}
	}
	if strings.Contains(calls, "limactl start --name=api") || h.world.called("spawn") || !strings.Contains(out, "from an image built") {
		t.Errorf("a fresh image is copied, and not rebuilt:\n%s\n%s", calls, out)
	}

	// A week-old image is still used, and replaced in the background.
	setBuilt(t, h, img, time.Now().Add(-8*24*time.Hour))
	h.world.calls = nil
	h.must("", "new", "old", "--with", "rust")
	if !h.world.called("limactl start --name=old") {
		t.Errorf("other packs, another image: %v", h.world.calls)
	}
	h.world.calls = nil
	h.must("", "new", "web")
	if !h.world.called("limactl clone") || !h.world.called("spawn images build") {
		t.Errorf("a stale image: %v", h.world.calls)
	}
	if imageNames(t, h)[0].Status != "stale" {
		t.Errorf("status: %+v", imageNames(t, h))
	}

	// A rebuild replaces it, and deletes the old one.
	h.must("", "images", "build")
	if imgs := imageNames(t, h); len(imgs) != 1 || imgs[0].Name == img || h.world.vms[img] != "" {
		t.Errorf("after a rebuild: %+v", imgs)
	}
}

func setBuilt(t *testing.T, h *harness, name string, built time.Time) {
	t.Helper()
	p := filepath.Join(filepath.Dir(h.dir), "state", "limabean", "images", name+".json")
	var im image
	b, _ := os.ReadFile(p)
	if err := json.Unmarshal(b, &im); err != nil {
		t.Fatal(err)
	}
	im.Built, im.Used = built, built
	b, _ = json.Marshal(im)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestImagesOutdatedPruneAndRm(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_IMAGES", "yes")
	t.Setenv("LIMABEAN_CODE_DISK", "")
	h.must("", "images", "build")
	old := imageNames(t, h)[0].Name

	// A template change (here, a pack of your own) outdates every image.
	if err := os.MkdirAll(filepath.Join(h.dir, "templates", "packs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.dir, "templates", "packs", "elixir.toml"), []byte("# Elixir\n[tools]\nelixir = \"1.19.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if imgs := imageNames(t, h); imgs[0].Status != "outdated" {
		t.Errorf("status: %+v", imgs)
	}
	h.world.calls = nil
	h.must("", "new", "dev")
	if h.world.called("limactl clone") || !h.world.called("limactl start --name=dev") {
		t.Errorf("an outdated image isn't used: %v", h.world.calls)
	}

	// A running build is never added to Claude Desktop, or listed.
	h.world.vms[old] = "Running"
	h.must("", "ls")
	if names, _ := (claude.Settings{Path: h.claude}).VMs(); strings.Join(names, ",") != "dev" {
		t.Errorf("Claude entries: %v", names)
	}
	h.world.vms[old] = "Stopped"

	// An image VM with no record: a build in progress is left alone.
	h.world.vms[imagePrefix+"deadbeef-x"] = "Running"
	unlock, err := lockFile(filepath.Join(filepath.Dir(h.dir), "state", "limabean", "images", "deadbeef.build"), true, false)
	if err != nil {
		t.Fatal(err)
	}
	if out := h.must("", "images", "prune"); !strings.Contains(out, "Deleted image "+old) || strings.Contains(out, "deadbeef") {
		t.Errorf("prune:\n%s", out)
	}
	unlock()
	h.world.activity["dev"] = "1000 990 load average 2"
	if out := h.must("", "autostop", "run"); !strings.Contains(out, "Deleted image "+imagePrefix+"deadbeef-x") {
		t.Errorf("autostop run prunes too:\n%s", out)
	}
	if out := h.must("", "images", "prune"); !strings.Contains(out, "Nothing to prune") {
		t.Errorf("prune again:\n%s", out)
	}

	h.must("", "images", "build")
	h.world.vms[imagePrefix+"unknown"] = "Stopped"
	if out := h.must("", "images", "rm", "--all"); strings.Count(out, "Deleted image") != 2 {
		t.Errorf("rm --all:\n%s", out)
	}
	if out := h.must("", "images", "ls"); !strings.Contains(out, "No images yet") {
		t.Errorf("ls:\n%s", out)
	}
	for _, args := range [][]string{{"images", "rm"}, {"images", "rm", "dev"}, {"images", "rm", "x", "--all"}, {"images", "build", "--packs", "go", "--with", "rust"}, {"images", "build", "--with", "cobol"}, {"images", "build", "--env", "nope"}} {
		if _, err := h.run("", args...); err == nil {
			t.Errorf("%v should fail", args)
		}
	}
	if out := h.must("", "images", "rm", "--all"); !strings.Contains(out, "No images") {
		t.Errorf("rm --all with none:\n%s", out)
	}
}

func TestImageBuildFailuresAndLocks(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_IMAGES", "yes")
	h.world.failStart = true
	if _, err := h.run("", "images", "build"); err == nil || !h.world.called("limactl delete --force "+imagePrefix) {
		t.Errorf("a failed build should fail and clean up: %v %v", err, h.world.calls)
	}
	h.world.failStart = false

	// Another build of the same image is running.
	h.must("", "images", "build")
	dir := filepath.Join(filepath.Dir(h.dir), "state", "limabean", "images")
	files, _ := filepath.Glob(filepath.Join(dir, imagePrefix+"*.json"))
	var im image
	b, _ := os.ReadFile(files[0])
	if err := json.Unmarshal(b, &im); err != nil {
		t.Fatal(err)
	}
	unlock, err := lockFile(filepath.Join(dir, im.Key[:8]+".build"), true, false)
	if err != nil {
		t.Fatal(err)
	}
	if out := h.must("", "images", "build", "--if-needed"); !strings.Contains(out, "already being built") {
		t.Errorf("a second build:\n%s", out)
	}
	unlock()
	h.must("", "images", "rm", "--all")

	h.world.spawnErr = errors.New("no exec")
	if out := h.must("", "new", "dev"); !strings.Contains(out, "Couldn't start building an image") {
		t.Errorf("spawn failure:\n%s", out)
	}
}

// spawn starts this binary detached; here, the test binary, told to run no
// tests.
func TestSpawn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := spawn("-test.run=^$"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(imagesLog()); !strings.Contains(string(b), "limabean -test.run=^$") {
		t.Errorf("log = %q", b)
	}
}
