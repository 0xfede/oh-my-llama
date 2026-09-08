package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ollama/ollama/app/branding"
	"github.com/ollama/ollama/app/version"
)

func infoPlistPayload(shortVersion string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>` + branding.BundleID + `</string>
	<key>CFBundleShortVersionString</key>
	<string>` + shortVersion + `</string>
	<key>LSUIElement</key>
	<true/>
</dict>
</plist>
`)
}

// stageTestUpdate writes a zip into a fresh etag directory under UpdateStageDir,
// the way a completed download leaves one behind.
func stageTestUpdate(t *testing.T, etag string, files []testPayload) string {
	t.Helper()
	bundle := filepath.Join(UpdateStageDir, etag, "test-darwin.zip")
	if err := os.MkdirAll(filepath.Dir(bundle), 0o755); err != nil {
		t.Fatalf("failed to create stage dir: %s", err)
	}
	if err := zipCreationHelper(bundle, files); err != nil {
		t.Fatalf("failed to create staged bundle: %s", err)
	}
	return bundle
}

func bundleWithVersion(shortVersion string) []testPayload {
	return []testPayload{
		{
			Name: updateArchiveRoot + "/Contents/Info.plist",
			Body: infoPlistPayload(shortVersion),
		},
		{
			Name: updateArchiveRoot + "/Contents/MacOS/" + branding.BundleName,
			Body: []byte("would be the app binary"),
		},
	}
}

func TestStagedUpdateVersion(t *testing.T) {
	UpdateStageDir = filepath.Join(t.TempDir(), "updates")

	if got := stagedUpdateVersion(stageTestUpdate(t, "versioned", bundleWithVersion("0.33.3-omll.1"))); got != "0.33.3-omll.1" {
		t.Fatalf("staged version: got %q want %q", got, "0.33.3-omll.1")
	}

	// No Info.plist at all, which is what the other tests in this package stage.
	noPlist := stageTestUpdate(t, "no-plist", []testPayload{
		{
			Name: updateArchiveRoot + "/Contents/MacOS/" + branding.BundleName,
			Body: []byte("would be the app binary"),
		},
	})
	if got := stagedUpdateVersion(noPlist); got != "" {
		t.Fatalf("bundle without Info.plist: got %q want %q", got, "")
	}

	// A binary plist, which plistString does not attempt to read.
	binaryPlist := stageTestUpdate(t, "binary-plist", []testPayload{
		{
			Name: updateArchiveRoot + "/Contents/Info.plist",
			Body: []byte("bplist00\xd1\x01\x02"),
		},
	})
	if got := stagedUpdateVersion(binaryPlist); got != "" {
		t.Fatalf("bundle with a binary Info.plist: got %q want %q", got, "")
	}

	corrupt := filepath.Join(UpdateStageDir, "corrupt", "test-darwin.zip")
	if err := os.MkdirAll(filepath.Dir(corrupt), 0o755); err != nil {
		t.Fatalf("failed to create stage dir: %s", err)
	}
	if err := os.WriteFile(corrupt, []byte{0x4b, 0x50, 0x40, 0x03}, 0o644); err != nil {
		t.Fatalf("failed to write corrupt zip: %s", err)
	}
	if got := stagedUpdateVersion(corrupt); got != "" {
		t.Fatalf("corrupt bundle: got %q want %q", got, "")
	}
}

func TestGetStagedUpdateDiscardsStaleDownloads(t *testing.T) {
	oldVersion := version.Version
	oldStageDir := UpdateStageDir
	t.Cleanup(func() {
		version.Version = oldVersion
		UpdateStageDir = oldStageDir
	})
	version.Version = "0.33.3-omll.1"

	cases := []struct {
		name    string
		staged  string
		payload []testPayload
		keep    bool
	}{
		{name: "newer upstream release", staged: "0.34.0-omll.1", keep: true},
		{name: "newer fork build", staged: "0.33.3-omll.2", keep: true},
		{name: "same version", staged: "0.33.3-omll.1", keep: false},
		{name: "older fork build", staged: "0.32.14-omll.1", keep: false},
		{name: "older upstream release", staged: "0.32.14-omll.9", keep: false},
		{name: "unparseable version", staged: "not-a-version", keep: false},
		// No version to compare: upstream's behavior of installing whatever is
		// staged has to survive, so DoUpgrade can report what is wrong with it.
		{name: "no version", payload: []testPayload{{Name: updateArchiveRoot + "/Contents/MacOS/x", Body: []byte("x")}}, keep: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			UpdateStageDir = filepath.Join(t.TempDir(), "updates")
			payload := tc.payload
			if payload == nil {
				payload = bundleWithVersion(tc.staged)
			}
			bundle := stageTestUpdate(t, "etag", payload)

			got := getStagedUpdate()
			if tc.keep {
				if got != bundle {
					t.Fatalf("getStagedUpdate: got %q want %q", got, bundle)
				}
				if !IsUpdatePending() {
					t.Fatal("IsUpdatePending should report a usable staged update")
				}
				return
			}
			if got != "" {
				t.Fatalf("getStagedUpdate: got %q want it discarded", got)
			}
			if IsUpdatePending() {
				t.Fatal("IsUpdatePending should not offer a staged update that is not newer")
			}
			// The etag directory goes too, so the tray stops offering it and the
			// download stops occupying disk.
			if _, err := os.Stat(filepath.Dir(bundle)); !os.IsNotExist(err) {
				t.Fatalf("stale staged update was left behind: %v", err)
			}
		})
	}
}

func TestPlistString(t *testing.T) {
	cases := []struct {
		name  string
		plist string
		key   string
		want  string
	}{
		{
			name:  "value found",
			plist: `<plist><dict><key>a</key><string>1</string><key>b</key><string>2</string></dict></plist>`,
			key:   "b",
			want:  "2",
		},
		{
			name:  "missing key",
			plist: `<plist><dict><key>a</key><string>1</string></dict></plist>`,
			key:   "b",
			want:  "",
		},
		{
			name:  "key holds a non string value",
			plist: `<plist><dict><key>a</key><true/><key>b</key><string>2</string></dict></plist>`,
			key:   "a",
			want:  "",
		},
		{
			name:  "not xml",
			plist: "bplist00",
			key:   "a",
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := plistString(strings.NewReader(tc.plist), tc.key); got != tc.want {
				t.Fatalf("plistString: got %q want %q", got, tc.want)
			}
		})
	}
}
