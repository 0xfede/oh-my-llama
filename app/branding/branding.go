//go:build windows || darwin

// Package branding centralizes the product identity of this build so that a
// rebranded fork only has to change one file when rebasing onto a new upstream
// release.
package branding

// Name is the user visible product name.
const Name = "Oh My Llama"

// BundleName is the macOS bundle directory name, without the ".app" extension.
// It also doubles as the name of the bundle executable.
const BundleName = "OhMyLlama"

// BundleID is the macOS CFBundleIdentifier, also used to name the LaunchAgent
// plist and to detect other running instances of this build.
const BundleID = "com.ohmyllama.app"

// CacheDirName is the directory this build uses under the user cache dir
// (~/Library/Caches on macOS) to stage downloaded updates, back up the old
// bundle mid-upgrade, and record the upgrade marker. It must NOT be shared with
// stock Ollama: a staged stock update sitting there would make this build report
// "update available" and then try to install Ollama over itself.
const CacheDirName = "ohmyllama"

// URLScheme is the URL scheme this build registers, without the "://" suffix.
// We keep "ollama" so existing links (notably the ollama.com sign-in callback)
// keep working.
const URLScheme = "ollama"

// CLIName is the name of the CLI symlink installed into /usr/local/bin. We keep
// "ollama" so existing muscle memory and scripts keep working.
const CLIName = "ollama"

// Repo is the GitHub repository this build updates itself from, as "owner/name".
const Repo = "0xfede/oh-my-llama"

// UpdateFeedURL is where the updater looks for new releases. It is a static JSON
// asset attached to the latest GitHub release, of the form
//
//	{"version": "v0.32.4-omll.1", "url": "https://github.com/.../download/v0.32.4-omll.1/OhMyLlama-darwin.zip"}
//
// A static file cannot answer 204 for an up-to-date client the way the official
// endpoint does, so the updater compares versions itself - see isNewer in the
// updater package.
const UpdateFeedURL = "https://github.com/" + Repo + "/releases/latest/download/update.json"
