extends SceneTree
const GameFiles = preload("res://scripts/game_files.gd")
var failed = false
func check(ok, message):
	if not ok:
		failed = true
		printerr("FAIL: ", message)
func _init():
	var files = GameFiles.new()
	var root = ""
	for arg in OS.get_cmdline_args():
		if arg.begins_with("--game-data="):
			root = arg.substr(12)
	check(files.validate(root.plus_file("cd1"), root.plus_file("cd2")), files.error)
	var original = files.index.duplicate()
	for arg in OS.get_cmdline_args():
		if arg.begins_with("--mods="):
			files.index = original.duplicate()
			check(files.apply_mods(arg.substr(7)), files.error)
			check(files.index != original, "mod overlay changes file map")
			if arg.ends_with("/mods"):
				check(files.index.has("1/lounge1c.set"), "lounge available on disc 1")
	var f = File.new()
	f.open("res://patches/manifest.json", File.READ)
	var manifest = JSON.parse(f.get_as_text()).result
	f.close()
	check(manifest.groups.size() == 6, "six selectable patch groups")
	var seen = {}
	for group in manifest.groups:
		for name in group.files:
			check(not seen.has(name), "patch groups do not overwrite each other")
			seen[name] = true
			var path = "res://patches/files/" + name
			check(f.get_sha256(path) == manifest.files[name].sha256, "patch bytes match " + name)
	check(seen.size() == 56, "all 56 patch files accounted for")
	print("MOD TESTS ", "FAIL" if failed else "PASS")
	quit(1 if failed else 0)
