extends Reference
# Unlocked achievements belong to the player, not to a saved game: loading an
# older save never takes one away. Times are Unix seconds.
var path = "user://achievements.cfg"
var unlocked = {}

func load_store():
	unlocked = {}
	var file = ConfigFile.new()
	if file.load(path) != OK:
		return
	if not file.has_section("unlocked"):
		return
	for id in file.get_section_keys("unlocked"):
		unlocked[id] = int(file.get_value("unlocked", id, 0))

func save_store():
	var file = ConfigFile.new()
	for id in unlocked:
		file.set_value("unlocked", id, unlocked[id])
	return file.save(path) == OK

func ids():
	return unlocked.keys()

func has(id):
	return unlocked.has(id)

func unlock(id, now = -1):
	if unlocked.has(id):
		return false
	unlocked[id] = OS.get_unix_time() if now < 0 else now
	save_store()
	return true

func unlocked_count(catalog):
	var count = 0
	for a in catalog:
		if unlocked.has(a.id):
			count += 1
	return count

# Unlocked first, newest at the top, then the rest in story order. Hidden
# ones stay secret until earned.
func sorted(catalog):
	var done = []
	var open = []
	for a in catalog:
		if unlocked.has(a.id):
			done.append(a)
		else:
			open.append(a)
	done.sort_custom(self, "newest_first")
	return done + open

func newest_first(a, b):
	return unlocked[a.id] > unlocked[b.id]
