extends SceneTree
class Recorder:
	extends Reference
	var commands = []
	var queries = 0
	var surface = {"context": "room", "key": "room", "targets": []}
	var catalog = [
		{"id": "a_story", "title": "Welcome Aboard", "description": "Board the ship.", "category": "story"},
		{"id": "b_secret", "title": "Hidden Thing", "description": "Spoiler text.", "category": "secret", "hidden": true},
		{"id": "c_meta", "title": "Milestone", "description": "Unlock one.", "category": "meta"},
	]
	func query(method):
		queries += 1
		if method == "achievements":
			return JSON.print(catalog)
		return JSON.print(surface)
	func execute(method, args = "{}"):
		if method == "command":
			commands.append(JSON.parse(args).result)
		return ""
var failed = false
func check(ok, message):
	if not ok:
		failed = true
		printerr("FAIL: ", message)
func _init(): call_deferred("begin")

func fits(player, label):
	var bounds = player.modal.get_global_rect()
	check(bounds.position.x >= 0 and bounds.position.y >= 0 and bounds.end.x <= player.layout_size.x and bounds.end.y <= player.layout_size.y, label + " fits viewport")

func find_buttons(node, found):
	if node is Button:
		found.append(node)
	for child in node.get_children():
		find_buttons(child, found)
	return found

func has_text(node, text):
	if node is Label and node.text.find(text) >= 0:
		return true
	for child in node.get_children():
		if has_text(child, text):
			return true
	return false

func button_named(player, prefix):
	for b in find_buttons(player.modal, []):
		if b.text.begins_with(prefix):
			return b
	return null

func begin():
	var store = load("res://scripts/save_store.gd").new()
	store.root = "user://modern-test-" + str(OS.get_ticks_usec())
	store.archives = store.root.plus_file("Archives")
	Directory.new().make_dir_recursive(store.root)
	Directory.new().make_dir_recursive(store.archives)
	var frame = Image.new()
	frame.create(512, 384, false, Image.FORMAT_RGBA8)
	frame.fill(Color(0.2, 0.4, 0.6, 1))

	# Pictures are separate files and never alter or hide a save.
	check(store.write("deck 2026-09-12-23-49-04", PoolByteArray([1, 2, 3])), "create manual save")
	check(store.write_thumbnail(store.last_written, frame), "picture written beside the save")
	check(store.list_saves().size() == 1, "pictures are not listed as saves")
	var picture = store.load_thumbnail("deck 2026-09-12-23-49-04.ti")
	check(picture != null and picture.get_width() == 160 and picture.get_height() == 120, "picture loads at list size")
	check(store.load_thumbnail("missing.ti") == null, "a save without a picture still lists")
	check(not store.write_thumbnail(store.last_written, Image.new()), "no picture before the first frame")
	check(store.list_name("deck 2026-09-12-23-49-04.ti") == "Manual: deck", "default timestamp leaves the list name")
	check(store.list_name("My voyage.ti") == "Manual: My voyage", "chosen names stay whole")
	check(store.list_name("2026-09-12-23-49-04.ti").begins_with("Manual: 2026"), "a bare timestamp stays a name")
	check(store.newest() == "deck 2026-09-12-23-49-04.ti", "newest save found")
	var noon = 1700000000 - 1700000000 % 86400 + 43200 - OS.get_time_zone_info().bias * 60
	check(store.describe_time(noon, noon + 3600) == "Today 12:00", "today: " + store.describe_time(noon, noon + 3600))
	check(store.describe_time(noon, noon + 86400) == "Yesterday 12:00", "yesterday")
	check(store.describe_time(noon, noon + 86400 * 9).ends_with(" 12:00") and store.describe_time(noon, noon + 86400 * 9).length() == 16, "older saves show the date")
	check(store.describe_time(0) == "", "unknown time stays blank")

	# Checkpoint rotation takes each picture with its save.
	for i in range(27):
		check(store.write_autosave("Quick save room", PoolByteArray([i])), store.error)
		store.write_thumbnail(store.last_written, frame)
	check(store.service_autosaves(store.autosave_prune_after).empty(), "rotation runs")
	var pictures = 0
	var folder = Directory.new()
	folder.open(store.root.plus_file("Autosaves"))
	folder.list_dir_begin(true, true)
	var entry = folder.get_next()
	while entry != "":
		if entry.ends_with(".png"):
			pictures += 1
		entry = folder.get_next()
	folder.list_dir_end()
	check(store.checkpoint_entries().size() == 25 and pictures == 25, "rotated checkpoints take their pictures along")
	check(store.display_name(store.list_saves()[1]).begins_with("Autosave: Quick save room") or store.display_name(store.list_saves()[0]).begins_with("Autosave: Quick save room"), "quick saves are labelled in the list")

	var player = load("res://Main.tscn").instance()
	get_root().add_child(player)
	player.set_process(false)
	yield(create_timer(0.3), "timeout")
	player.close_modal()
	player.saves = store
	var recorder = Recorder.new()
	player.runtime = recorder
	player.ready = true

	player.show_menu()
	check(button_named(player, "Quick save") != null and button_named(player, "Settings") != null, "voyage menu offers quick save and settings")
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	fits(player, "voyage menu")
	button_named(player, "Quick save").emit_signal("pressed")
	check(recorder.commands.back().action == "quicksave" and player.modal == null, "quick save needs no dialog")
	player.handle_event({"type": "saved", "name": "Quick save room", "quick": true})
	check(player.checkpoint_toast != null and player.checkpoint_toast.get_child(1).text == "Quick save stored", "quick save is confirmed on screen")
	player.hide_checkpoint_toast(player.checkpoint_toast_serial)

	player.show_menu()
	player.show_settings()
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	fits(player, "settings")
	check(button_named(player, "Picture / sound") != null and button_named(player, "Controls / display") != null, "settings gathers the option pages")

	var saved = {}
	for pair in [["audio", "master"], ["display", "frame_rate"], ["graphics", "smooth_picture"], ["accessibility", "hotspots"]]:
		saved[pair] = player.config.get_value(pair[0], pair[1]) if player.config.has_section_key(pair[0], pair[1]) else null
	var rate = Engine.target_fps
	var bus_volume = AudioServer.get_bus_volume_db(0)
	var bus_mute = AudioServer.is_bus_mute(0)
	player.config.set_value("audio", "master", 1.0)
	player.config.set_value("accessibility", "hotspots", "hold")
	player.show_display_options()
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	fits(player, "picture and sound")
	for _i in range(3):
		button_named(player, "Volume -").emit_signal("pressed")
	check(abs(player.master_volume() - 0.7) < 0.001 and abs(AudioServer.get_bus_volume_db(0) - linear2db(0.7)) < 0.01, "volume steps reach the mixer")
	check(player.modal.find_node("Volume", true, false).text.strip_edges() == "70%", "volume level shown")
	check(player.get_focus_owner() == button_named(player, "Volume -"), "the pressed option keeps focus")
	for _i in range(9):
		button_named(player, "Volume -").emit_signal("pressed")
	check(player.master_volume() == 0.0 and AudioServer.is_bus_mute(0), "volume stops at silence")
	button_named(player, "Volume +").emit_signal("pressed")
	check(not AudioServer.is_bus_mute(0), "raising the volume unmutes")
	Engine.target_fps = 60
	player.show_display_options()
	button_named(player, "Frame rate").emit_signal("pressed")
	check(Engine.target_fps == 30 and button_named(player, "Frame rate").text.find("battery") >= 0, "battery saver halves the frame rate")
	button_named(player, "Frame rate").emit_signal("pressed")
	check(Engine.target_fps == 60, "frame rate returns to 60")
	button_named(player, "Picture:").emit_signal("pressed")
	check(player.config.get_value("graphics", "smooth_picture", false) and button_named(player, "Picture: Smooth") != null, "picture smoothing can be chosen")
	button_named(player, "Picture:").emit_signal("pressed")
	check(not player.config.get_value("graphics", "smooth_picture", true), "sharp pixels restored")

	# Hotspot hints mark what the room offers, never the toolbar.
	player.close_modal()
	player.menu = null
	recorder.surface = {"context": "room", "key": "room", "targets": [
		{"id": "prop:bag", "label": "bag", "x": 164, "y": 282, "w": 70, "h": 86, "aim_x": 199, "aim_y": 325},
		{"id": "actor:penny", "label": "penny", "x": 200, "y": 100, "w": 40, "h": 90, "aim_x": 220, "aim_y": 140},
		{"id": "painting:door", "label": "door", "x": 300, "y": 60, "w": 60, "h": 150, "aim_x": 330, "aim_y": 130}]}
	player.process_hotspots(0.3)
	check(player.hotspots.empty(), "no marks until asked")
	var click = InputEventMouseButton.new()
	click.button_index = BUTTON_RIGHT
	click.pressed = true
	click.position = player.game_origin + Vector2(100, 100)
	player._unhandled_input(click)
	check(recorder.commands.back().action != "pointer", "the second mouse button never clicks in the game")
	player.process_hotspots(0.3)
	check(player.hotspots.size() == 2 and player.hotspots[0].id == "actor:penny", "held button marks characters and objects")
	var asked = recorder.queries
	player.process_hotspots(0.01)
	check(recorder.queries == asked, "marks are gathered a few times a second, not every frame")
	recorder.surface = {"context": "dialogue", "key": "talk", "targets": [{"id": "choice:0:1", "label": "Yes.", "x": 0, "y": 264, "w": 512, "h": 24, "aim_x": 256, "aim_y": 276}]}
	player.process_hotspots(0.3)
	check(player.hotspots.empty(), "conversation replies are not marked")
	recorder.surface.context = "room"
	recorder.surface.targets[0].id = "prop:notebook"
	player.process_hotspots(0.3)
	check(player.hotspots.size() == 1, "marks follow the room")
	player.show_menu()
	player.process_hotspots(0.3)
	check(player.hotspots.empty() and not player.hotspot_mouse_held, "menus clear the marks")
	player.resume_game()
	player.config.set_value("accessibility", "hotspots", "always")
	player.process_hotspots(0.6)
	check(player.hotspots.size() == 1, "always-on marks need no button")
	player.config.set_value("accessibility", "hotspots", "off")
	player.hotspot_mouse_held = true
	player.process_hotspots(0.6)
	check(player.hotspots.empty(), "hints can be switched off")
	player.hotspot_mouse_held = false
	check("hints" in player.default_controller_bindings(), "controllers have a hotspot button")
	player.config.set_value("accessibility", "hotspots", "hold")
	var home = InputEventKey.new()
	home.scancode = KEY_HOME
	home.pressed = true
	player._input(home)
	player.process_hotspots(0.3)
	check(player.hotspots.size() == 1, "holding Home marks the room on a desktop keyboard")
	home.pressed = false
	player._input(home)
	player.process_hotspots(0.3)
	check(player.hotspots.empty(), "releasing Home clears the marks")

	# An unchanged surface is not parsed or applied again.
	recorder.surface = {"context": "panel", "key": "bag", "targets": [{"id": "button:a", "label": "a", "x": 10, "y": 10, "w": 20, "h": 20, "aim_x": 20, "aim_y": 20}]}
	player.refresh_controller_surface()
	check(player.controller_surface.key == "bag", "controller surface follows the engine")
	player.controller_surface = {"context": "busy", "key": "", "targets": []}
	player.refresh_controller_surface()
	check(player.controller_surface.key == "", "an identical answer is skipped")
	player.set_controller_surface({"context": "room", "key": "elsewhere", "targets": []})
	player.refresh_controller_surface()
	check(player.controller_surface.key == "bag", "a surface set elsewhere is replaced by the engine's")

	# The load list shows a picture and a time for each save.
	player.has_frame = true
	player.frame_image = frame
	check(JSON.parse(player.bridge_call("write", JSON.print({"path": "save:Pictured"}), PoolByteArray([7]))).result.has("ok"), "save through the bridge")
	check(File.new().file_exists(store.thumbnail_path(store.manual_path_for("Pictured"))), "saving keeps a picture of the moment")
	player.importing_save = true
	check(JSON.parse(player.bridge_call("write", JSON.print({"path": "save:Pictured"}), PoolByteArray([8]))).result.has("ok"), "import over an existing name")
	check(not File.new().file_exists(store.thumbnail_path(store.manual_path_for("Pictured"))), "an imported save never shows another save's picture")
	player.importing_save = false
	player.has_frame = false
	player.show_save_list("load")
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	fits(player, "save list")
	var rows = 0
	var pictured = 0
	for b in find_buttons(player.modal, []):
		if b.has_meta("autosave"):
			rows += 1
			check(b.has_node("When") and b.get_node("When").text.strip_edges().begins_with("Today"), "each save shows when it was made")
			if b.has_meta("thumbnail"):
				pictured += 1
				check(b.icon != null, "pictured rows carry their image")
	check(rows == 27 and pictured == 26, "rows: %d pictured: %d" % [rows, pictured])
	player.close_modal()
	player.current_dialog = -1

	# Loading the latest save always asks first.
	recorder.commands.clear()
	player.quick_load()
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	fits(player, "quick load")
	check(recorder.commands.back().action == "pause" and recorder.commands.back().on, "the game waits while asked")
	button_named(player, "Cancel").emit_signal("pressed")
	check(player.modal == null and recorder.commands.back().action == "pause", "cancel returns to the game")
	player.quick_load()
	button_named(player, "Load").emit_signal("pressed")
	var last = recorder.commands.back()
	check(last.action == "load" and last.path == store.path_for(store.newest()), "confirming loads the newest save")
	player.close_modal()

	# Achievements persist outside saved games, announce once and keep secrets.
	var trophies = load("res://scripts/achievement_store.gd").new()
	trophies.path = store.root.plus_file("achievements.cfg")
	player.achievements = trophies
	player.achievement_catalog = []
	check(trophies.unlock("a_story", 1700000000), "first unlock is new")
	check(not trophies.unlock("a_story"), "an achievement unlocks once")
	var reread = load("res://scripts/achievement_store.gd").new()
	reread.path = trophies.path
	reread.load_store()
	check(reread.has("a_story") and reread.unlocked["a_story"] == 1700000000, "unlocks survive a restart")
	player.handle_event({"type": "achievement", "id": "c_meta", "title": "Milestone", "description": "Unlock one."})
	player.handle_event({"type": "achievement", "id": "c_meta", "title": "Milestone", "description": "Unlock one."})
	yield(self, "idle_frame")
	check(is_instance_valid(player.achievement_toast) and player.achievement_toast.get_global_rect().end.y <= player.layout_size.y, "unlocking shows a toast on screen")
	check(player.achievement_queue.empty() and trophies.has("c_meta"), "a repeated announcement is not queued twice")
	player.finish_achievement_toast(player.achievement_toast)
	check(player.achievement_toast == null, "the toast goes once shown")
	player.show_achievements()
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	fits(player, "achievements")
	check(has_text(player.modal, "2 / 3"), "progress shows in the title")
	var achievement_rows = []
	for b in find_buttons(player.modal, []):
		if b.has_meta("achievement"):
			achievement_rows.append(b)
	check(achievement_rows.size() == 3, "every achievement is listed")
	check(achievement_rows[0].get_meta("achievement") == "c_meta" and achievement_rows[1].get_meta("achievement") == "a_story", "newest unlocks come first")
	var secret = achievement_rows[2]
	check(secret.get_node("Title").text == "Secret achievement" and secret.get_node("About").text.find("Spoiler") < 0, "hidden achievements keep their secret")
	check(not secret.has_node("When") and achievement_rows[0].has_node("When"), "only unlocked ones show a time")
	check(player.get_focus_owner() == achievement_rows[0], "the list takes controller focus")
	player.close_modal()

	# Players choose on first start and can switch achievements off later.
	for pair in [["achievements", "enabled"], ["achievements", "chosen"]]:
		if not saved.has(pair):
			saved[pair] = player.config.get_value(pair[0], pair[1]) if player.config.has_section_key(pair[0], pair[1]) else null
	if player.config.has_section("achievements"):
		player.config.erase_section("achievements")
	check(player.needs_achievement_choice() and player.achievements_enabled(), "first start asks, defaulting to on")
	player.show_achievement_choice()
	yield(self, "idle_frame")
	fits(player, "achievement choice")
	check(player.get_focus_owner() == button_named(player, "Play with achievements"), "on is the focused default")
	recorder.commands.clear()
	button_named(player, "Play without achievements").emit_signal("pressed")
	check(not player.achievements_enabled() and not player.needs_achievement_choice(), "choice is remembered")
	check(recorder.commands.back().action == "achievements" and not recorder.commands.back().on, "the running game stops tracking")
	check(button_named(player, "Achievements: Off") != null, "settings show the choice")
	player.handle_event({"type": "achievement", "id": "b_secret", "title": "Hidden Thing", "description": "Spoiler text."})
	check(not trophies.has("b_secret") and not is_instance_valid(player.achievement_toast), "nothing unlocks while off")
	player.close_modal()
	player.show_menu()
	check(button_named(player, "Achievements") == null, "the menu hides achievements while off")
	player.close_modal()
	player.show_achievement_choice()
	button_named(player, "Play with achievements").emit_signal("pressed")
	check(player.achievements_enabled() and recorder.commands.back().on, "achievements turn back on")
	player.close_modal()

	for pair in saved:
		if saved[pair] == null:
			if player.config.has_section_key(pair[0], pair[1]):
				player.config.erase_section_key(pair[0], pair[1])
		else:
			player.config.set_value(pair[0], pair[1], saved[pair])
	player.config.save("user://settings.cfg")
	Engine.target_fps = rate
	AudioServer.set_bus_volume_db(0, bus_volume)
	AudioServer.set_bus_mute(0, bus_mute)
	print("MODERN TESTS ", "FAIL" if failed else "PASS")
	quit(1 if failed else 0)
