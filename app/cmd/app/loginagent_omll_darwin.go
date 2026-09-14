//go:build darwin

package main

// The prototype below is declared here rather than in app_darwin.h so the fork
// keeps its hands off upstream's header; the symbol itself comes from
// app_darwin.m, which cgo already compiles into this package. It has to be the
// last comment before the import to be the cgo preamble, hence the split.

// void omllUnregisterLoginAgentForUpgrade(void);
import "C"

// unregisterLoginAgentForUpgrade drops the login agent's registration on the way
// out of an upgrade so the incoming build can register it against its own launch
// constraint. LaunchNewApp is the single place both upgrade paths - the tray's
// "Restart to update" through StartUpdate, and DoUpgradeAtStartup - hand over to
// the replaced bundle, so calling it from there covers both.
//
// See omllUnregisterLoginAgentForUpgrade in app_darwin.m for why this cannot be
// done by re-registering within one process.
func unregisterLoginAgentForUpgrade() {
	C.omllUnregisterLoginAgentForUpgrade()
}
