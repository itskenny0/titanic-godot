extends Reference
var root = "user://Saves"
var archives = "user://Save Archives"
var error = ""
var autosave_prune_after = -1

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

func list_saves(include_autosaves = true):
	var names = []
	var entries = directory_entries(root)
	if include_autosaves:
		for entry in checkpoint_entries():
			entry.name = "Autosaves/" + entry.name
			entries.append(entry)
	entries.sort_custom(self, "newest_first")
	for entry in entries:
		names.append(entry.name)
	return names

func directory_entries(folder):
	var entries = []
	var file = File.new()
	var d = Directory.new()
	if d.open(folder) != OK:
		return entries
	d.list_dir_begin(true, true)
	var name = d.get_next()
	while name != "":
		if not d.current_is_dir() and name.to_lower().ends_with(".ti"):
			entries.append({"name": name, "modified": file.get_modified_time(folder.plus_file(name))})
		name = d.get_next()
	d.list_dir_end()
	return entries

func newest_first(a, b):
	if a.modified == b.modified:
		if a.has("sequence") and b.has("sequence"):
			return a.sequence > b.sequence
		return a.name.to_lower() < b.name.to_lower()
	return a.modified > b.modified

func is_autosave(name):
	return name.begins_with("Autosaves/")

func display_name(name):
	if is_autosave(name):
		return "Autosave: " + name.get_file().substr(26).get_basename()
	return "Manual: " + name.get_file().get_basename()

func path_for(name):
	if is_autosave(name):
		return root.plus_file("Autosaves").plus_file(safe_name(name.get_file()))
	return manual_path_for(name)

func manual_path_for(name):
	var clean = safe_name(name)
	for existing in list_saves(false):
		if existing.to_lower() == clean.to_lower():
			clean = existing
	return root.plus_file(clean)

func write(name, bytes):
	return write_file(manual_path_for(name), bytes)

func checkpoint_entries():
	var entries = []
	for entry in directory_entries(root.plus_file("Autosaves")):
		if not entry.name.begins_with("Checkpoint ") or entry.name.length() <= 29 or entry.name.substr(23, 3) != " - ":
			continue
		var sequence = entry.name.substr(11, 12)
		if not sequence.is_valid_integer() or sequence != "%012d" % int(sequence):
			continue
		entry.sequence = int(sequence)
		entries.append(entry)
	entries.sort_custom(self, "checkpoint_oldest")
	return entries

func checkpoint_oldest(a, b):
	return a.sequence < b.sequence

func write_autosave(label, bytes):
	var folder = root.plus_file("Autosaves")
	if Directory.new().make_dir_recursive(folder) != OK:
		error = "Cannot create the autosave folder."
		return false
	var entries = checkpoint_entries()
	var sequence = 1 if entries.empty() else entries.back().sequence + 1
	var name = safe_name("Checkpoint %012d - %s" % [sequence, safe_name(label).get_basename()])
	if not write_file(folder.plus_file(name), bytes, false):
		return false
	# Give slow removable storage a quiet interval after publishing the new file.
	# The player's regular polling handles cleanup without a blocking sleep.
	autosave_prune_after = OS.get_ticks_msec() + 3000
	return true

func service_autosaves(now = -1):
	if now < 0:
		now = OS.get_ticks_msec()
	if autosave_prune_after == -1:
		# Recover extra checkpoints left by a shutdown during the grace period.
		autosave_prune_after = now + 3000 if checkpoint_entries().size() > 25 else 0
	if autosave_prune_after == 0 or now < autosave_prune_after:
		return ""
	autosave_prune_after = 0
	var folder = root.plus_file("Autosaves")
	var entries = checkpoint_entries()
	# Sequence numbers preserve FIFO even if the device clock changes.
	while entries.size() > 25:
		if Directory.new().remove(folder.plus_file(entries[0].name)) != OK:
			return "Checkpoint saved, but an old autosave could not be removed."
		entries.pop_front()
	return ""

func write_file(target, bytes, replace_existing = true):
	error = ""
	# Checkpoints only create new files. They never use the manual-save
	# overwrite/archive path, even if a filename unexpectedly collides.
	if not replace_existing and File.new().file_exists(target):
		error = "An autosave already exists at this path; it has been kept."
		return false
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
		if not replace_existing:
			error = "An autosave already exists at this path; it has been kept."
			d.remove(temp)
			return false
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
