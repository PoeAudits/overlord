package sync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/syncthing"
)

func TestBuildFolderIncludesLocalAndPeerDevices(t *testing.T) {
	folder := BuildFolder("proj", "/srv/proj", "local-id", "peer-id")
	if folder.ID != "proj" || folder.Label != "proj" {
		t.Fatalf("folder metadata mismatch: %+v", folder)
	}
	if folder.Path != "/srv/proj" {
		t.Fatalf("expected path /srv/proj, got %s", folder.Path)
	}
	if folder.Type != "sendreceive" {
		t.Fatalf("expected type sendreceive, got %s", folder.Type)
	}
	if len(folder.Devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(folder.Devices))
	}
	if folder.Devices[0].DeviceID != "local-id" || folder.Devices[1].DeviceID != "peer-id" {
		t.Fatalf("unexpected device order: %+v", folder.Devices)
	}
	if !folder.FSWatcherEnabled || folder.Copiers != 1 || folder.Hashers != 1 || !folder.CaseSensitiveFS {
		t.Fatalf("expected folder tuning flags to be set: %+v", folder)
	}
}

func TestEnsureIgnoreFileCreatesIncludeLine(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	globalIgnore := filepath.Join(home, ".config", "syncthing", ".stglobalignore")
	if err := os.MkdirAll(filepath.Dir(globalIgnore), 0o755); err != nil {
		t.Fatalf("mkdir global ignore: %v", err)
	}
	if err := os.WriteFile(globalIgnore, []byte("# shared ignores\n"), 0o644); err != nil {
		t.Fatalf("write global ignore: %v", err)
	}

	projectDir := filepath.Join(home, "projects", "sample")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	if err := EnsureIgnoreFile(projectDir); err != nil {
		t.Fatalf("ensure ignore failed: %v", err)
	}
	s, err := os.ReadFile(filepath.Join(projectDir, ".stignore"))
	if err != nil {
		t.Fatalf("read .stignore: %v", err)
	}
	rel, err := filepath.Rel(projectDir, globalIgnore)
	if err != nil {
		t.Fatalf("rel path: %v", err)
	}
	expected := "#include " + filepath.ToSlash(rel)
	if got := strings.TrimSpace(string(s)); got != expected {
		t.Fatalf("unexpected .stignore contents: %q (want %q)", got, expected)
	}

	if err := EnsureIgnoreFile(projectDir); err != nil {
		t.Fatalf("second ensure should succeed: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(projectDir, ".stignore"))
	if err != nil {
		t.Fatalf("read .stignore: %v", err)
	}
	if strings.Count(string(content), expected) != 1 {
		t.Fatalf("include line should be unique: %q", content)
	}
}

func TestGlobalIgnorePathUsesHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path, err := GlobalIgnorePath()
	if err != nil {
		t.Fatalf("global path error: %v", err)
	}
	expected := filepath.Join(home, ".config", "syncthing", ".stglobalignore")
	if path != expected {
		t.Fatalf("expected %s, got %s", expected, path)
	}
}

func TestProjectPathForPeerVariants(t *testing.T) {
	absProject := registry.Project{Path: "/srv/projects/app"}
	if path, err := ProjectPathForPeer(absProject, "", ""); err != nil || path != absProject.Path {
		t.Fatalf("absolute paths should pass through, got %q err=%v", path, err)
	}

	relProject := registry.Project{Path: "apps/api"}
	regBaseAbs := filepath.Join("/srv", "projects")
	peerBase := filepath.Join("/mnt", "projects")
	if path, err := ProjectPathForPeer(relProject, regBaseAbs, peerBase); err != nil || path != filepath.Join(peerBase, relProject.Path) {
		t.Fatalf("expected peer base dir to override: path=%q err=%v", path, err)
	}

	regBaseRel := "projects"
	if _, err := ProjectPathForPeer(relProject, regBaseRel, ""); err != nil {
		// expected
	} else {
		t.Fatalf("expected error when both registry and peer base dirs are insufficient for relative project path")
	}

	if _, err := ProjectPathForPeer(registry.Project{}, regBaseAbs, peerBase); err == nil {
		t.Fatalf("empty project path should error")
	}
}

func TestEnsureDeviceUsesPeerMetadata(t *testing.T) {
	selection := PeerSelection{
		Name: "vps",
		Peer: machine.Peer{
			Syncthing: machine.PeerSyncthing{
				DeviceID: "peer-id",
				Address:  "tcp://peer",
			},
		},
	}
	mock := &mockDeviceClient{}
	if err := EnsureDevice(context.Background(), mock, selection); err != nil {
		t.Fatalf("ensure device returned error: %v", err)
	}
	if mock.last.DeviceID != "peer-id" {
		t.Fatalf("expected device id peer-id, got %s", mock.last.DeviceID)
	}
	if mock.last.Name != "vps" {
		t.Fatalf("expected peer name as device name, got %s", mock.last.Name)
	}
	if len(mock.last.Addresses) != 1 || mock.last.Addresses[0] != "tcp://peer" {
		t.Fatalf("unexpected addresses: %+v", mock.last.Addresses)
	}
	if mock.last.Compression != "metadata" {
		t.Fatalf("expected metadata compression, got %s", mock.last.Compression)
	}

	mock.err = context.DeadlineExceeded
	if err := EnsureDevice(context.Background(), mock, selection); err == nil {
		t.Fatalf("expected error propagation from client")
	}
}

type mockDeviceClient struct {
	last syncthing.Device
	err  error
}

func (m *mockDeviceClient) EnsureDevice(_ context.Context, device syncthing.Device) (bool, error) {
	m.last = device
	return true, m.err
}
