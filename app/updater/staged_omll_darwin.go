package updater

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"

	"github.com/ollama/ollama/app/version"
)

// The staged-update version guard lives here rather than inline in
// updater_darwin.go so that it survives rebases: upstream owns getStagedUpdate,
// DoUpgrade and DoUpgradeAtStartup, and the only fork edit inside them is the
// three-line call to staleStagedUpdate.
//
// Upstream never compares versions when it installs a staged download, and
// nothing on disk records what a staged zip holds - the stage directory is named
// after the download's etag and the file after the installer - so DoUpgrade
// installs whichever zip it finds and IsUpdatePending is really just "is there a
// zip". A leftover download older than the running build therefore downgrades the
// app: 0.32.6 downloaded 0.32.14 and was quit before installing it, 0.33.3 was
// then installed by hand, and the next "Restart to update" replayed the 0.32.14
// zip over it. The same leftover also makes the tray offer an update forever on a
// build that has nothing to update to.

// staleStagedUpdate reports whether a staged update bundle should be thrown away
// rather than installed, and throws it away when so.
func staleStagedUpdate(bundle string) bool {
	staged := stagedUpdateVersion(bundle)
	if staged == "" {
		// A bundle we cannot read a version out of is upstream's to deal with:
		// DoUpgrade fails on it with a specific error, which is more useful than
		// silently deleting a download here. Only a version that can actually be
		// compared is grounds for discarding one.
		return false
	}
	if isNewer(staged, version.Version) {
		return false
	}
	slog.Info("discarding staged update that is not newer than the running build",
		"bundle", bundle, "staged", staged, "running", version.Version)
	// Remove the whole etag directory rather than just the zip: cleanupOldDownloads
	// only runs when a fresh download starts, so an emptied directory would linger.
	// Callers reach this via a glob of UpdateStageDir/*/*.zip, so the parent is
	// always one of those directories.
	if err := os.RemoveAll(filepath.Dir(bundle)); err != nil {
		slog.Warn("failed to remove stale staged update", "bundle", bundle, "error", err)
	}
	return true
}

// stagedUpdateVersion reports the CFBundleShortVersionString of the app bundle
// inside a staged update zip, or "" when it cannot be determined.
func stagedUpdateVersion(bundle string) string {
	r, err := zip.OpenReader(bundle)
	if err != nil {
		slog.Debug("unable to open staged update", "bundle", bundle, "error", err)
		return ""
	}
	defer r.Close()

	// zip.Reader paths are always slash separated, hence path over filepath.
	f, err := r.Open(path.Join(updateArchiveRoot, "Contents", "Info.plist"))
	if err != nil {
		slog.Debug("staged update has no Info.plist", "bundle", bundle, "error", err)
		return ""
	}
	defer f.Close()

	staged := plistString(f, "CFBundleShortVersionString")
	if staged == "" {
		slog.Debug("staged update Info.plist carries no version", "bundle", bundle)
	}
	return staged
}

// plistString returns the <string> value that follows key in an XML property
// list, or "" when there is none.
//
// This is deliberately not a plist parser. The one value needed here is a string
// in the top level dict of a file the build writes itself, and pulling in a plist
// dependency to read it is not worth it. A binary plist yields "", which callers
// treat as "unknown" rather than as grounds for deleting anything.
func plistString(r io.Reader, key string) string {
	dec := xml.NewDecoder(r)
	wantValue := false
	for {
		tok, err := dec.Token()
		if err != nil {
			// Includes io.EOF: the key is simply not there.
			return ""
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "key":
			var name string
			if err := dec.DecodeElement(&name, &start); err != nil {
				return ""
			}
			wantValue = name == key
		case "string":
			var value string
			if err := dec.DecodeElement(&value, &start); err != nil {
				return ""
			}
			if wantValue {
				return value
			}
		default:
			// The key was followed by a value that is not a string.
			wantValue = false
		}
	}
}
