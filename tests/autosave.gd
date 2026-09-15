extends SceneTree
var failed = false

func check(ok, label):
	if not ok:
		failed = true
		printerr("FAIL: ", label)

func read_bytes(path):
	var f = File.new()
	if f.open(path, File.READ) != OK:
		return PoolByteArray()
	var bytes = f.get_buffer(f.get_len())
	f.close()
	return bytes

func _init():
	call_deferred("begin")

func begin():
	var store = load("res://scripts/save_store.gd").new()
	store.root = "user://autosave-test-" + str(OS.get_ticks_usec())
	store.archives = store.root.plus_file("Archives")
	Directory.new().make_dir_recursive(store.root)
	Directory.new().make_dir_recursive(store.archives)
	check(store.write("My manual save", PoolByteArray([99])), "create manual save")
	for i in range(1, 28):
		check(store.write_autosave("room", PoolByteArray([i])), store.error)
	check(store.checkpoint_entries().size() == 27, "keep old checkpoints during storage grace period")
	check(store.service_autosaves(store.autosave_prune_after - 1).empty(), "cleanup waits three seconds")
	check(store.checkpoint_entries().size() == 27, "no early deletion")
	check(store.service_autosaves(store.autosave_prune_after).empty(), "delayed FIFO cleanup")
	var entries = store.checkpoint_entries()
	check(entries.size() == 25, "retain exactly 25 autosaves")
	check(entries[0].sequence == 3 and entries.back().sequence == 27, "evict FIFO even within same timestamp")
	check(read_bytes(store.path_for("Autosaves/" + entries[0].name)) == PoolByteArray([3]), "oldest retained checkpoint loads")
	check(read_bytes(store.path_for("Autosaves/" + entries.back().name)) == PoolByteArray([27]), "newest checkpoint loads")
	check(read_bytes(store.path_for("My manual save")) == PoolByteArray([99]), "manual save untouched")
	check(store.list_saves().size() == 26, "checkpoints appear in the load list")
	check(store.newest_first({"modified": 1, "sequence": 27}, {"modified": 1, "sequence": 26}), "checkpoint order breaks timestamp ties newest first")
	check(store.directory_entries(store.archives).empty(), "FIFO does not accumulate archived autosaves")
	var restored = load("res://scripts/save_store.gd").new()
	restored.root = store.root
	restored.archives = store.archives
	check(restored.write_autosave("another room", PoolByteArray([28])), "continue sequence after restart")
	# Simulate closing before cleanup, then reopening with a fresh store.
	var reopened = load("res://scripts/save_store.gd").new()
	reopened.root = store.root
	check(reopened.service_autosaves(100).empty(), "restart schedules leftover cleanup")
	check(reopened.checkpoint_entries().size() == 26, "restart also gives storage a grace period")
	check(reopened.service_autosaves(3100).empty(), "restart finishes FIFO cleanup")
	entries = restored.checkpoint_entries()
	check(entries[0].sequence == 4 and entries.back().sequence == 28, "FIFO sequence persists without separate state")
	var original_root = store.root
	store.root = store.path_for("My manual save")
	check(not store.write_autosave("blocked", PoolByteArray([29])), "write failure reported")
	store.root = original_root
	check(store.checkpoint_entries().size() == 25, "failed write preserves previous checkpoints")
	var manual_name = entries[0].name
	check(store.write(manual_name, PoolByteArray([81,82])), "manual save may share a checkpoint basename")
	var checkpoint_path = store.path_for("Autosaves/" + manual_name)
	var checkpoint_bytes = read_bytes(checkpoint_path)
	check(store.manual_path_for("Autosaves/" + manual_name) != checkpoint_path, "manual writes cannot resolve into autosave storage")
	check(not store.write_file(checkpoint_path, PoolByteArray([0]), false), "checkpoint creation refuses replacement")
	check(read_bytes(checkpoint_path) == checkpoint_bytes, "collision preserves existing save bytes")
	for i in range(29, 56):
		check(store.write_autosave("../../My manual save", PoolByteArray([i])), "sanitize checkpoint label before adding sequence")
	check(store.service_autosaves(store.autosave_prune_after).empty(), "prune all earlier autosaves")
	check(read_bytes(store.manual_path_for(manual_name)) == PoolByteArray([81,82]), "same-name manual survives autosave rotation")
	check(read_bytes(store.manual_path_for("My manual save")) == PoolByteArray([99]), "path-like labels cannot touch manual saves")
	check(store.directory_entries(store.archives).empty(), "autosaves never archive manual saves")
	check(store.checkpoint_entries().size() == 25, "sanitized labels remain valid checkpoints")
	var player = load("res://Main.tscn").instance()
	get_root().add_child(player)
	player.set_process(false)
	player.saves = store
	player.show_save_list("load")
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	var counts = check_save_buttons(player.modal)
	check(counts[0] == 25 and counts[1] == 2, "load menu visibly labels autosaves and manual saves")
	player.handle_event({"type": "autosaving"})
	var toast = player.checkpoint_toast
	check(toast != null and toast.get_child(1).text == "Checkpoint reached - saving ...", "exact checkpoint toast text")
	check(toast.get_node("Background").color == Color(0, 0, 0, 0.25), "25 percent black toast background")
	check(toast.mouse_filter == Control.MOUSE_FILTER_IGNORE, "toast does not block controls")
	check(toast.rect_position == player.game_origin + Vector2(16,16), "toast stays inside handheld game area")
	var previous = player.checkpoint_toast_serial
	player.handle_event({"type": "autosave_failed", "text": "disk full"})
	player.hide_checkpoint_toast(previous)
	check(player.checkpoint_toast != null, "old timeout cannot dismiss newer failure toast")
	player.hide_checkpoint_toast(player.checkpoint_toast_serial)
	check(player.checkpoint_toast == null, "current toast can expire")
	print("AUTOSAVE TESTS ", "FAIL" if failed else "PASS")
	quit(1 if failed else 0)

func check_save_buttons(node):
	var counts = [0, 0]
	if node is Button and node.has_meta("autosave"):
		var automatic = node.get_meta("autosave")
		check(node.text.begins_with("Autosave: " if automatic else "Manual: "), "save row has explicit type label")
		if automatic:
			check(node.get_color("font_color") == Color("b5d8ef"), "autosave rows use distinct text color")
		counts[0 if automatic else 1] += 1
	for child in node.get_children():
		var nested = check_save_buttons(child)
		counts[0] += nested[0]
		counts[1] += nested[1]
	return counts
