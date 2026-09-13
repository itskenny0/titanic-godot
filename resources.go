// Package resources exposes the same pinned patch manifest used by the frontend.
package resources

import _ "embed"

//go:embed godot/patches/manifest.json
var patchManifest string

func PatchManifest() string { return patchManifest }
