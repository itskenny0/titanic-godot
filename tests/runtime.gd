extends SceneTree
var failed = false
func check(value, label):
	if not value:
		failed = true
		printerr("FAIL: ", label)
func bridge_call(method, args, bytes):
	if method == "read":
		return PoolByteArray([42, 0, 255])
	if method == "echo":
		return bytes
	return "null"
func _init():
	var engine = ClassDB.instance("DreamRuntime") if ClassDB.class_exists("DreamRuntime") else load("res://native/runtime.gdns").new()
	check(engine != null, "native runtime loads")
	if engine == null:
		quit(1)
		return
	check(engine.initialize(self) == "", "runtime initialization")
	check(engine.execute("boot", JSON.print({"index": {}, "testing": true})) == "", "native boot")
	var state = JSON.parse(engine.query("state")).result
	check(state.ready, "native startup finishes")
	check(JSON.parse(engine.query("memory")).result.runtime == "go", "gameplay runtime is Go")
	check(engine.execute("test", JSON.print({"op": "set_global", "name": "answer", "value": 42})) == "", "test state assignment")
	check(JSON.parse(engine.query("test", JSON.print({"op": "get_global", "name": "answer"}))).result == 42, "state query")
	check(not engine.execute("unsupported").empty(), "invalid operation is reported")
	check(engine.execute("command", JSON.print({"action": "pause", "on": true})) == "", "pause command")
	check(JSON.parse(engine.query("state")).result.paused, "a past error does not poison commands")
	check(engine.execute("command", JSON.print({"action": "pause", "on": false})) == "", "resume command")
	check(engine.buffer("frame").empty(), "no stale or uninitialized frame")
	check(engine.buffer("audio", JSON.print({"id": 9999})).empty(), "unknown audio buffer is absent")
	var bytes = PoolByteArray([0, 127, 128, 255])
	var store = load("res://scripts/save_store.gd").new()
	var id = "retanic-test-" + str(OS.get_ticks_usec())
	store.root = "user://test-saves/" + id
	store.archives = "user://test-archives/" + id
	Directory.new().make_dir_recursive(store.root)
	Directory.new().make_dir_recursive(store.archives)
	check(store.write("voyage", bytes), "first save")
	check(store.write("VOYAGE", PoolByteArray([1,2,3])), "case-insensitive replacement")
	check(store.list_saves().size() == 1, "one save after replacement")
	var d = Directory.new()
	d.open(store.archives)
	d.list_dir_begin(true,true)
	var archived = d.get_next()
	check(not archived.empty(), "previous save archived")
	var f = File.new()
	f.open(store.archives.plus_file(archived),File.READ)
	check(f.get_buffer(f.get_len()) == bytes, "archived bytes unchanged")
	f.close()
	check(store.safe_name("../../escape") == "escape.ti", "save names cannot escape directory")
	print("PORTABLE TESTS ", "FAIL" if failed else "PASS")
	engine = null
	quit(1 if failed else 0)
