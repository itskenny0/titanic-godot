extends Reference
var root = "user://Saves"
var archives = "user://Save Archives"
var error = ""

func _init():
	var d = Directory.new()
	d.make_dir_recursive(root)
	d.make_dir_recursive(archives)

func safe_name(name):
	var clean = name.strip_edges().get_file()
	for ch in ["/", "\\", ":", "*", "?", "\"", "<", ">", "|"]:
		clean = clean.replace(ch, "_")
	if clean.to_lower().ends_with(".ti"):
		clean = clean.substr(0, clean.length() - 3)
	clean = clean.strip_edges().rstrip(".")
	if clean.empty() or clean == "." or clean == "..":
		clean = "Voyage"
	return clean.substr(0, 120) + ".ti"

func list_saves():
	var names = []
	var entries = []
	var file = File.new()
	var d = Directory.new()
	if d.open(root) != OK:
		return names
	d.list_dir_begin(true, true)
	var name = d.get_next()
	while name != "":
		if not d.current_is_dir() and name.to_lower().ends_with(".ti"):
			entries.append({"name": name, "modified": file.get_modified_time(root.plus_file(name))})
		name = d.get_next()
	d.list_dir_end()
	entries.sort_custom(self, "newest_first")
	for entry in entries:
		names.append(entry.name)
	return names

func newest_first(a, b):
	if a.modified == b.modified:
		return a.name.to_lower() < b.name.to_lower()
	return a.modified > b.modified

func path_for(name):
	var clean = safe_name(name)
	for existing in list_saves():
		if existing.to_lower() == clean.to_lower():
			clean = existing
	return root.plus_file(clean)

func write(name, bytes):
	error = ""
	var target = path_for(name)
	var temp = target + ".pending-" + str(OS.get_ticks_usec())
	var f = File.new()
	if f.open(temp, File.WRITE) != OK:
		error = "Cannot create a saved game in " + root
		return false
	f.store_buffer(bytes)
	f.flush()
	var err = f.get_error()
	f.close()
	var d = Directory.new()
	if err != OK:
		error = "Writing the saved game failed."
		d.remove(temp)
		return false
	# Copy and verify the archive before touching the previous save.
	var archive = ""
	if f.file_exists(target):
		archive = archives.plus_file(str(OS.get_unix_time()) + "-" + str(OS.get_ticks_usec()) + "-" + target.get_file())
		if d.copy(target, archive) != OK or f.get_sha256(target) != f.get_sha256(archive):
			error = "Could not archive the previous save; it has been kept."
			d.remove(temp)
			return false
	# Godot Directory.rename replaces atomically on POSIX. Windows may require removal.
	if d.rename(temp, target) != OK:
		if archive.empty() or d.remove(target) != OK or d.rename(temp, target) != OK:
			if not archive.empty() and not f.file_exists(target):
				d.copy(archive, target)
			error = "Could not replace the saved game. Previous data is in Save Archives."
			return false
	return true
