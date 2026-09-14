extends SceneTree
var failed = false

func check(ok, label):
	if not ok:
		failed = true
		printerr("FAIL: ", label)

func write_file(path, bytes):
	check(Directory.new().make_dir_recursive(path.get_base_dir()) == OK, "create fixture directory")
	var file = File.new()
	check(file.open(path, File.WRITE) == OK, "create fixture file")
	file.store_buffer(bytes)
	file.close()

func _init():
	var files = load("res://scripts/game_files.gd").new()
	var root = "user://layout-test-" + str(OS.get_ticks_usec())
	var file = File.new()
	file.open("res://required_files.json", File.READ)
	var required = JSON.parse(file.get_as_text()).result
	file.close()
	var digital = root.plus_file("digital/LoCaL")
	var discs = root.plus_file("discs")
	var common = ""
	for disc in [1, 2]:
		for relative in required[str(disc)]:
			write_file(digital.plus_file(relative.get_file().to_upper()), PoolByteArray([42]))
			write_file(discs.plus_file("CD%d" % disc).plus_file(relative.to_upper()), PoolByteArray([disc]))
			if disc == 2 and relative in required["1"]:
				common = relative.get_file()
	check(not common.empty(), "fixture includes shared disc filenames")
	for selected in [digital.get_base_dir(), digital]:
		var roots = files.discover(selected)
		check(roots == [digital, digital], "discover digital install or LOCAL directly")
		check(files.validate(roots[0], roots[1]), files.error)
		check(files.index["1/" + common] == files.index["2/" + common], "digital namespaces share the source file")
		check(files.index["1/bootfile"].begins_with(digital), "digital files read in place")
	var roots = files.discover(discs)
	check(files.validate(roots[0], roots[1]), files.error)
	check(files.index["1/" + common] != files.index["2/" + common], "original discs retain distinct files")
	# Empty or incomplete selections cannot leave a partly usable index.
	var target = digital.plus_file("CARGO.SET")
	Directory.new().remove(target)
	check(not files.validate(digital, digital), "reject missing digital file")
	check(files.index.empty(), "clear index after failed validation")
	write_file(target, PoolByteArray())
	check(not files.validate(digital, digital), "reject empty digital file")
	write_file(target, PoolByteArray([42]))
	write_file(digital.plus_file("bootfile"), PoolByteArray([43]))
	check(files.discover(digital).empty(), "reject ambiguous filename case")
	check(not files.validate(digital, digital), "validation also rejects ambiguous case")
	Directory.new().remove(digital.plus_file("bootfile"))
	check(files.validate(digital, digital), "recheck changed directory after error")
	check(files.discover(root.plus_file("missing")).empty(), "reject unrelated selection")
	var iso_dir = root.plus_file("images")
	var first = iso_dir.plus_file("Titanic_CD1_1996.iso")
	var second = iso_dir.plus_file("my-cd2-copy.ISO")
	write_file(first, PoolByteArray([1]))
	check(files.discover(iso_dir).empty() and not files.error.empty(), "report missing second ISO")
	write_file(second, PoolByteArray([2]))
	check(files.discover(iso_dir) == [first, second], "loose case-insensitive ISO discovery")
	write_file(iso_dir.plus_file("backup-cd1.iso"), PoolByteArray([3]))
	check(files.discover(iso_dir).empty() and not files.error.empty(), "reject ambiguous ISO selection")
	var range_file = root.plus_file("range.bin")
	write_file(range_file, PoolByteArray([9, 8, 7, 6]))
	check(files.read_asset("iso:" + JSON.print({"path": range_file, "offset": 1, "size": 2})) == PoolByteArray([8, 7]), "read only the ISO asset range")
	check(files.read_asset("iso:" + JSON.print({"path": range_file, "offset": 3, "size": 2})) == null, "reject truncated ISO range")
	print("GAME FILES TEST ", "FAIL" if failed else "PASS")
	quit(1 if failed else 0)
