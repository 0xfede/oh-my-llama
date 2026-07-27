// Product identity for this build, kept in sync with branding.go so that a
// rebranded fork only has to change these two files when rebasing onto a new
// upstream release.
//
// These are NSString literals, so adjacent ones concatenate, e.g.
// @"/Applications/" OML_BUNDLE_NAME @".app".

#define OML_NAME @"Oh My Llama"
#define OML_BUNDLE_NAME @"OhMyLlama"
#define OML_BUNDLE_ID @"com.ohmyllama.app"
// URL scheme this build registers. We keep "ollama" so existing links (notably
// the ollama.com sign-in callback) keep working.
#define OML_URL_SCHEME @"ollama"

// Name of the CLI symlink installed into /usr/local/bin. We keep "ollama" so
// existing muscle memory and scripts keep working.
#define OML_CLI_NAME @"ollama"
