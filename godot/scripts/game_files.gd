extends Reference
# Original discs keep separate paths; digital LOCAL files serve both namespaces.
var error = ""
var index = {}
var iso_indexer = null

# Range descriptors let the existing Godot file bridge seek inside an ISO.
# The Go reader indexes directories once; game assets are read only on demand.
func read_asset(path):
	var offset = 0
	var size = -1
	if path.begins_with("iso:"):
		var parsed = JSON.parse(path.substr(4)).result
		if not parsed is Dictionary or not parsed.get("path", null) is String:
			return null
		path = parsed.path
		offset = int(parsed.get("offset", -1))
		size = int(parsed.get("size", -1))
		if offset < 0 or size <= 0 or size > 512 * 1024 * 1024:
			return null
	var file = File.new()
	if file.open(path, File.READ) != OK:
		return null
	var length = file.get_len()
	if size < 0:
		size = length
	if offset > length or size > length - offset or size > 512 * 1024 * 1024:
		file.close()
		return null
	file.seek(offset)
	var data = file.get_buffer(size)
	file.close()
	return data if data.size() == size else null

func scan(path):
	var result = {}
	var dir = Directory.new()
	if dir.open(path) != OK:
		return result
	dir.list_dir_begin(true, true)
	var name = dir.get_next()
	while name != "":
		var key = name.to_lower()
		if result.has(key):
			error = "Conflicting file names in " + path
			break
		result[key] = name
		name = dir.get_next()
	dir.list_dir_end()
	return result

func resolve_case(root, relative, directories = null):
	var path = root
	for part in relative.split("/"):
		var names
		if directories == null:
			names = scan(path)
		else:
			if not directories.has(path):
				directories[path] = scan(path)
			names = directories[path]
		if not names.has(part.to_lower()):
			return ""
		path = path.plus_file(names[part.to_lower()])
	return path

func validate(disc1, disc2):
	error = ""
	index = {}
	var file = File.new()
	if file.open("res://required_files.json", File.READ) != OK:
		error = "Missing required-file manifest. Rebuild the player."
		return false
	var required = JSON.parse(file.get_as_text()).result
	file.close()
	# Reuse listings only within this validation, so changed folders are checked
	# again next time. Slow SD cards otherwise read each directory for every file.
	var directories = {}
	var digital = disc1 == disc2
	var pending = {}
	for disc in [1, 2]:
		var root = disc1 if disc == 1 else disc2
		if root.get_extension().to_lower() == "iso":
			if iso_indexer == null:
				error = "This build cannot index ISO images."
				return false
			var image = iso_indexer.index_iso(root)
			if not image.get("error", "").empty():
				error = "Disc %d: %s" % [disc, image.error]
				return false
			for relative in required[str(disc)]:
				var entry = image.get("files", {}).get(relative.to_lower(), {})
				if entry.get("size", 0) <= 0:
					error = "Disc %d ISO is missing %s." % [disc, relative]
					return false
				pending[str(disc) + "/" + relative.get_file().to_lower()] = "iso:" + JSON.print({"path": root, "offset": entry.offset, "size": entry.size})
			continue
		for relative in required[str(disc)]:
			var path = resolve_case(root, relative.get_file() if digital else relative, directories)
			if path.empty() or file.open(path, File.READ) != OK:
				error = "Game folder is missing " + relative.get_file() if digital else "Disc %d is missing %s." % [disc, relative]
				return false
			var length = file.get_len()
			file.close()
			if length == 0:
				error = "Empty game file: " + path
				return false
			pending[str(disc) + "/" + relative.get_file().to_lower()] = path
	if not error.empty():
		return false
	index = pending
	return true

func discover(root):
	error = ""
	var names = scan(root)
	if not error.empty():
		return []
	for pair in [["cd1", "cd2"], ["titanic1", "titanic2"]]:
		if names.has(pair[0]) and names.has(pair[1]):
			return [root.plus_file(names[pair[0]]), root.plus_file(names[pair[1]])]
	# Android imports the contents of a selected folder, so recognize a directly
	# selected LOCAL directory by its files rather than its directory name.
	if names.has("local"):
		var local = root.plus_file(names["local"])
		if Directory.new().open(local) == OK:
			return [local, local]
	if names.has("bootfile") and names.has("bedsit1.set"):
		return [root, root]
	var images = ["", ""]
	for name in names:
		if not name.ends_with(".iso") or not File.new().file_exists(root.plus_file(names[name])):
			continue
		var one = "cd1" in name
		var two = "cd2" in name
		if not one and not two:
			continue
		if one and two:
			error = "ISO filename matches both cd1 and cd2: " + names[name]
			return []
		var disc = 0 if one else 1
		if not images[disc].empty():
			error = "Multiple ISOs match cd%d. Keep one image for each disc in the folder." % (disc + 1)
			return []
		images[disc] = root.plus_file(names[name])
	if not images[0].empty() and not images[1].empty():
		return images
	if not images[0].empty() or not images[1].empty():
		error = "Select a folder with both cd1 and cd2 ISO images."
	return []

func mod_files(root, depth = 0):
	var paths = []
	if depth > 5:
		return paths
	var d = Directory.new()
	if d.open(root) != OK:
		return paths
	d.list_dir_begin(true, true)
	var name = d.get_next()
	while name != "":
		var path = root.plus_file(name)
		if d.current_is_dir():
			paths += mod_files(path, depth + 1)
		elif name.get_extension().to_lower() in ["set", "shp", "stg", "cst", "pup", "mov", "trk", "sfx", "11k"] or name.to_lower() == "bootfile":
			paths.append(path)
		name = d.get_next()
	d.list_dir_end()
	paths.sort()
	return paths

func apply_mods(root):
	var replacements = mod_files(root)
	if replacements.empty():
		error = "No extracted game mods found in " + root
		return false
	var seen = {}
	for path in replacements:
		var name = path.get_file().to_lower()
		if seen.has(name):
			error = "Multiple mod versions of " + name + ". Select one mod pack at a time."
			return false
		seen[name] = path
		var f = File.new()
		if f.open(path, File.READ) != OK:
			error = "Cannot read mod " + path
			return false
		var length = f.get_len()
		if length < 1536:
			f.close()
			error = "Incomplete mod file: " + path
			return false
		f.seek(4)
		var declared = f.get_32()
		f.seek(32)
		var magic = f.get_buffer(8).get_string_from_ascii()
		f.close()
		if declared != length or not magic in ["ODTRTRFD", "LPPALPPA"]:
			error = "Invalid DreamFactory mod container: " + path
			return false
		# M3tox replacements target the digital LOCAL layout, so apply to both namespaces.
		for disc in [1, 2]:
			index[str(disc) + "/" + name] = path
	# The lounge-unlock mod explicitly requires LOUNGE1C on both discs.
	if seen.has("lnghall.set"):
		for name in ["lounge1c.set", "lounge.shp", "lounge.trk", "lounge.sfx"]:
			if not index.has("1/" + name) and index.has("2/" + name):
				index["1/" + name] = index["2/" + name]
		if not index.has("1/lounge1c.set"):
			error = "The lounge mod requires LOUNGE1C.SET from your original game files."
			return false
	return true
