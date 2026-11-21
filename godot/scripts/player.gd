extends Control

const GameFiles = preload("res://scripts/game_files.gd")
const UIStyle = preload("res://scripts/ui_style.gd")
const AdaptiveLayout = preload("res://scripts/adaptive_layout.gd")
const SaveStore = preload("res://scripts/save_store.gd")
const AchievementStore = preload("res://scripts/achievement_store.gd")
const Scratchpad = preload("res://scripts/scratchpad.gd")
var runtime
var files = GameFiles.new()
var saves = SaveStore.new()
var config = ConfigFile.new()
var game_index = {}
var hd_pack_path = ""
var artwork_choice_dialog = null
var artwork_choice_first = false
var artwork_choice_save = ""
var achievement_choice_dialog = null
var achievement_choice_first = false
var achievement_choice_save = ""
var helper_choice_save = ""
var helper_choice_dialog = null
var scratchpad_options_from = "settings"
var scratchpad_options_dialog = null
var adaptive = AdaptiveLayout.new()
var adaptive_panel_style = UIStyle.plate(Color("1b202140"),Color("645b4660"),6)
var adaptive_button_style = UIStyle.plate(Color("24292960"),Color("4f504580"),4)
var adaptive_hover_style = UIStyle.plate(Color("35372f90"),Color("9b875dc0"),4)
var touch_surround_style = UIStyle.plate(Color(0,0,0),Color(0,0,0),0)
var adaptive_active = false
var adaptive_available = false
var prefer_classic_now = false
# The engine marks short moments (a room loading, the bag animating, an
# item in hand) as "hold": the side panels give way to the classic layout
# only if one lasts adaptive_hold_ms, so short sequences stay widescreen.
# Full-screen movies, conversations and menus still switch at once.
var adaptive_hold_ms = 1500
var adaptive_leaving_since = -1
var layout_switch
var adaptive_metadata = {}
var adaptive_atlas_image = Image.new()
var adaptive_atlas_texture = ImageTexture.new()
var adaptive_atlas_revision = -1
var adaptive_atlas_scale = 1
var device_format_checked = -1
var device_format_classic = false
var adaptive_text = ""
var adaptive_parsed = null
var adaptive_configured = []
var display_pointer = Vector2(256,192)
var drawn_display_pointer = Vector2(-1000,-1000)
var game_origin = Vector2.ZERO
var layout_size = Vector2(512, 384)
var frame_image = Image.new()
var frame_texture = ImageTexture.new()
var has_frame = false
var overlays = []
var fonts = {}
var sounds = {}
var volumes = {"sound": 1.0, "voice": 1.0, "theme": 0.6}
var audio_paused = false
var pointer = Vector2(256, 192)
var pointer_name = "arrow"
var pointer_pressed = false
var pointer_visible = true
var controller_pointer_event = false
var cursor_layer
var drawn_pointer = Vector2(-1000, -1000)
var drawn_pointer_name = ""
var modal
var menu
var status
var checkpoint_toast = null
var achievements = AchievementStore.new()
var scratchpad = null
var item_queue = []
var item_notice = null
var achievement_catalog = []
var achievement_queue = []
var achievement_toast = null
var checkpoint_toast_serial = 0
var current_dialog = -1
var queued_dialogs = []
var focused = true
var app_suspended = false
var resume_frame = false
var last_process_ms = 0
var controller_neutral_required = false
var manual_pause = false
var close_first_ms = -1
var close_last_ms = -1
var close_count = 0
var close_menu_pending = false
var dialogue = preload("res://scripts/dialogue_layout.gd").new()
var dialogue_active = false
# A new set of replies starts on the first one, selected but not drawn, so
# the keys never continue from where the previous answers were.
var dialogue_preselected = false
var dialogue_request = null
var dialogue_metadata = {}
var dialogue_choice_dialog = null
var dialogue_choice_first = false
var dialogue_choice_save = ""
var dialogue_preview = null
var close_request_dialog = null
var close_dialog_pause = false
var ready = false
var runtime_failed = false
var nav_timer = 0.0
var last_nav = ""
var controller_surface = {"context": "busy", "key": "", "targets": []}
var controller_selection = -1
var controller_poll = 0.0
var controller_triggers = [false, false]
var controller_bindings = {}
var controller_held = {}
var remap_action = ""
var remap_release = ""
var remap_buttons = {}
var remap_status
var remap_open = false
var remap_deadline = 0
const CONTROLLER_ACTIONS = [
	["confirm", "Confirm / click"], ["back", "Back / skip"], ["menu", "Voyage menu"],
	["door", "Door / Space"], ["keyboard", "Keyboard"], ["mouse", "Pointer click"],
	["previous", "Previous target"], ["next", "Next target"], ["precision", "Slow pointer"], ["hints", "Show hotspots"],
	["up", "Up"], ["down", "Down"], ["left", "Left"], ["right", "Right"]]
# Hand-drawn stand-ins for the original's cursor set, relative to the hot spot:
# a pointing hand over anything that responds, an open hand and fist for
# things to pick up and carry, direction arrows, a four-way sight and an
# hourglass while the game is not listening.
const POINTER_ARROW = [Vector2(0,0), Vector2(0,15), Vector2(4,11.5), Vector2(7.5,18), Vector2(10.5,16.5), Vector2(7,10), Vector2(12.5,10)]
const POINTER_UP = [Vector2(0,-9), Vector2(8,-1), Vector2(3.5,-1), Vector2(3.5,8), Vector2(-3.5,8), Vector2(-3.5,-1), Vector2(-8,-1)]
const POINTER_ARM = [Vector2(-1.5,-1.5), Vector2(-1.5,-5.5), Vector2(-4,-5.5), Vector2(0,-10), Vector2(4,-5.5), Vector2(1.5,-5.5), Vector2(1.5,-1.5)]
const POINTER_SHAPES = {
	"touch": [
		Vector2(-2,1), Vector2(-1,0), Vector2(1,0), Vector2(2,1), Vector2(2,8), Vector2(4,7), Vector2(5.5,8),
		Vector2(7,8), Vector2(8,9.5), Vector2(9.5,9.5), Vector2(10.5,11), Vector2(10.5,16), Vector2(8.5,20),
		Vector2(-1,20), Vector2(-3,17), Vector2(-6.5,13), Vector2(-7,11), Vector2(-5.5,10.5), Vector2(-2,12.5)],
	"hand": [
		Vector2(-5,2), Vector2(-5,-7), Vector2(-3.8,-8), Vector2(-2.5,-7), Vector2(-2.5,-9), Vector2(-1.2,-10),
		Vector2(0,-9), Vector2(0.2,-9.5), Vector2(1.5,-10.5), Vector2(2.8,-9.5), Vector2(3,-8), Vector2(4.4,-8.8),
		Vector2(5.6,-7.8), Vector2(5.6,4), Vector2(3.5,9), Vector2(-3,9), Vector2(-5,6), Vector2(-8.5,2.5),
		Vector2(-9,0.5), Vector2(-7.5,0)],
	"fist": [
		Vector2(-5.5,-4), Vector2(-4,-6), Vector2(4.5,-6), Vector2(6.5,-4), Vector2(6.5,3), Vector2(4,7.5),
		Vector2(-3,7.5), Vector2(-6,4.5), Vector2(-8.5,1.5), Vector2(-8.5,-1), Vector2(-6.5,-1.5)],
	"watch": [
		Vector2(-6,-9), Vector2(6,-9), Vector2(6,-7), Vector2(1.2,0), Vector2(6,7), Vector2(6,9), Vector2(-6,9),
		Vector2(-6,7), Vector2(-1.2,0), Vector2(-6,-7)]}
const POINTER_DETAILS = {
	"touch": [[Vector2(2,11), Vector2(2,14)], [Vector2(5,11), Vector2(5,14)], [Vector2(7.5,11), Vector2(7.5,14)]],
	"hand": [[Vector2(-2.5,-6), Vector2(-2.5,-1)], [Vector2(0.2,-8), Vector2(0.2,-1)], [Vector2(3,-7), Vector2(3,-1)]],
	"fist": [[Vector2(-2,-6), Vector2(-2,-2)], [Vector2(1,-6), Vector2(1,-2)], [Vector2(4,-6), Vector2(4,-2)]],
	"watch": [[Vector2(-7,-9), Vector2(7,-9)], [Vector2(-7,9), Vector2(7,9)], [Vector2(-3,6.5), Vector2(3,6.5)], [Vector2(-1.5,5), Vector2(1.5,5)]]}
var smoke = false
var smoke_time = 0.0
var pending_restart = null
var active_mods = ""
var virtual_keyboard = null
var keyboard_target
var saved_dialog
var saved_keyboard_focus
var save_name
var patch_manifest = {}
var patch_boxes = {}
var patch_runtime = null
var patch_busy = false
var patch_status_label = null
var patch_poll_timer = 0.0
var patch_buttons = []
var patch_import_copy = ""

var android_selected_disc = 0
var backdrop = null
var importing_save = false
var hotspots = []
var hotspot_text = ""
var hotspot_poll = 0.0
var hotspot_phase = 0.0
var hotspot_mouse_held = false
var surface_cache = ""
var dialogue_text = ""
var dialogue_parsed = {}
const HOTSPOT_TOOLBAR = ["prop:life", "prop:watch", "prop:bag", "prop:map", "prop:navarrow", "prop:light"]
var perf_elapsed = 0.0
var perf_frames = 0
var perf_tick_us = 0
var perf_present_us = 0
var perf_events_us = 0
var perf_peak_us = 0

func _ready():
	theme = UIStyle.create_theme()
	files.iso_indexer = self
	Engine.target_fps = 30 if not OS.get_environment("RETANIC_ARCH").empty() else 60
	var unattended = false
	for arg in OS.get_cmdline_args():
		if arg in ["--integration-test", "--smoke-test", "--ui-test"]:
			unattended = true
	OS.low_processor_usage_mode = true
	get_tree().set_auto_accept_quit(false)
	get_tree().set_quit_on_go_back(false)
	get_tree().connect("files_dropped", self, "files_dropped")
	config.load("user://settings.cfg")
	achievements.load_store()
	if not unattended:
		apply_display_settings()
	load_controller_bindings()
	Input.set_mouse_mode(Input.MOUSE_MODE_HIDDEN)
	pointer_visible = Input.get_connected_joypads().empty() and OS.get_environment("RETANIC_ARCH").empty()
	cursor_layer = Node2D.new()
	cursor_layer.z_index = 100
	add_child(cursor_layer)
	cursor_layer.connect("draw", self, "draw_cursor")
	status = Label.new()
	status.rect_position = Vector2(20, 160)
	status.rect_size = Vector2(472, 70)
	status.autowrap = true
	status.align = Label.ALIGN_CENTER
	status.add_font_override("font", get_font_for("14px Arial"))
	status.text = "Preparing your voyage…"
	add_child(status)
	if Engine.has_singleton("TitanicFiles"):
		Engine.get_singleton("TitanicFiles").connect("import_finished", self, "android_import_finished")
		Engine.get_singleton("TitanicFiles").connect("hd_folder_selected", self, "android_hd_selected")
		Engine.get_singleton("TitanicFiles").connect("hd_detected", self, "android_hd_detected")
		Engine.get_singleton("TitanicFiles").connect("mod_import_finished", self, "android_mod_finished")
		Engine.get_singleton("TitanicFiles").connect("patch_import_finished", self, "android_patch_finished")
		Engine.get_singleton("TitanicFiles").connect("save_import_finished", self, "android_save_finished")
		Engine.get_singleton("TitanicFiles").connect("export_finished", self, "android_export_finished")
	make_touch_controls()
	layout_switch = Button.new()
	layout_switch.add_font_override("font",get_font_for("11px Arial"))
	layout_switch.add_stylebox_override("normal",adaptive_button_style)
	layout_switch.add_stylebox_override("hover",adaptive_hover_style)
	layout_switch.add_stylebox_override("pressed",adaptive_hover_style)
	layout_switch.connect("pressed",self,"toggle_exploration_layout")
	layout_switch.focus_mode = Control.FOCUS_NONE
	layout_switch.hide()
	add_child(layout_switch)
	scratchpad = Scratchpad.new()
	scratchpad.name = "Scratchpad"
	scratchpad.host = self
	scratchpad.rect_size = layout_size
	add_child(scratchpad)
	scratchpad.load_store()
	scratchpad.refresh()
	# Menus sit on a dimmed picture, so their text never competes with it.
	backdrop = ColorRect.new()
	backdrop.color = Color(0, 0, 0, 0.5)
	backdrop.mouse_filter = Control.MOUSE_FILTER_IGNORE
	backdrop.hide()
	add_child(backdrop)
	var manifest_file = File.new()
	if manifest_file.open("res://patches/manifest.json", File.READ) == OK:
		patch_manifest = JSON.parse(manifest_file.get_as_text()).result
		manifest_file.close()
	var automated = false
	for arg in OS.get_cmdline_args():
		if arg in ["--integration-test", "--smoke-test", "--ui-test"]:
			automated = true
	if not patch_manifest.empty() and (config.get_value("patches", "seen", "") != patch_manifest.version or (not config.get_value("patches", "enabled", []).empty() and not patches_available())) and not automated:
		show_patch_picker(true)
	else:
		boot_configured_game()

func boot_configured_game():
	var automated = false
	for arg in OS.get_cmdline_args():
		if arg in ["--integration-test", "--smoke-test", "--ui-test"]:
			automated = true
	if not automated and not config.get_value("graphics","layout_chosen",false):
		if classic_format_device():
			config.set_value("graphics","layout_chosen",true)
			config.set_value("graphics","adaptive_exploration",false)
			config.save("user://settings.cfg")
		else:
			show_first_layout()
			return
	var root = config.get_value("game", "root", "")
	var startup_save = ""
	active_mods = config.get_value("game", "mods", "")
	for arg in OS.get_cmdline_args():
		if arg.begins_with("--game-data="):
			root = arg.substr(12)
		elif arg.begins_with("--patches="):
			var selected = []
			for group in patch_manifest.get("groups", []):
				if arg == "--patches=all" or group.id in arg.substr(10).split(","):
					selected.append(group.id)
			config.set_value("patches", "enabled", selected)
		elif arg.begins_with("--mods="):
			active_mods = arg.substr(7)
		elif arg.begins_with("--save="):
			startup_save = arg.substr(7)
		elif arg.to_lower().ends_with(".ti") and File.new().file_exists(arg):
			startup_save = arg
		elif arg == "--smoke-test":
			smoke = true
	if root.empty():
		var base = OS.get_executable_path().get_base_dir()
		var candidates = ["user://gamedata", base.plus_file("gamedata"), base, ProjectSettings.globalize_path("res://../gamedata")]
		if OS.get_name() in ["OSX", "macOS"]:
			candidates.append(base.get_base_dir().get_base_dir().get_base_dir())
		for candidate in candidates:
			if not files.discover(candidate).empty():
				root = candidate
				break
	var roots = files.discover(root) if not root.empty() else []
	# FRT runs from a mounted runtime directory, not beside the game pack.
	var port_dir = OS.get_environment("RETANIC_GAME_DIR")
	if roots.empty() and files.error.empty() and not port_dir.empty():
		for candidate in [port_dir, port_dir.get_base_dir()]:
			roots = files.discover(candidate)
			if not roots.empty() or not files.error.empty():
				break
	if roots.empty():
		var d1 = config.get_value("game", "disc1", "")
		var d2 = config.get_value("game", "disc2", "")
		if not d1.empty() and not d2.empty():
			roots = [d1, d2]
	if roots.size() == 2 and prepare_index(roots):
		config.set_value("game", "disc1", roots[0])
		config.set_value("game", "disc2", roots[1])
		start_runtime(startup_save)
	else:
		show_setup()

func prepare_index(roots):
	if not files.validate(roots[0], roots[1]):
		status.text = files.error
		return false
	var enabled = config.get_value("patches", "enabled", [])
	for group in patch_manifest.get("groups", []):
		if group.id in enabled:
			for name in group.files:
				var path = patch_path(name)
				if not File.new().file_exists(path):
					status.text = "Missing patch: " + name + ". Download or import the patch ZIP in Game files / mods."
					return false
				for disc in [1, 2]:
					files.index[str(disc) + "/" + name.to_lower()] = path
	game_index = files.index.duplicate()
	if not active_mods.empty():
		if not files.apply_mods(active_mods):
			status.text = files.error
			return false
		game_index = files.index.duplicate()
	return true

func start_runtime(save_path = ""):
	dialogue_active = false
	dialogue_request = null
	set_adaptive_active(false)
	adaptive_atlas_revision = -1
	adaptive_text = ""
	adaptive_parsed = null
	adaptive_configured = []
	prefer_classic_now = false
	if is_instance_valid(close_request_dialog):
		close_request_dialog.queue_free()
	close_request_dialog = null
	manual_pause = false
	runtime_failed = false
	for id in sounds.keys():
		stop_sound(id)
	runtime = null
	ready = false
	pending_restart = null
	importing_save = false
	hotspots = []
	hotspot_text = ""
	surface_cache = ""
	queued_dialogs.clear()
	current_dialog = -1
	close_modal()
	# Clues belong to the voyage: those stored beside the save, or none.
	if is_instance_valid(scratchpad):
		scratchpad.begin_voyage(save_file_path(save_path))
	hd_pack_path = find_hd_pack()
	runtime = create_runtime()
	if runtime == null:
		return
	var automated = false
	for arg in OS.get_cmdline_args():
		if arg in ["--integration-test", "--smoke-test", "--ui-test"]:
			automated = true
	if not automated and needs_artwork_choice():
		show_artwork_choice(save_path, true)
		return
	if not automated and needs_dialogue_choice():
		show_dialogue_options(save_path, true)
		return
	if not automated and needs_achievement_choice():
		show_achievement_choice(save_path, true)
		return
	if not automated and needs_helper_choice():
		show_helper_choice(save_path)
		return
	var error = ""
	runtime.execute("profile", JSON.print({"on": OS.is_debug_build()}))
	error = runtime.execute("boot", JSON.print({"index": game_index, "save": save_path, "hd_pack": active_hd_pack(), "vector_ui": active_vector_ui(), "timed_subtitles": config.get_value("dialogue", "timed_subtitles", false), "disable_autosave": not config.get_value("saves", "autosave_enabled", true), "testing": "--integration-test" in OS.get_cmdline_args(), "achievements": achievements.ids(), "disable_achievements": not achievements_enabled(), "clues": scratchpad_enabled(), "original_items": not item_notices_enabled(), "gamma": config.get_value("display", "gamma", [])}))
	if not error.empty():
		show_note(error)
		return
	status.show()
	status.text = "Preparing your voyage…"
	send({"action": "pause", "on": interrupted()})

func find_hd_pack():
	for arg in OS.get_cmdline_args():
		if arg.begins_with("--hd-pack="):
			return arg.substr(10)
	var base = OS.get_executable_path().get_base_dir()
	var candidates = ["res://hdpack", "user://hdpack", base.plus_file("hdpack")]
	var selected = config.get_value("graphics", "hd_folder", "")
	if Engine.has_singleton("TitanicFiles"):
		var plugin = Engine.get_singleton("TitanicFiles")
		var tree = config.get_value("game", "android_tree", "")
		var folder = selected
		if folder.empty() and not tree.empty():
			folder = plugin.find_hd_folder(tree)
		if not folder.empty():
			var error = plugin.prepare_hd_folder(folder)
			if error.empty():
				return "saf-hd:" + folder
			print("HD folder unavailable: ", error)
	elif not selected.empty():
		candidates.push_front(selected)
	for root in [config.get_value("game", "root", ""), config.get_value("game", "disc1", ""), config.get_value("game", "disc2", "")]:
		if not root.empty():
			if root.to_lower().ends_with(".iso"):
				root = root.get_base_dir()
			candidates.append(root.plus_file("hdpack"))
	var game_dir = OS.get_environment("RETANIC_GAME_DIR")
	if not game_dir.empty():
		candidates.append(game_dir.plus_file("hdpack"))
	var appimage = OS.get_environment("APPIMAGE")
	if not appimage.empty():
		candidates.append(appimage.get_base_dir().plus_file("hdpack"))
	if OS.get_name() in ["OSX", "macOS"]:
		candidates.append(base.get_base_dir().get_base_dir().get_base_dir().plus_file("hdpack"))
	for candidate in candidates:
		if File.new().file_exists(candidate.plus_file("manifest.json")):
			return candidate
	return ""

func active_hd_pack():
	return hd_pack_path if config.get_value("graphics", "hd_enabled", true) else ""

func active_vector_ui():
	return "res://artwork/ui" if config.get_value("graphics", "redrawn_ui", true) else ""

# The HD pack and the redrawn interface change in the running game at once;
# the next frame arrives at the new size.
func apply_artwork():
	send({"action": "artwork", "value": {"hd_pack": active_hd_pack(), "vector_ui": active_vector_ui()}})

func toggle_hd_artwork():
	var enabled = not config.get_value("graphics", "hd_enabled", true)
	config.set_value("graphics", "hd_enabled", enabled)
	config.save("user://settings.cfg")
	apply_artwork()
	show_hd_settings()

func show_checkpoint_toast(text):
	if is_instance_valid(checkpoint_toast):
		checkpoint_toast.queue_free()
	checkpoint_toast_serial += 1
	checkpoint_toast = Control.new()
	checkpoint_toast.mouse_filter = Control.MOUSE_FILTER_IGNORE
	checkpoint_toast.rect_position = game_origin + Vector2(16, 16)
	checkpoint_toast.rect_size = Vector2(480, 48)
	var background = ColorRect.new()
	background.name = "Background"
	background.color = Color(0, 0, 0, 0.25)
	background.rect_size = checkpoint_toast.rect_size
	background.mouse_filter = Control.MOUSE_FILTER_IGNORE
	checkpoint_toast.add_child(background)
	var label = Label.new()
	label.text = text
	label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	label.rect_position = Vector2(12, 4)
	label.rect_size = Vector2(456, 40)
	label.autowrap = true
	label.align = Label.ALIGN_CENTER
	label.valign = Label.VALIGN_CENTER
	label.add_font_override("font", get_font_for("14px Arial"))
	label.add_color_override("font_color", Color.white)
	checkpoint_toast.add_child(label)
	add_child(checkpoint_toast)
	get_tree().create_timer(5.0).connect("timeout", self, "hide_checkpoint_toast", [checkpoint_toast_serial])

func hide_checkpoint_toast(serial):
	if serial != checkpoint_toast_serial:
		return
	if is_instance_valid(checkpoint_toast):
		checkpoint_toast.queue_free()
	checkpoint_toast = null

# Unlocks queue up: a milestone often arrives with the event that earned it.
func show_next_achievement():
	if achievement_queue.empty():
		achievement_toast = null
		return
	var event = achievement_queue.pop_front()
	var toast = PanelContainer.new()
	toast.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var plate = UIStyle.plate(Color(0.07, 0.07, 0.06, 0.88), Color("d0b983"), 6)
	for side in ["left", "right"]:
		plate.set("content_margin_" + side, 10)
	for side in ["top", "bottom"]:
		plate.set("content_margin_" + side, 6)
	toast.add_stylebox_override("panel", plate)
	var box = VBoxContainer.new()
	box.mouse_filter = Control.MOUSE_FILTER_IGNORE
	box.add_constant_override("separation", 2)
	toast.add_child(box)
	var heading = Label.new()
	heading.text = "Achievement unlocked"
	heading.add_font_override("font", get_font_for("11px Arial"))
	heading.add_color_override("font_color", Color("d0b983"))
	box.add_child(heading)
	var title = Label.new()
	title.name = "Title"
	title.text = event.title
	title.add_font_override("font", get_font_for("15px Arial"))
	box.add_child(title)
	var about = Label.new()
	about.text = event.description
	about.autowrap = true
	about.rect_min_size.x = 300
	about.add_font_override("font", get_font_for("11px Arial"))
	about.add_color_override("font_color", Color("b8ad94"))
	box.add_child(about)
	add_child(toast)
	toast.rect_size = Vector2(320, 0)
	achievement_toast = toast
	place_achievement_toast()
	get_tree().create_timer(4.0 if achievement_queue.empty() else 2.5).connect("timeout", self, "finish_achievement_toast", [toast])

# Top right, clear of the toolbar and dialogue at the bottom. A checkpoint
# notice in the top left can reach across a narrow screen, so go below it.
func place_achievement_toast():
	if not is_instance_valid(achievement_toast):
		return
	var size = achievement_toast.get_combined_minimum_size()
	achievement_toast.rect_size = Vector2(320, size.y)
	var spot = Vector2(layout_size.x - 320 - 12, 12)
	if is_instance_valid(checkpoint_toast) and Rect2(spot, achievement_toast.rect_size).intersects(checkpoint_toast.get_rect()):
		spot.y = checkpoint_toast.get_rect().end.y + 8
	achievement_toast.rect_position = spot
	if is_instance_valid(item_notice):
		place_item_notice()

func finish_achievement_toast(toast):
	if is_instance_valid(toast):
		toast.queue_free()
	if toast == achievement_toast:
		show_next_achievement()

func load_achievement_catalog():
	if achievement_catalog.empty() and runtime != null:
		var list = JSON.parse(runtime.query("achievements")).result
		if list is Array:
			achievement_catalog = list
	return achievement_catalog

func show_achievements():
	var catalog = load_achievement_catalog()
	var box = panel("Achievements  %d / %d" % [achievements.unlocked_count(catalog), catalog.size()])
	var scroll = ScrollContainer.new()
	scroll.rect_min_size = Vector2(390, 228)
	scroll.follow_focus = true
	scroll.scroll_horizontal_enabled = false
	box.add_child(scroll)
	var list = VBoxContainer.new()
	list.size_flags_horizontal = SIZE_EXPAND_FILL
	scroll.add_child(list)
	var first = null
	for a in achievements.sorted(catalog):
		var row = achievement_row(list, a)
		if first == null:
			first = row
	var back = button(box, "Back", "controller_settings_back")
	if first != null:
		first.grab_focus()
	else:
		back.grab_focus()

# Rows are buttons so a controller can scroll through them; pressing one does nothing.
func achievement_row(list, a):
	var earned = achievements.has(a.id)
	var secret = a.get("hidden", false) and not earned
	var row = Button.new()
	row.rect_min_size = Vector2(370, 40)
	row.focus_mode = Control.FOCUS_ALL
	row.set_meta("achievement", a.id)
	list.add_child(row)
	var title = Label.new()
	title.name = "Title"
	title.text = ("Secret achievement" if secret else a.title)
	title.clip_text = true
	title.rect_position = Vector2(10, 3)
	title.rect_size = Vector2(250, 18)
	title.mouse_filter = Control.MOUSE_FILTER_IGNORE
	title.add_font_override("font", get_font_for("13px Arial"))
	title.add_color_override("font_color", Color("e9d9a6") if earned else Color("8f8a7c"))
	row.add_child(title)
	var about = Label.new()
	about.name = "About"
	about.text = "Keep playing to discover it." if secret else a.description
	about.clip_text = true
	about.rect_position = Vector2(10, 20)
	about.rect_size = Vector2(352, 16)
	about.mouse_filter = Control.MOUSE_FILTER_IGNORE
	about.add_font_override("font", get_font_for("11px Arial"))
	about.add_color_override("font_color", Color("b8ad94") if earned else Color("6f6b60"))
	row.add_child(about)
	row.hint_tooltip = about.text
	if earned:
		var when = Label.new()
		when.name = "When"
		when.text = saves.describe_time(achievements.unlocked[a.id])
		when.rect_position = Vector2(256, 3)
		when.rect_size = Vector2(106, 18)
		when.set("align", 2)
		when.set("horizontal_alignment", 2)
		when.mouse_filter = Control.MOUSE_FILTER_IGNORE
		when.add_font_override("font", get_font_for("11px Arial"))
		when.add_color_override("font_color", Color("a39a84"))
		row.add_child(when)
	return row

func note_area():
	if adaptive_active:
		return Rect2(adaptive.rail, 0, layout_size.x - 2 * adaptive.rail, layout_size.y)
	if dialogue_active:
		return Rect2(Vector2.ZERO, layout_size)
	return Rect2(game_origin, Vector2(512, 384))

const CLUE_KINDS = ["places", "story", "errands", "puzzles"]
const CLUE_KIND_NAMES = {"places": "Places: cabins and directions", "story": "People and story", "errands": "Errands: what people want", "puzzles": "Puzzle hints (gives solutions away)"}
const SCRATCHPAD_MODE_NAMES = {"off": "Off", "own": "My notes only", "clues": "My notes and game clues"}

# "off", "own" (the player's notes only) or "clues" (also notes from what
# characters say, which amount to hints).
func scratchpad_mode():
	var mode = config.get_value("scratchpad", "mode", "")
	if not mode in ["off", "own", "clues"]:
		mode = "clues" if config.get_value("scratchpad", "enabled", true) else "off"
	return mode

func scratchpad_enabled():
	return scratchpad_mode() != "off"

func clue_kinds():
	return config.get_value("scratchpad", "kinds", ["places", "story", "errands"])

func clue_kind_shown(kind):
	return scratchpad_mode() == "clues" and (kind in clue_kinds() or not kind in CLUE_KINDS)

func item_notices_enabled():
	return config.get_value("items", "notices", true)

func toggle_scratchpad():
	show_scratchpad_options("settings")

# Off hides every note and the tray and ignores new clues; notes are kept for
# when it is turned back on in Settings.
func turn_scratchpad_off():
	set_scratchpad(false)
	close_scratchpad()

func set_scratchpad(enabled):
	set_scratchpad_mode("clues" if enabled else "off")

# The game keeps reporting clues unless the scratchpad is off; which of them
# show is the player's choice, so switching back brings them back.
func set_scratchpad_mode(mode):
	config.set_value("scratchpad", "mode", mode)
	config.set_value("scratchpad", "enabled", mode != "off")
	config.save("user://settings.cfg")
	send({"action": "clues", "on": mode != "off"})
	if is_instance_valid(scratchpad):
		scratchpad.clear_popups()
		scratchpad.refresh()

func toggle_clue_kind(kind):
	var kinds = clue_kinds().duplicate()
	if kind in kinds:
		kinds.erase(kind)
	else:
		kinds.append(kind)
	config.set_value("scratchpad", "kinds", kinds)
	config.save("user://settings.cfg")
	if is_instance_valid(scratchpad):
		scratchpad.clear_popups()
		scratchpad.refresh()
	show_scratchpad_options(scratchpad_options_from)

func choose_scratchpad_mode(mode):
	set_scratchpad_mode(mode)
	show_scratchpad_options(scratchpad_options_from)

# Opened from Settings or from the first-start question.
func show_scratchpad_options(from = "settings"):
	scratchpad_options_from = from
	var box = panel("Scratchpad options")
	scratchpad_options_dialog = modal
	var hint = Label.new()
	hint.text = "Keep your own notes, or also get notes from what characters tell you. Game clues are hints and make the game easier; choose which kinds you want."
	hint.autowrap = true
	hint.rect_min_size = Vector2(396, 46)
	hint.add_font_override("font", get_font_for("12px Arial"))
	box.add_child(hint)
	var row = HBoxContainer.new()
	box.add_child(row)
	var focus = null
	for mode in ["off", "own", "clues"]:
		var b = button(row, ("● " if scratchpad_mode() == mode else "") + SCRATCHPAD_MODE_NAMES[mode], "choose_scratchpad_mode", [mode])
		b.name = "mode_" + mode
		b.size_flags_horizontal = SIZE_EXPAND_FILL
		if scratchpad_mode() == mode:
			focus = b
	for kind in CLUE_KINDS:
		var b = button(box, "%s: %s" % [CLUE_KIND_NAMES[kind], "On" if kind in clue_kinds() else "Off"], "toggle_clue_kind", [kind])
		b.name = "kind_" + kind
		b.disabled = scratchpad_mode() != "clues"
	button(box, "Back", "close_scratchpad_options")
	if focus != null:
		focus.grab_focus()

func close_scratchpad_options():
	if scratchpad_options_from == "helpers":
		show_helper_choice(helper_choice_save)
	else:
		show_settings()

func scratchpad_summary():
	if scratchpad_mode() != "clues":
		return SCRATCHPAD_MODE_NAMES[scratchpad_mode()]
	var kinds = clue_kinds()
	return "Game clues" if kinds.empty() else "Notes and clues (%d of %d kinds)" % [kinds.size(), CLUE_KINDS.size()]

func set_item_notices(enabled):
	config.set_value("items", "notices", enabled)
	config.save("user://settings.cfg")
	send({"action": "item_notices", "on": enabled})
	if not enabled:
		item_queue.clear()
		if is_instance_valid(item_notice):
			item_notice.queue_free()
		item_notice = null

func set_autosave(enabled):
	config.set_value("saves", "autosave_enabled", enabled)
	config.save("user://settings.cfg")
	send({"action": "autosave_enabled", "on": enabled})

func toggle_item_notices():
	set_item_notices(not item_notices_enabled())
	show_settings()

# A received item is named for five seconds at the top of the screen, with
# its picture; the engine has already put it in the bag.
func show_next_item():
	if item_queue.empty():
		item_notice = null
		return
	var event = item_queue.pop_front()
	var notice = PanelContainer.new()
	notice.name = "ItemNotice"
	notice.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var plate = UIStyle.plate(Color(0.07, 0.07, 0.06, 0.88), Color("d0b983"), 6)
	for side in ["left", "right"]:
		plate.set("content_margin_" + side, 10)
	for side in ["top", "bottom"]:
		plate.set("content_margin_" + side, 6)
	notice.add_stylebox_override("panel", plate)
	var row = HBoxContainer.new()
	row.mouse_filter = Control.MOUSE_FILTER_IGNORE
	row.add_constant_override("separation", 10)
	notice.add_child(row)
	var picture = item_picture(event.get("icon", ""))
	if picture != null:
		var icon = TextureRect.new()
		icon.name = "Icon"
		icon.texture = picture
		icon.mouse_filter = Control.MOUSE_FILTER_IGNORE
		icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		row.add_child(icon)
	var box = VBoxContainer.new()
	box.mouse_filter = Control.MOUSE_FILTER_IGNORE
	box.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(box)
	var title = Label.new()
	title.name = "Title"
	title.text = "Item received: " + event.name
	title.add_font_override("font", get_font_for("15px Arial"))
	box.add_child(title)
	if event.get("bag", false):
		var about = Label.new()
		about.text = "It is in your bag."
		about.add_font_override("font", get_font_for("11px Arial"))
		about.add_color_override("font_color", Color("b8ad94"))
		box.add_child(about)
	add_child(notice)
	item_notice = notice
	place_item_notice()
	get_tree().create_timer(5.0).connect("timeout", self, "finish_item_notice", [notice])

func item_picture(encoded):
	if encoded.empty():
		return null
	var img = Image.new()
	if img.load_png_from_buffer(Marshalls.base64_to_raw(encoded)) != OK:
		return null
	# Keep the original pixels, at a size that suits the notice.
	var scale = max(1, floor(48.0 / max(img.get_width(), img.get_height())))
	if max(img.get_width(), img.get_height()) > 56:
		scale = 56.0 / max(img.get_width(), img.get_height())
	img.resize(int(max(1, img.get_width() * scale)), int(max(1, img.get_height() * scale)), Image.INTERPOLATE_NEAREST)
	var texture = ImageTexture.new()
	texture.create_from_image(img)
	return texture

# Top centre, clear of the achievement notice at the top right.
func place_item_notice():
	if not is_instance_valid(item_notice):
		return
	item_notice.rect_size = item_notice.get_combined_minimum_size()
	var spot = Vector2((layout_size.x - item_notice.rect_size.x) / 2, 12)
	for other in [achievement_toast, checkpoint_toast]:
		if is_instance_valid(other) and Rect2(spot, item_notice.rect_size).intersects(other.get_rect()):
			spot.y = max(spot.y, other.get_rect().end.y + 8)
	item_notice.rect_position = spot

func finish_item_notice(notice):
	if is_instance_valid(notice):
		notice.queue_free()
	if notice == item_notice:
		show_next_item()

var note_editor = null
# The list opened from the notes tray returns to the game, not the menu.
var scratchpad_from_tray = false

func open_scratchpad():
	if modal != null or runtime == null:
		return
	send({"action": "pause", "on": true})
	manual_pause = true
	scratchpad_from_tray = true
	show_scratchpad()

func open_scratchpad_from_menu():
	scratchpad_from_tray = false
	show_scratchpad()

func close_scratchpad():
	if scratchpad_from_tray:
		scratchpad_from_tray = false
		resume_game()
	else:
		controller_settings_back()

func toggle_notes_shown():
	scratchpad.set_collapsed(not scratchpad.collapsed)
	scratchpad.finish_animations()
	show_scratchpad()

func show_scratchpad():
	var box = panel("Scratchpad")
	var hint = Label.new()
	hint.text = "Clues from this voyage and your own notes. Drag pinned notes anywhere, or onto the notes tray to put them away."
	hint.autowrap = true
	hint.rect_min_size = Vector2(396, 30)
	hint.add_font_override("font", get_font_for("12px Arial"))
	box.add_child(hint)
	var scroll = ScrollContainer.new()
	scroll.rect_min_size = Vector2(396, 128)
	scroll.follow_focus = true
	scroll.scroll_horizontal_enabled = false
	box.add_child(scroll)
	var list = VBoxContainer.new()
	list.name = "Notes"
	list.size_flags_horizontal = SIZE_EXPAND_FILL
	scroll.add_child(list)
	var first = null
	for id in scratchpad.listed():
		var focus = scratchpad_row(list, id)
		if first == null:
			first = focus
	if first == null:
		var none = Label.new()
		none.text = "No notes yet."
		none.add_font_override("font", get_font_for("12px Arial"))
		list.add_child(none)
	var write = button(box, "Write a note", "show_note_writer")
	if scratchpad.pinned_count() > 0:
		button(box, "Lay out pinned notes" if scratchpad.collapsed else "Gather pinned notes into the tray", "toggle_notes_shown")
	var clues = false
	for id in scratchpad.order:
		clues = clues or not scratchpad.notes[id].own
	if clues:
		button(box, "Remove this voyage's clues", "confirm_clear_scratchpad_clues")
	var row = HBoxContainer.new()
	box.add_child(row)
	button(row, "Back", "close_scratchpad")
	button(row, "Turn the scratchpad off", "turn_scratchpad_off")
	if first != null:
		first.grab_focus()
	else:
		write.grab_focus()

func scratchpad_row(list, id):
	var note = scratchpad.notes[id]
	var row = HBoxContainer.new()
	row.set_meta("note", id)
	list.add_child(row)
	var text = Label.new()
	text.name = "Text"
	text.text = ("" if note.own else "Clue: ") + note.text
	text.autowrap = true
	text.rect_min_size.x = 236
	text.size_flags_horizontal = SIZE_EXPAND_FILL
	text.add_font_override("font", get_font_for("12px Arial"))
	row.add_child(text)
	var pin = button(row, "Unpin" if note.pinned else "Pin", "toggle_note_pin", [id])
	pin.name = "Pin"
	pin.rect_min_size.x = 58
	var remove = button(row, "Delete", "delete_note", [id])
	remove.name = "Delete"
	return pin

func toggle_note_pin(id):
	scratchpad.set_pinned(id, not scratchpad.notes[id].pinned)
	show_scratchpad()

func delete_note(id):
	scratchpad.remove(id)
	show_scratchpad()

func confirm_clear_scratchpad_clues():
	var box = panel("Remove this voyage's clues?")
	text_scroller(box, "Your own notes are kept. Clues stored with your saves come back when you load them.", 60)
	button(box, "Remove clues", "clear_scratchpad_clues")
	button(box, "Cancel", "show_scratchpad").grab_focus()

func clear_scratchpad_clues():
	scratchpad.clear_clues()
	show_scratchpad()

func show_note_writer():
	var box = panel("Write a note")
	note_editor = LineEdit.new()
	note_editor.placeholder_text = "Something to remember"
	note_editor.add_font_override("font", get_font_for("14px Arial"))
	note_editor.connect("text_entered", self, "save_written_note")
	box.add_child(note_editor)
	button(box, "On-screen keyboard", "show_keyboard", [note_editor])
	button(box, "Pin note", "save_written_note")
	button(box, "Back", "show_scratchpad")
	note_editor.grab_focus()

func save_written_note(_text = ""):
	if is_instance_valid(note_editor) and not scratchpad.write(note_editor.text).empty():
		show_scratchpad()

func toggle_autosave():
	set_autosave(not autosave_enabled())
	show_setup()

func autosave_enabled():
	return config.get_value("saves", "autosave_enabled", true)

# The helpers the original never had are asked about once, together; each
# can be changed at any time in Settings, also during a voyage.
func needs_helper_choice():
	return not config.get_value("helpers", "chosen", false)

func helper_label(key):
	match key:
		"scratchpad":
			return "Scratchpad: %s" % scratchpad_summary()
		"items":
			return "New items: %s" % ("Into the bag, with a notice" if item_notices_enabled() else "In hand (original)")
		"autosave":
			return "Autosave checkpoints: %s" % ("On" if autosave_enabled() else "Off")
	return key

func show_helper_choice(save_path = ""):
	helper_choice_save = save_path
	var box = panel("Modern helpers")
	helper_choice_dialog = modal
	status.hide()
	var hint = Label.new()
	hint.text = "The original game had none of these. The scratchpad keeps your notes, and if you like, clues people mention, which make the game easier; new items go into the bag with a notice instead of waiting in hand; the game saves at checkpoints. Change them at any time in Settings."
	hint.autowrap = true
	hint.rect_min_size = Vector2(396, 92)
	hint.add_font_override("font", get_font_for("13px Arial"))
	box.add_child(hint)
	for key in ["scratchpad", "items", "autosave"]:
		var b = button(box, helper_label(key), "toggle_helper", [key])
		b.name = "helper_" + key
	button(box, "Start", "choose_helpers").grab_focus()

func toggle_helper(key):
	match key:
		"scratchpad":
			show_scratchpad_options("helpers")
			return
		"items":
			set_item_notices(not item_notices_enabled())
		"autosave":
			set_autosave(not autosave_enabled())
	var b = modal.find_node("helper_" + key, true, false)
	if b != null:
		b.text = helper_label(key)
		b.grab_focus()

func choose_helpers():
	helper_choice_dialog = null
	config.set_value("helpers", "chosen", true)
	config.save("user://settings.cfg")
	start_runtime(helper_choice_save)

func create_runtime():
	var runtime = null
	if ClassDB.class_exists("DreamRuntime"):
		runtime = ClassDB.instance("DreamRuntime")
	else:
		var library = GDNativeLibrary.new()
		var library_config = ConfigFile.new()
		library_config.set_value("general", "symbol_prefix", "godot_")
		library_config.set_value("general", "reloadable", false)
		# FRT identifies as X11 on some versions, Server on others. Select by actual CPU.
		var platform = OS.get_name()
		var arch = "x86_64" if OS.has_feature("64") else "x86"
		if OS.has_feature("arm64"):
			arch = "aarch64"
		elif OS.has_feature("armv7") or OS.has_feature("arm"):
			arch = "armhf"
		# PortMaster explicitly supplies the architecture because older FRT feature tags vary.
		var forced = OS.get_environment("RETANIC_ARCH")
		if forced in ["aarch64", "armhf", "x86_64", "x86"]:
			arch = forced
		var binary = "res://native/libtitanic.%s.so" % arch
		if platform == "Windows":
			binary = "res://native/libtitanic.%s.dll" % arch
		elif platform == "OSX":
			binary = "res://native/libtitanic.dylib"
		var native_dir = OS.get_environment("RETANIC_NATIVE_DIR")
		if not native_dir.empty():
			binary = native_dir.plus_file(binary.get_file())
		library_config.set_value("entry", platform, binary)
		library.config_file = library_config
		var script = NativeScript.new()
		script.library = library
		script.set_class_name("DreamRuntime")
		runtime = script.new()
	if runtime == null:
		show_note("The engine library could not be loaded for this architecture.")
		return
	var error = runtime.initialize(self)
	if not error.empty():
		show_note(error)
		return
	return runtime

func index_iso(path):
	var reader = create_runtime()
	if reader == null:
		return {"error": "The ISO reader could not be loaded."}
	var result = JSON.parse(reader.query("iso_index", JSON.print({"path": ProjectSettings.globalize_path(path)}))).result
	return result if result is Dictionary else {"error": "This engine build does not support ISO images."}

func bridge_call(method, args_json, bytes):
	var args = JSON.parse(args_json).result
	match method:
		"svg":
			return preload("res://scripts/svg_artwork.gd").rasterize(bytes, int(args.width), int(args.height), args.path)
		"read":
			var path = args.path
			if path.begins_with("saf-hd:") and Engine.has_singleton("TitanicFiles"):
				return Engine.get_singleton("TitanicFiles").read_hd_file(path.substr(7))
			if path.begins_with("save:"):
				path = saves.path_for(path.substr(5))
			return files.read_asset(path)
		"write":
			if args.path.begins_with("autosave:"):
				var saved = saves.write_autosave(args.path.substr(9), bytes)
				if saved and has_frame:
					saves.write_thumbnail(saves.last_written, frame_image)
				if saved:
					scratchpad.save_voyage(saves.last_written)
				return JSON.print({"ok": true} if saved else {"error": saves.error})
			if not args.path.begins_with("save:"):
				return JSON.print({"error": "Invalid save destination"})
			var ok = saves.write(args.path.substr(5), bytes)
			if ok:
				# An imported file was saved somewhere else: the picture on
				# screen is not its picture, and an older one would mislead.
				if importing_save or not has_frame:
					Directory.new().remove(saves.thumbnail_path(saves.last_written))
				else:
					saves.write_thumbnail(saves.last_written, frame_image)
				# An imported save brings no clues; this voyage's are not its.
				if importing_save:
					Directory.new().remove(scratchpad.voyage_path(saves.last_written))
				else:
					scratchpad.save_voyage(saves.last_written)
			return JSON.print({"error": saves.error} if not ok else {"ok": true})
		"measure":
			return JSON.print(get_font_for(args.font).get_string_size(args.text).x)
		"log":
			print(args.text)
	return "null"

func get_font_for(css):
	var key = str(css)
	if not fonts.has(key):
		var font = DynamicFont.new()
		font.size = int(key.split("px")[0])
		font.font_data = load(font_file(key))
		fonts[key] = font
	return fonts[key]

func font_file(key):
	if "Courier" in key:
		return "res://fonts/LiberationMono-Regular.ttf"
	if "Serif Italic" in key:
		return "res://fonts/LiberationSerif-Italic.ttf"
	if "Serif" in key:
		return "res://fonts/LiberationSerif-Regular.ttf"
	return "res://fonts/LiberationSans-Regular.ttf"

func ink(text):
	if text.begins_with("rgb"):
		var start = text.find("(") + 1
		var values = text.substr(start, text.length() - start - 1).split(",")
		return Color(float(values[0]) / 255, float(values[1]) / 255, float(values[2]) / 255, float(values[3]) if values.size() == 4 else 1.0)
	return Color(text)

func adaptive_enabled():
	return config.get_value("graphics", "adaptive_exploration", not classic_format_device())

func adaptive_roomy():
	return config.get_value("graphics","adaptive_roomy",false)

func classic_format(size):
	return max(size.x,size.y) < min(size.x,size.y)*1.6

func classic_format_device():
	# Desktop windows start at 4:3 even on wide monitors. Classify the device,
	# while the active layout below always uses the actual window size.
	# Asking the display server is a round trip, and layout code asks several
	# times per frame; a screen keeps its shape, so ask about once a second.
	var now = OS.get_ticks_msec()
	if device_format_checked < 0 or now - device_format_checked >= 1000:
		device_format_checked = now
		var size = OS.get_screen_size(OS.current_screen)
		if size.x <= 0 or size.y <= 0:
			size = OS.window_size
		device_format_classic = classic_format(size)
	return device_format_classic

func small_display():
	var size = OS.window_size
	return min(size.x, size.y) < 576

func adaptive_widescreen():
	return not classic_format_device() and OS.window_size.x >= OS.window_size.y * 1.6

func set_adaptive_active(active):
	if adaptive_active == active:
		return
	# A screen change must not leave a held click attached to a different UI.
	if pointer_pressed:
		send({"action": "pointer", "kind": "release", "x": -1, "y": -1})
		pointer_pressed = false
	adaptive_active = active
	controller_selection = -1
	dialogue_preselected = false
	update_touch_layout()
	# Layout changes move the artwork, not the physical mouse. Reinterpret
	# its screen position instead of projecting an old game target onto it.
	pointer = display_to_game(display_pointer)
	refresh_cursor()
	update()

func sync_adaptive_layout():
	adaptive_available = false
	if not adaptive_enabled() or not adaptive_widescreen() or modal != null or runtime == null or runtime_failed or not has_frame:
		adaptive_leaving_since = -1
		set_adaptive_active(false)
		update_layout_switch()
		return
	# The answer rarely changes between frames; parsing it every frame costs
	# more than the engine spends producing it.
	var text = runtime.query("adaptive_layout")
	if text != adaptive_text:
		adaptive_text = text
		adaptive_parsed = JSON.parse(text).result
	var metadata = adaptive_parsed
	if not metadata is Dictionary or not metadata.get("eligible", false) or not adaptive_overlays_fit():
		if adaptive_active and not prefer_classic_now and metadata is Dictionary and metadata.get("hold", false) and adaptive_overlays_fit():
			var now = OS.get_ticks_msec()
			if adaptive_leaving_since < 0:
				adaptive_leaving_since = now
			if now - adaptive_leaving_since < adaptive_hold_ms:
				# Keep the last panels and artwork; the world shows the frame.
				adaptive_available = true
				update_layout_switch()
				return
		adaptive_leaving_since = -1
		set_adaptive_active(false)
		update_layout_switch()
		return
	adaptive_leaving_since = -1
	adaptive_available = true
	if prefer_classic_now:
		set_adaptive_active(false)
		update_layout_switch()
		return
	adaptive_metadata = metadata
	var configured = [text, layout_size, adaptive_roomy()]
	if configured != adaptive_configured:
		adaptive_configured = configured
		adaptive.configure(layout_size, metadata, adaptive_roomy())
	var atlas_scale = int(metadata.get("atlas_scale",1))
	if not atlas_scale in [1,2]:
		adaptive_available = false
		set_adaptive_active(false)
		update_layout_switch()
		return
	# Five toolbar slots, then one each for an item in hand and the Help button.
	var atlas_width = 128 * int(metadata.get("atlas_slots", 5))
	var resized = adaptive_atlas_image.get_width() != atlas_width * atlas_scale
	if metadata.revision != adaptive_atlas_revision or atlas_scale != adaptive_atlas_scale or resized:
		var bytes = runtime.buffer("adaptive_atlas")
		if atlas_width < 640 or atlas_width > 896 or bytes.size() != atlas_width * 128 * 4 * atlas_scale * atlas_scale:
			adaptive_available = false
			set_adaptive_active(false)
			update_layout_switch()
			return
		adaptive_atlas_image.create_from_data(atlas_width*atlas_scale,128*atlas_scale,false,Image.FORMAT_RGBA8,bytes)
		if adaptive_atlas_revision < 0 or atlas_scale != adaptive_atlas_scale or resized:
			adaptive_atlas_texture.create_from_image(adaptive_atlas_image,0)
		else:
			adaptive_atlas_texture.set_data(adaptive_atlas_image)
		adaptive_atlas_revision = metadata.revision
		adaptive_atlas_scale = atlas_scale
		update()
	set_adaptive_active(true)
	update_layout_switch()

func update_layout_switch():
	if not is_instance_valid(layout_switch):
		return
	layout_switch.visible = adaptive_available and modal == null and not touch_editing and adaptive_widescreen()
	layout_switch.text = "Classic" if adaptive_active else "Side panels"
	layout_switch.rect_position = Vector2(6,adaptive.SWITCH_Y) if adaptive_active else Vector2(6,24)
	layout_switch.rect_size = Vector2(adaptive.rail-12 if adaptive_active else max(60,game_origin.x-12),26)

func toggle_exploration_layout():
	if not adaptive_available or not adaptive_widescreen():
		return
	prefer_classic_now = not prefer_classic_now
	sync_adaptive_layout()

func game_to_display(point, target_id = ""):
	if dialogue_active:
		return dialogue.game_to_screen(point)
	return adaptive.game_to_screen(point, target_id) if adaptive_active else game_origin + point

func display_to_game(point):
	if dialogue_active:
		return dialogue.screen_to_game(point)
	return adaptive.screen_to_game(point) if adaptive_active else point - game_origin

func pointer_in_picture(point):
	var local = display_to_game(point)
	return Rect2(0,0,512,384).has_point(local)

func selection_circle(target):
	if dialogue_active and target.aim_y >= 264 and int((target.aim_y-264)/24) < dialogue.choices.size():
		var r = dialogue.choices[int((target.aim_y-264)/24)]
		return {"center":r.position+Vector2(-12,12), "radius":7}
	if target.id == "ui:layout":
		return {"center":layout_switch.rect_position+layout_switch.rect_size/2-(Vector2.ZERO if adaptive_active or dialogue_active else game_origin),"radius":16}
	if target.id == "ui:notes":
		return {"center":scratchpad.tray_centre()-(Vector2.ZERO if adaptive_active or dialogue_active else game_origin),"radius":22}
	if adaptive_active:
		return adaptive.target_circle(target)
	return {"center": Vector2(target.aim_x,target.aim_y), "radius": clamp(min(target.w,target.h) / 2.0 + 5, 12, 30)}

func draw_touch_surround():
	if not touch_enabled or modal != null:
		return
	if OS.window_size.y > OS.window_size.x:
		draw_style_box(touch_surround_style, Rect2(12,game_origin.y+408,488,132))
	else:
		for x in [4,layout_size.x-game_origin.x+4]:
			draw_style_box(touch_surround_style,Rect2(x,16,max(56,game_origin.x-8),352))

func adaptive_overlays_fit():
	for command in overlays:
		if command.op == "text":
			if command.y < 0 or command.y > 264:
				return false
		elif not command.op in ["rect","stroke"]:
			return false
	return true

func draw_adaptive_overlays():
	draw_set_transform(adaptive.world.position,0,adaptive.world_scale())
	for command in overlays:
		if command.op == "text":
			draw_string(get_font_for(command.font),Vector2(command.x,command.y),command.text,ink(command.color),max(0,512-command.x))
		else:
			var bounds = Rect2(command.x,command.y,command.w,command.h).clip(Rect2(0,0,512,264))
			if bounds.size.x > 0 and bounds.size.y > 0:
				if command.op == "rect":
					draw_rect(bounds,ink(command.color))
				else:
					draw_rect(bounds,ink(command.color),false,command.get("line",1.0))
	draw_set_transform(Vector2.ZERO,0,Vector2.ONE)

func draw_adaptive():
	var hd = frame_image.get_width() / 512.0
	draw_texture_rect_region(frame_texture, adaptive.world, Rect2(0,0,512*hd,264*hd))
	draw_adaptive_overlays()
	for bounds in adaptive.panels:
		draw_style_box(adaptive_panel_style,bounds)
	for c in adaptive.controls:
		var highlighted = pointer_visible and c.display.has_point(display_pointer)
		draw_style_box(adaptive_hover_style if highlighted else adaptive_button_style,c.display)
		var scale = min((c.display.size.x-8)/c.w,58.0/c.h)
		var size = Vector2(c.w,c.h)*scale
		var destination = Rect2(c.display.position+Vector2((c.display.size.x-size.x)/2,4+(58-size.y)/2),size)
		draw_texture_rect_region(adaptive_atlas_texture,destination,Rect2(Vector2(c.slot*128,0)*adaptive_atlas_scale,Vector2(c.w,c.h)*adaptive_atlas_scale))
		var font = get_font_for("11px Arial")
		draw_string(font,c.display.position+Vector2(6,74),c.label,UIStyle.INK)
	# Keep the entire arrow visible at the bottom of the expanded world.
	if not adaptive.navigation.empty():
		var n = adaptive.navigation
		draw_texture_rect_region(adaptive_atlas_texture,adaptive.navigation_display,Rect2(Vector2(n.slot*128,0)*adaptive_atlas_scale,Vector2(n.w,n.h)*adaptive_atlas_scale))

func _draw():
	if dialogue_active and has_frame:
		draw_wide_dialogue()
		return
	if adaptive_active and has_frame:
		draw_adaptive()
		return
	draw_touch_surround()
	draw_set_transform(game_origin, 0, Vector2.ONE)
	if has_frame:
		draw_texture_rect(frame_texture, Rect2(0, 0, 512, 384), false)
		for command in overlays:
			if command.op == "text":
				draw_string(get_font_for(command.font), Vector2(command.x, command.y), command.text, ink(command.color))
			else:
				if command.op == "rect":
					draw_rect(Rect2(command.x, command.y, command.w, command.h), ink(command.color))
				else:
					draw_rect(Rect2(command.x, command.y, command.w, command.h), ink(command.color), false, command.get("line", 1.0))

func display_cursor_name():
	# The game's hidden cursor applies only to its picture, never host controls.
	if modal != null or not pointer_in_picture(display_pointer):
		return "arrow"
	if is_instance_valid(layout_switch) and layout_switch.visible and layout_switch.get_global_rect().has_point(display_pointer):
		return "arrow"
	return pointer_name

func draw_cursor():
	if modal == null and not hotspots.empty():
		draw_hotspots()
	if touch_enabled and modal != null:
		return
	if modal == null and not dialogue_preselected and controller_selection >= 0 and controller_selection < controller_surface.targets.size():
		var target = controller_surface.targets[controller_selection]
		var circle = selection_circle(target)
		cursor_layer.draw_arc(circle.center, circle.radius, 0, TAU, 64, Color(0.78,0.68,0.45,0.50), 1.4, true)
	var name = display_cursor_name()
	if not pointer_visible or name == "none":
		return
	var cursor = display_pointer - cursor_layer.position
	# A light shape with a dark edge stays visible on both the bright cabins
	# and the night decks; the plain arrow is white, anything special brass.
	var shape = cursor_shape(name)
	var outline = PoolVector2Array()
	for p in shape:
		outline.append(cursor + p)
	cursor_layer.draw_colored_polygon(outline, Color("f7e6a4") if shape != POINTER_ARROW else Color.white)
	outline.append(outline[0])
	cursor_layer.draw_polyline(outline, Color(0,0,0), 1.5, true)
	for detail in POINTER_DETAILS.get(name, []):
		cursor_layer.draw_line(cursor + detail[0], cursor + detail[1], Color(0,0,0,0.6), 1.0, true)

func cursor_shape(name):
	if POINTER_SHAPES.has(name):
		return POINTER_SHAPES[name]
	if name != "sight" and not name.begins_with("go"):
		return POINTER_ARROW
	var shape = []
	if name == "sight":
		for quarter in range(4):
			for p in POINTER_ARM:
				shape.append(p.rotated(quarter * PI / 2))
	elif name.begins_with("go"):
		var turn = {"goright": PI / 2, "godown": PI, "goleft": -PI / 2}.get(name, 0.0)
		for p in POINTER_UP:
			shape.append(p.rotated(turn))
	return shape

func send(command):
	if command.get("action", "") == "pause":
		command.on = command.on or interrupted()
	if interrupted() and command.get("action", "") in ["key", "pointer"]:
		return
	if runtime_failed and command.get("action", "") in ["key", "pointer"]:
		return
	if runtime != null:
		var started = OS.get_ticks_usec()
		runtime.execute("command", JSON.print(command))
		var elapsed = OS.get_ticks_usec() - started
		if OS.is_debug_build() and elapsed > 50000:
			print("PERF command ", command.get("action", ""), " ", elapsed / 1000.0, " ms")

func _process(delta):
	var now = OS.get_ticks_msec()
	# Some handheld backends do not send lifecycle notifications on suspend.
	# Discard the first elapsed interval and stale controls after a long gap.
	if last_process_ms > 0 and now - last_process_ms > 2000:
		send({"action": "pause", "on": true})
		clear_interrupted_input()
		send({"action": "pause", "on": manual_pause or interrupted()})
		resume_frame = true
	last_process_ms = now
	if resume_frame:
		delta = 0.0
		resume_frame = false
	var cleanup_error = saves.service_autosaves()
	if not cleanup_error.empty():
		show_checkpoint_toast(cleanup_error)
		print("AUTOSAVE CLEANUP: ", cleanup_error)
	poll_patches(delta)
	process_touch(delta)
	process_controller(delta)
	process_hotspots(delta)
	refresh_cursor()
	if runtime != null:
		var wide_requested = wide_dialogue_enabled() and adaptive_widescreen()
		if wide_requested != dialogue_request:
			send({"action":"wide_dialogue", "on":wide_requested})
			dialogue_request = wide_requested
		var tick_started = OS.get_ticks_usec()
		var error = "" if runtime_failed else runtime.execute("tick", JSON.print({"dt": min(delta, 0.25) * 1000.0}))
		var tick_finished = OS.get_ticks_usec()
		if not error.empty():
			if smoke:
				print("SMOKE FAILURE: ", error)
				get_tree().quit(1)
			elif not runtime_failed:
				runtime_failed = true
				manual_pause = true
				send({"action": "pause", "on": true})
				show_note("The game stopped. Open Menu to load a saved game.\n\n" + error.split("\n")[0])
			return
		var bytes = runtime.buffer("frame")
		if bytes.size() == 512 * 384 * 4 or bytes.size() == 1024 * 768 * 4:
			var scale = 2 if bytes.size() == 1024 * 768 * 4 else 1
			var resized = not has_frame or frame_image.get_width() != 512 * scale
			frame_image.create_from_data(512 * scale, 384 * scale, false, Image.FORMAT_RGBA8, bytes)
			if resized:
				frame_texture.create_from_image(frame_image, 0)
				apply_picture_filter()
				has_frame = true
			else:
				frame_texture.set_data(frame_image)
			overlays = JSON.parse(runtime.query("overlay")).result
			update()
		var present_finished = OS.get_ticks_usec()
		# Most frames carry no events; skip the parser for those.
		var events_text = runtime.query("events")
		if events_text != "[]":
			for event in JSON.parse(events_text).result:
				handle_event(event)
		if pending_restart != null:
			start_runtime(pending_restart)
		sync_dialogue_layout()
		sync_adaptive_layout()
		if OS.is_debug_build():
			var finished = OS.get_ticks_usec()
			perf_frames += 1
			perf_elapsed += delta
			perf_tick_us += tick_finished - tick_started
			perf_present_us += present_finished - tick_finished
			perf_events_us += finished - present_finished
			perf_peak_us = max(perf_peak_us, finished - tick_started)
			if perf_elapsed >= 5.0:
				print("PERF fps=", perf_frames / perf_elapsed, " tick_ms=", perf_tick_us / (1000.0 * perf_frames), " present_ms=", perf_present_us / (1000.0 * perf_frames), " events_ms=", perf_events_us / (1000.0 * perf_frames), " peak_ms=", perf_peak_us / 1000.0)
				print("PERF engine totals ", runtime.query("timings"))
				perf_elapsed = 0.0
				perf_frames = 0
				perf_tick_us = 0
				perf_present_us = 0
				perf_events_us = 0
				perf_peak_us = 0
		if smoke:
			smoke_time += delta
			if smoke_time > 12.0:
				print("SMOKE STATE: ", runtime.query("state"))
				frame_image.save_png("user://smoke.png")
				get_tree().quit(0 if ready and has_frame else 1)

func refresh_cursor():
	var name = display_cursor_name()
	cursor_layer.visible = not ((touch_enabled and modal != null) or (name == "none" and controller_selection < 0 and hotspots.empty()))
	var origin = Vector2.ZERO if adaptive_active or dialogue_active else game_origin
	var origin_changed = cursor_layer.position != origin
	# Godot 3 invalidates the canvas even when the assigned position is equal.
	if origin_changed:
		cursor_layer.position = origin
	if origin_changed or drawn_pointer != pointer or drawn_pointer_name != name or drawn_display_pointer != display_pointer:
		drawn_pointer = pointer
		drawn_display_pointer = display_pointer
		drawn_pointer_name = name
		cursor_layer.update()

func handle_event(event):
	match event.type:
		"ready", "stage":
			ready = true
			status.hide()
		"error":
			print("ENGINE ERROR: ", event.text)
			if smoke:
				get_tree().quit(1)
			else:
				show_note(event.text)
		"log":
			print(event.text)
		"status":
			status.text = event.text
		"cursor":
			pointer_name = event.name
		"dialog":
			queued_dialogs.append(event)
			if current_dialog == -1:
				show_next_dialog()
		"audio_play":
			play_sound(event)
		"audio_stop":
			stop_sound(event.id)
		"audio_volume":
			volumes[event.channel] = event.volume
			for id in sounds:
				if sounds[id].get_meta("channel") == event.channel:
					sounds[id].volume_db = linear2db(max(0.00001, event.volume))
		"audio_pause":
			audio_paused = event.on
			for player in sounds.values():
				player.stream_paused = audio_paused or interrupted() or manual_pause
		"quit":
			get_tree().quit()
		"restart":
			pending_restart = event.get("save", "")
		"autosaving":
			show_checkpoint_toast("Checkpoint reached - saving ...")
		"autosave_failed":
			show_checkpoint_toast("Autosave failed: " + event.text)
			print("AUTOSAVE ERROR: ", event.text)
		"gamma":
			# Brightness chosen with F1-F9 lasts across loads and starts.
			config.set_value("display", "gamma", event.channels)
			config.save("user://settings.cfg")
		"achievement":
			if achievements_enabled() and achievements.unlock(event.id):
				achievement_queue.append(event)
				if not is_instance_valid(achievement_toast):
					show_next_achievement()
		"item":
			if item_notices_enabled():
				item_queue.append(event)
				if not is_instance_valid(item_notice):
					show_next_item()
		"clue":
			if scratchpad_enabled() and scratchpad.learn(event.id, event.text, event.get("kind", "")):
				scratchpad.popup(event.id)
		"saved":
			print("Saved: ", event.name)
			show_checkpoint_toast("Quick save stored" if event.get("quick", false) else "Game saved")

func play_sound(event):
	var bytes = runtime.buffer("audio", JSON.print({"id": event.id}))
	if bytes.empty():
		return
	var stream = AudioStreamSample.new()
	stream.format = AudioStreamSample.FORMAT_16_BITS
	stream.stereo = true
	stream.mix_rate = event.rate
	stream.data = bytes
	if event.loop:
		stream.loop_mode = AudioStreamSample.LOOP_FORWARD
		stream.loop_begin = 0
		stream.loop_end = event.samples
	var player = AudioStreamPlayer.new()
	player.stream = stream
	player.volume_db = linear2db(max(0.00001, volumes[event.channel]))
	player.set_meta("channel", event.channel)
	add_child(player)
	player.connect("finished", self, "sound_finished", [event.id])
	sounds[event.id] = player
	player.play()
	player.stream_paused = audio_paused or interrupted() or manual_pause

func sound_finished(id):
	stop_sound(id)
	send({"action": "audio_done", "id": id})

func stop_sound(id):
	if sounds.has(id):
		sounds[id].stop()
		sounds[id].queue_free()
		sounds.erase(id)

func interrupted():
	return app_suspended or not focused

func update_audio_pause():
	for player in sounds.values():
		player.stream_paused = audio_paused or interrupted() or manual_pause

func clear_interrupted_input():
	pointer_pressed = false
	hotspot_mouse_held = false
	controller_held.clear()
	controller_triggers = [false, false]
	controller_neutral_required = true
	nav_timer = 0.0
	last_nav = ""
	touch_nav_direction = ""
	touch_nav_timer = 0.0
	if touch_strip != null:
		touch_strip.get_node("joystick").reset()
	for action in ["up", "down", "left", "right", "space", "escape", "menu", "keyboard"]:
		if InputMap.has_action("touch_" + action):
			Input.action_release("touch_" + action)

func lifecycle_changed():
	if interrupted():
		clear_interrupted_input()
	else:
		resume_frame = true
	send({"action": "pause", "on": manual_pause or interrupted()})
	# Do this immediately: Android may suspend before the next event poll.
	update_audio_pause()

func _notification(what):
	if what == MainLoop.NOTIFICATION_WM_FOCUS_OUT:
		focused = false
		lifecycle_changed()
	elif what == MainLoop.NOTIFICATION_WM_FOCUS_IN:
		focused = true
		lifecycle_changed()
	elif what == MainLoop.NOTIFICATION_APP_PAUSED:
		app_suspended = true
		lifecycle_changed()
	elif what == MainLoop.NOTIFICATION_APP_RESUMED:
		app_suspended = false
		lifecycle_changed()
	elif what == MainLoop.NOTIFICATION_WM_GO_BACK_REQUEST:
		if virtual_keyboard != null:
			close_keyboard()
		elif menu != null:
			resume_game()
		elif modal == null:
			show_menu()
	elif what == MainLoop.NOTIFICATION_WM_QUIT_REQUEST:
		if window_close_requested(OS.get_ticks_msec()):
			OS.kill(OS.get_process_id())

func window_close_requested(now):
	if close_last_ms < 0 or now < close_last_ms or now - close_last_ms > 2500:
		close_first_ms = now
		close_count = 0
	close_last_ms = now
	close_count += 1
	# A double-click only opens the menu. Persistent requests over several
	# seconds bypass normal shutdown, including a stuck gameplay session.
	if close_count >= 4 and now - close_first_ms >= 2500:
		return true
	# OS notifications may arrive while the scene tree is locked.
	if not close_menu_pending:
		close_menu_pending = true
		call_deferred("show_window_close_menu")
	return false

func show_window_close_menu():
	close_menu_pending = false
	if is_instance_valid(menu) and modal == menu:
		menu.show()
		return false
	menu = null
	if virtual_keyboard != null:
		close_keyboard()
	if modal != null:
		if is_instance_valid(close_request_dialog):
			close_request_dialog.queue_free()
		close_request_dialog = modal
		close_dialog_pause = manual_pause
		modal.hide()
		modal = null
	show_menu()
	return false

func _input(event):
	if interrupted():
		return
	if block_touch_mouse(event):
		get_tree().set_input_as_handled()
		return
	if controller_binding_input(event):
		get_tree().set_input_as_handled()
		return
	if touch_editing:
		return
	if event is InputEventMouseMotion or event is InputEventMouseButton:
		if not controller_pointer_event:
			set_pointer_visible(true)
		display_pointer = event.position
		var local = display_to_game(event.position)
		# A selected world target can sit behind the translucent controls. Keep
		# its original aim instead of redirecting A to the overlaid toolbar.
		if controller_pointer_event and controller_selection >= 0 and event is InputEventMouseButton:
			local = pointer
		if adaptive_active:
			update()
		# Keep outside coordinates too: the cursor crosses Classic side areas,
		# and releasing a drag outside the picture must not reuse its last hit.
		pointer = local
		if event is InputEventMouseMotion and controller_selection >= 0:
			controller_selection = -1
			dialogue_preselected = false
			cursor_layer.update()
	if event is InputEventKey and event.pressed and not event.echo:
		if event.scancode == KEY_F11 or (event.scancode == KEY_F and event.control and event.meta):
			set_fullscreen(not OS.window_fullscreen)
			get_tree().set_input_as_handled()
		elif modal == null and (event.control or event.meta) and event.shift and event.scancode in [KEY_S, KEY_O]:
			if event.scancode == KEY_S:
				send({"action": "quicksave"})
			else:
				quick_load()
			get_tree().set_input_as_handled()
		elif modal == null and (event.control or event.meta) and event.scancode in [KEY_S, KEY_O]:
			send({"action": "save" if event.scancode == KEY_S else "load"})
			get_tree().set_input_as_handled()
	if event is InputEventKey and event.pressed and not event.echo:
		if event.scancode == KEY_ESCAPE and modal != null:
			controller_back()
			get_tree().set_input_as_handled()
		elif event.scancode in [KEY_PAGEUP, KEY_PAGEDOWN]:
			controller_cycle(-1 if event.scancode == KEY_PAGEUP else 1)
			get_tree().set_input_as_handled()
		elif event.scancode == KEY_F12:
			controller_keyboard()
			get_tree().set_input_as_handled()

func _unhandled_input(event):
	if modal != null or not ready or interrupted() or touch_editing:
		return
	if event is InputEventMouseMotion:
		var point = display_to_game(event.position)
		send({"action": "pointer", "kind": "move", "x": point.x, "y": point.y})
	elif event is InputEventMouseButton and event.button_index == BUTTON_LEFT:
		if event.pressed and not pointer_in_picture(event.position):
			return
		if not event.pressed and not pointer_pressed:
			return
		pointer_pressed = event.pressed
		send({"action": "pointer", "kind": "press" if event.pressed else "release", "x": pointer.x, "y": pointer.y, "shift": event.shift, "radius": interaction_radius(event)})
	elif event is InputEventMouseButton and event.button_index == BUTTON_RIGHT:
		# The original never used the second button: hold it to see what
		# in the room responds to a click.
		hotspot_mouse_held = event.pressed
	elif event is InputEventKey and event.pressed:
		if event.scancode == KEY_F12:
			show_keyboard(null)
			return
		if event.scancode == KEY_F10:
			show_menu()
			return
		if event.scancode >= KEY_F1 and event.scancode <= KEY_F9:
			send({"action": "gamma", "key": event.scancode - KEY_F1 + 1})
			return
		var keys = {KEY_UP: "uparrow", KEY_DOWN: "downarrow", KEY_LEFT: "leftarrow", KEY_RIGHT: "rightarrow", KEY_ESCAPE: ".", KEY_ENTER: "\r", KEY_KP_ENTER: "\r", KEY_BACKSPACE: "\b", KEY_TAB: "\t"}
		var key = keys.get(event.scancode, char(event.unicode).to_lower() if event.unicode > 0 else "")
		if not key.empty():
			send({"action": "key", "key": key, "special": event.scancode == KEY_ESCAPE or event.control})

func interaction_radius(event):
	if event.device == -1:
		return 14.0
	if controller_pointer_event or not OS.get_environment("RETANIC_ARCH").empty():
		return 10.0
	return 0.0

func mouse_button(pressed):
	var event = InputEventMouseButton.new()
	event.button_index = BUTTON_LEFT
	event.pressed = pressed
	event.position = display_pointer if adaptive_active or dialogue_active else pointer + game_origin
	event.global_position = event.position
	controller_pointer_event = true
	dispatch_pointer(event)
	controller_pointer_event = false

func dispatch_pointer(event):
	# Controller coordinates already belong to this viewport. Sending them
	# through global Input would apply the window's scaling a second time.
	# Godot 3 retains SceneTree's handled flag after a mapped button event.
	# Deliver game input directly so a held stick keeps moving after that event.
	_input(event)
	if modal == null:
		_unhandled_input(event)
		return
	var viewport = get_viewport()
	var local_handling = viewport.is_handling_input_locally()
	viewport.set_handle_input_locally(true)
	viewport.input(event)
	if not viewport.is_input_handled():
		viewport.unhandled_input(event)
	viewport.set_handle_input_locally(local_handling)

func process_controller(delta):
	if interrupted() or touch_editing:
		return
	if not remap_action.empty():
		if OS.get_ticks_msec() >= remap_deadline:
			cancel_controller_remap()
		return
	var pads = Input.get_connected_joypads()
	controller_poll -= delta
	if controller_poll <= 0 and (touch_enabled or not pads.empty() or not OS.get_environment("RETANIC_ARCH").empty()):
		refresh_controller_surface()
		controller_poll = 0.1
	var pad = -1 if pads.empty() else pads[0]
	var motion = Vector2.ZERO if pad < 0 else Vector2(Input.get_joy_axis(pad, JOY_AXIS_2), Input.get_joy_axis(pad, JOY_AXIS_3))
	if controller_neutral_required:
		var left = Vector2.ZERO if pad < 0 else Vector2(Input.get_joy_axis(pad, JOY_AXIS_0), Input.get_joy_axis(pad, JOY_AXIS_1))
		if motion.length() > 0.2 or left.length() > 0.4:
			return
		controller_neutral_required = false
	var deadzone = 0.2
	if motion.length() > deadzone:
		controller_selection = -1
		dialogue_preselected = false
		var speed = 65.0 if controller_action_held("precision") else 240.0
		var step = motion.normalized() * pow(min(1.0, (motion.length() - deadzone) / (1.0 - deadzone)), 1.5) * speed * delta
		if adaptive_active or dialogue_active:
			display_pointer = Vector2(clamp(display_pointer.x + step.x,0,layout_size.x-1),clamp(display_pointer.y + step.y,0,layout_size.y-1))
		else:
			pointer = Vector2(clamp(pointer.x + step.x, 0, 511), clamp(pointer.y + step.y, 0, 383))
		var event = InputEventMouseMotion.new()
		event.position = display_pointer if adaptive_active or dialogue_active else pointer + game_origin
		event.global_position = event.position
		event.relative = step
		dispatch_pointer(event)
	var movement = Vector2.ZERO if pad < 0 else Vector2(Input.get_joy_axis(pad, JOY_AXIS_0), Input.get_joy_axis(pad, JOY_AXIS_1))
	var key = ""
	if movement.length() > 0.4:
		if abs(movement.x) > abs(movement.y):
			key = "leftarrow" if movement.x < 0 else "rightarrow"
		else:
			key = "uparrow" if movement.y < 0 else "downarrow"
	for pair in [["up", "uparrow"], ["down", "downarrow"], ["left", "leftarrow"], ["right", "rightarrow"]]:
		if controller_action_held(pair[0]):
			key = pair[1]
	nav_timer -= delta
	if not key.empty() and (key != last_nav or nav_timer <= 0):
		controller_direction(key)
		nav_timer = 0.30 if key != last_nav else 0.15
	last_nav = key

func refresh_controller_surface(include_room = false):
	if runtime == null or runtime_failed or modal != null:
		return
	# Polled ten times a second; the answer is almost always the same text.
	var text = runtime.query("targets" if include_room else "controls")
	var cache = text + ("A" if adaptive_available else "a") + ("V" if adaptive_active else "v") + ("N" if notes_tray_shown() else "n")
	if cache == surface_cache:
		return
	var surface = JSON.parse(text).result
	if surface is Dictionary:
		set_controller_surface(surface)
		surface_cache = cache

func set_controller_surface(surface):
	# A surface set from anywhere else must not be mistaken for the engine's.
	surface_cache = ""
	if adaptive_available and surface.context == "room" and not surface.targets.empty():
		surface.targets = surface.targets.duplicate()
		surface.targets.append({"id":"ui:layout","aim_x":-1,"aim_y":-1,"w":32,"h":26})
	if notes_tray_shown() and surface.context == "room" and not surface.targets.empty():
		surface.targets = surface.targets.duplicate()
		surface.targets.append({"id":"ui:notes","aim_x":-1,"aim_y":-1,"w":44,"h":38})
	if adaptive_active:
		var visible_targets = []
		for target in surface.targets:
			if target.id != "prop:light":
				visible_targets.append(target)
		surface.targets = visible_targets
	if surface.context != "dialogue":
		surface.targets = surface.targets.duplicate()
		surface.targets.sort_custom(self,"controller_target_before")
	var same_place = controller_surface.context == surface.context and controller_surface.key == surface.key
	if same_place and surface.context == "room" and surface.targets.empty():
		return
	var old_id = ""
	if same_place and controller_selection >= 0 and controller_selection < controller_surface.targets.size():
		old_id = controller_surface.targets[controller_selection].id
	var new_replies = surface.context == "dialogue" and not surface.targets.empty() and (controller_surface.context != "dialogue" or target_ids(controller_surface) != target_ids(surface))
	var changed = JSON.print(controller_surface) != JSON.print(surface)
	controller_surface = surface
	update_touch_context()
	if new_replies or (dialogue_preselected and surface.context == "dialogue"):
		controller_selection = 0
		dialogue_preselected = true
		cursor_layer.update()
		return
	if not changed:
		return
	controller_selection = -1
	dialogue_preselected = false
	for i in range(surface.targets.size()):
		if surface.targets[i].id == old_id:
			controller_selection = i
	if controller_selection < 0 and surface.context != "room" and not surface.targets.empty():
		controller_selection = 0
	if controller_selection >= 0 and not pointer_pressed:
		focus_controller_target(controller_selection)
	cursor_layer.update()

func target_ids(surface):
	var ids = []
	for target in surface.get("targets", []):
		ids.append(target.get("id", ""))
	return ids

# The notes button belongs to the side panels; in Classic it sits in the
# margin below the layout switch, or on a 4:3 screen in the empty right end
# of the original toolbar. It is not needed in a conversation.
func notes_tray_rect():
	if dialogue_active or adaptive_metadata is Dictionary and adaptive_metadata.get("reason", "") == "conversation" and not adaptive_active:
		return Rect2()
	if adaptive_active:
		return adaptive.notes_display
	if game_origin.x >= 52:
		return Rect2(6, 56, min(84, game_origin.x - 12), 26)
	return Rect2(game_origin + Vector2(422, 344), Vector2(62, 26))

func notes_tray_shown():
	return is_instance_valid(scratchpad) and scratchpad.tray.visible

func controller_target_position(target):
	if target.id == "ui:layout":
		return layout_switch.rect_position+layout_switch.rect_size/2
	if target.id == "ui:notes":
		return scratchpad.tray_centre()
	return game_to_display(Vector2(target.aim_x,target.aim_y),target.id)

func controller_target_before(a,b):
	var first = controller_target_position(a)
	var second = controller_target_position(b)
	# Read across approximate rows, with an ID tie-break for overlapping items.
	var row_a = int(floor(first.y/24.0))
	var row_b = int(floor(second.y/24.0))
	if row_a != row_b:
		return row_a < row_b
	if first.x != second.x:
		return first.x < second.x
	if first.y != second.y:
		return first.y < second.y
	return a.id < b.id

func controller_spatial_target(key):
	if controller_selection < 0:
		return 0
	var direction = {"uparrow":Vector2.UP,"downarrow":Vector2.DOWN,"leftarrow":Vector2.LEFT,"rightarrow":Vector2.RIGHT}[key]
	var origin = controller_target_position(controller_surface.targets[controller_selection])
	var best = controller_selection
	var best_score = INF
	for index in range(controller_surface.targets.size()):
		if index == controller_selection:
			continue
		var offset = controller_target_position(controller_surface.targets[index])-origin
		var forward = offset.dot(direction)
		if forward <= 1.0:
			continue
		var across = abs(offset.cross(direction))
		# Prefer an item in the requested row/column over a closer diagonal.
		var score = forward + across*2.0 + across*across*4.0/forward
		if score < best_score:
			best = index
			best_score = score
	return best

func focus_controller_target(index):
	if index < 0 or index >= controller_surface.targets.size():
		return
	controller_selection = index
	dialogue_preselected = false
	set_pointer_visible(false)
	var target = controller_surface.targets[index]
	if target.id in ["ui:layout", "ui:notes"]:
		display_pointer = controller_target_position(target)
		pointer = display_pointer-game_origin
		cursor_layer.update()
		return
	pointer = Vector2(target.aim_x, target.aim_y)
	display_pointer = game_to_display(pointer,target.id)
	send({"action": "pointer", "kind": "move", "x": pointer.x, "y": pointer.y})
	cursor_layer.update()

func controller_direction(key):
	set_pointer_visible(false)
	if modal != null:
		controller_ui_key({"uparrow": KEY_UP, "downarrow": KEY_DOWN, "leftarrow": KEY_LEFT, "rightarrow": KEY_RIGHT}[key], true)
		controller_ui_key({"uparrow": KEY_UP, "downarrow": KEY_DOWN, "leftarrow": KEY_LEFT, "rightarrow": KEY_RIGHT}[key], false)
		return
	refresh_controller_surface()
	if controller_surface.context in ["dialogue", "movie", "panel"]:
		var count = controller_surface.targets.size()
		if count > 0 and not pointer_pressed:
			if controller_surface.context == "dialogue":
				var step = -1 if key in ["uparrow", "leftarrow"] else 1
				focus_controller_target(0 if controller_selection < 0 else (controller_selection + step + count) % count)
			else:
				focus_controller_target(controller_spatial_target(key))
		return
	send({"action": "key", "key": key})

func controller_cycle(step):
	if modal != null:
		controller_direction("uparrow" if step < 0 else "downarrow")
		return
	refresh_controller_surface(true)
	var count = controller_surface.targets.size()
	if count > 0 and not pointer_pressed:
		focus_controller_target((0 if step > 0 else count - 1) if controller_selection < 0 else (controller_selection + step + count) % count)

func controller_confirm(pressed):
	if modal != null:
		var control = get_focus_owner()
		if pressed and control is LineEdit:
			show_keyboard(control)
			return
		controller_ui_key(KEY_ENTER, pressed)
		return
	if pressed:
		refresh_controller_surface()
		if dialogue_preselected and controller_selection >= 0 and controller_selection < controller_surface.targets.size() and not pointer_pressed:
			# Answer with the first reply even though it was never shown.
			focus_controller_target(controller_selection)
		set_pointer_visible(controller_selection < 0)
		if controller_selection >= 0 and controller_surface.targets[controller_selection].id == "ui:layout":
			toggle_exploration_layout()
			return
		if controller_selection >= 0 and controller_surface.targets[controller_selection].id == "ui:notes":
			scratchpad.activate_tray()
			return
	mouse_button(pressed)

func set_pointer_visible(visible):
	if pointer_visible != visible:
		pointer_visible = visible
		cursor_layer.update()

func controller_ui_key(code, pressed):
	var event = InputEventAction.new()
	event.action = {KEY_ENTER: "ui_accept", KEY_UP: "ui_up", KEY_DOWN: "ui_down", KEY_LEFT: "ui_left", KEY_RIGHT: "ui_right"}[code]
	event.pressed = pressed
	Input.parse_input_event(event)

func controller_back():
	# A first-start question is answered with Start, not skipped.
	if is_instance_valid(helper_choice_dialog) and modal == helper_choice_dialog:
		return
	if is_instance_valid(scratchpad_options_dialog) and modal == scratchpad_options_dialog:
		close_scratchpad_options()
		return
	if is_instance_valid(dialogue_choice_dialog) and modal == dialogue_choice_dialog:
		if not dialogue_choice_first:
			finish_dialogue_options()
		return
	if is_instance_valid(achievement_choice_dialog) and modal == achievement_choice_dialog:
		if not achievement_choice_first:
			achievement_choice_dialog = null
			show_settings()
		return
	if is_instance_valid(artwork_choice_dialog) and modal == artwork_choice_dialog:
		if not artwork_choice_first:
			artwork_choice_dialog = null
			show_hd_settings()
		return
	if touch_editing:
		finish_touch_positions()
		return
	if remap_open:
		controller_settings_back()
		return
	var control = get_focus_owner()
	while is_instance_valid(control):
		if control is Popup and control.visible:
			control.hide()
			return
		control = control.get_parent()
	if virtual_keyboard != null:
		close_keyboard()
	elif menu != null:
		resume_game()
	elif current_dialog != -1:
		reply_dialog(null)
	elif modal != null:
		if patch_busy:
			patch_cancel()
		elif runtime != null:
			note_closed()
	else:
		send({"action": "key", "key": ".", "special": true})

func controller_keyboard():
	if virtual_keyboard != null:
		close_keyboard()
	elif modal == null:
		show_keyboard(null)
	elif current_dialog != -1 and is_instance_valid(save_name):
		show_keyboard(save_name)

# Store native indices separately for Godot 3 and 4, whose button numbers differ.
func controller_config_section():
	return "controller_" + str(Engine.get_version_info().major)

func default_controller_bindings():
	return {
		"confirm": ["b:" + str(JOY_BUTTON_0), "b:" + str(JOY_R), "k:" + str(KEY_ENTER), "k:" + str(KEY_KP_ENTER)],
		"back": ["b:" + str(JOY_BUTTON_1), "k:" + str(KEY_ESCAPE)],
		"menu": ["b:" + str(JOY_START), "k:" + str(KEY_F10)],
		"door": ["b:" + str(JOY_BUTTON_2), "k:" + str(KEY_SPACE)],
		"keyboard": ["b:" + str(JOY_BUTTON_3), "k:" + str(KEY_F12)],
		"mouse": ["b:" + str(JOY_R3)], "precision": ["b:" + str(JOY_L)], "hints": ["b:" + str(JOY_L3), "k:" + str(KEY_HOME)],
		"previous": ["t:0", "k:" + str(KEY_PAGEUP)], "next": ["t:1", "k:" + str(KEY_PAGEDOWN)],
		"up": ["b:" + str(JOY_DPAD_UP), "k:" + str(KEY_UP)],
		"down": ["b:" + str(JOY_DPAD_DOWN), "k:" + str(KEY_DOWN)],
		"left": ["b:" + str(JOY_DPAD_LEFT), "k:" + str(KEY_LEFT)],
		"right": ["b:" + str(JOY_DPAD_RIGHT), "k:" + str(KEY_RIGHT)]}

func load_controller_bindings():
	controller_bindings = default_controller_bindings()
	var saved = config.get_value(controller_config_section(), "bindings", {})
	if saved is Dictionary:
		for action in controller_bindings:
			if saved.get(action) is Array:
				controller_bindings[action] = saved[action].duplicate()

func controller_binding_input(event):
	var token = ""
	var pressed = false
	if event is InputEventJoypadButton:
		token = "b:" + str(event.button_index)
		pressed = event.pressed
	elif event is InputEventJoypadMotion and event.axis in [JOY_AXIS_6, JOY_AXIS_7]:
		var index = 0 if event.axis == JOY_AXIS_6 else 1
		pressed = event.axis_value > 0.5
		if pressed == controller_triggers[index]:
			return true
		controller_triggers[index] = pressed
		token = "t:" + str(index)
	elif event is InputEventKey:
		token = "k:" + str(event.scancode)
		pressed = event.pressed
	else:
		return false
	var held_action = controller_held.get(token, "")
	if not pressed:
		controller_held.erase(token)
	if token == remap_release:
		if not pressed:
			remap_release = ""
		return true
	if not remap_action.empty():
		if pressed and not (event is InputEventKey and event.echo):
			assign_controller_binding(remap_action, token)
			remap_release = token
			cancel_controller_remap()
		return true
	if event is InputEventKey:
		# Finish a keyboard click even if its reply vanished or focus changed.
		if held_action == "confirm":
			if not pressed:
				controller_action("confirm", false)
				return true
			if event.echo:
				return true
		# Keep normal typing and keyboard shortcuts in text fields.
		if get_focus_owner() is LineEdit or event.control or event.meta or event.alt:
			return false
		# Keyboard arrows also cover Android controllers that report a D-pad as keys.
		var portable = not OS.get_environment("RETANIC_ARCH").empty() or OS.get_name() == "Android"
		var navigation = event.scancode in [KEY_UP, KEY_DOWN, KEY_LEFT, KEY_RIGHT]
		if event.scancode in [KEY_ENTER, KEY_KP_ENTER] and modal == null and event.pressed:
			# New replies may have arrived since the last key; they start on the first.
			refresh_controller_surface()
		var confirmation = event.scancode in [KEY_ENTER, KEY_KP_ENTER] and modal == null and controller_selection >= 0
		var custom = config.get_value(controller_config_section(), "keyboard_remapped", false)
		var hinting = token in controller_bindings.get("hints", [])
		if not portable and not navigation and not confirmation and not custom and not hinting:
			return false
	for action in controller_bindings:
		if token in controller_bindings[action]:
			if pressed:
				controller_held[token] = action
				set_pointer_visible(false)
			else:
				controller_held.erase(token)
			if event is InputEventKey and event.echo:
				return true
			controller_action(action, pressed)
			return true
	# Consume old navigation bindings, but leave typing available for game puzzles.
	if event is InputEventKey:
		return event.scancode in [KEY_UP, KEY_DOWN, KEY_LEFT, KEY_RIGHT, KEY_ENTER, KEY_KP_ENTER, KEY_ESCAPE, KEY_F10, KEY_F12, KEY_SPACE, KEY_PAGEUP, KEY_PAGEDOWN]
	# Unassigned native buttons must not activate Godot's default UI bindings.
	return true

func controller_action_held(action):
	return action in controller_held.values()

func controller_action(action, pressed):
	if touch_editing:
		if pressed and action in ["back","menu"]:
			finish_touch_positions()
		return
	if action == "confirm":
		controller_confirm(pressed)
	elif action == "mouse":
		if pressed:
			set_pointer_visible(true)
		mouse_button(pressed)
	elif pressed:
		match action:
			"back": controller_back()
			"menu": show_menu()
			"keyboard": controller_keyboard()
			"previous": controller_cycle(-1)
			"next": controller_cycle(1)
			"door":
				if modal == null:
					send({"action": "key", "key": " "})
			"up", "down", "left", "right":
				last_nav = action + "arrow"
				nav_timer = 0.30
				controller_direction(last_nav)

func controller_binding_label(token):
	if token.begins_with("k:"):
		return OS.get_scancode_string(int(token.substr(2)))
	if token.begins_with("t:"):
		return "L2" if token == "t:0" else "R2"
	var names = {JOY_BUTTON_0: "A", JOY_BUTTON_1: "B", JOY_BUTTON_2: "X", JOY_BUTTON_3: "Y",
		JOY_START: "Start", JOY_R: "R1", JOY_L: "L1", JOY_R3: "R3",
		JOY_DPAD_UP: "D-pad up", JOY_DPAD_DOWN: "D-pad down", JOY_DPAD_LEFT: "D-pad left", JOY_DPAD_RIGHT: "D-pad right"}
	var index = int(token.substr(2))
	return names.get(index, "Button " + str(index))

func add_layout_preview(box):
	var preview = load("res://scripts/layout_preview.gd").new()
	preview.player = self
	box.add_child(preview)

func show_first_layout():
	var box = panel("Choose your layout")
	add_layout_preview(box)
	var hint = Label.new()
	hint.text = "Side panels work on wide screens. Full-screen movies and close-ups use Classic. Change your choice later in Menu > Settings > Controls / display."
	hint.autowrap = true
	hint.rect_min_size = Vector2(390,64)
	hint.add_font_override("font",get_font_for("13px Arial"))
	box.add_child(hint)
	var supported = not classic_format_device()
	var classic = button(box,"Classic" if supported else "Classic (default)","choose_first_layout",[0])
	var compact = button(box,"Side panels: Compact (default)" if supported else "Side panels: Compact","choose_first_layout",[1])
	var roomy = button(box,"Side panels: Roomy","choose_first_layout",[2])
	compact.disabled = not supported
	roomy.disabled = not supported
	# Match the highlighted preview, including an explicit saved preference.
	var selected = 0 if not supported or not adaptive_enabled() else (2 if adaptive_roomy() else 1)
	[classic,compact,roomy][selected].grab_focus()

func choose_first_layout(choice):
	config.set_value("graphics","adaptive_exploration",choice != 0 and not classic_format_device())
	config.set_value("graphics","adaptive_roomy",choice == 2)
	config.set_value("graphics","layout_chosen",true)
	config.save("user://settings.cfg")
	close_modal()
	update_touch_layout()
	boot_configured_game()

func show_control_options():
	var box = panel("Controls / display")
	if not classic_format_device():
		button(box,"Adaptive exploration: %s" % ("On" if adaptive_enabled() else "Off"),"toggle_adaptive_exploration").grab_focus()
		button(box,"Panel size: " + ("Roomy" if adaptive_roomy() else "Compact"),"toggle_adaptive_size")
		add_layout_preview(box)
		text_scroller(box,"Side panels need a wide screen. Full-screen movies and close-ups use Classic. Dialogue has its own layout setting.",40)
	var row = HBoxContainer.new()
	box.add_child(row)
	var bindings = button(row,"Controller bindings","show_controller_settings")
	if classic_format_device():
		bindings.grab_focus()
	button(row,"Touch controls","show_touch_help")
	button(box,"Dialogue / subtitles","show_dialogue_options")
	button(box,"Back","show_settings")

func toggle_adaptive_size():
	config.set_value("graphics","adaptive_roomy",not adaptive_roomy())
	config.save("user://settings.cfg")
	update_touch_layout()
	show_control_options()

func toggle_adaptive_exploration():
	config.set_value("graphics","adaptive_exploration",not adaptive_enabled())
	config.save("user://settings.cfg")
	update_touch_layout()
	sync_adaptive_layout()
	show_control_options()

func show_touch_help():
	var box = panel("Touch controls")
	var appearance = config.get_value("touch","joystick_style","full")
	button(box,"Joystick: " + {"full":"Ring and knob","knob":"Knob only","hidden":"Invisible"}.get(appearance,"Ring and knob"),"cycle_touch_style")
	button(box,"Control opacity: %d%%" % int(config.get_value("touch","opacity",0.5)*100),"cycle_touch_opacity")
	button(box,"Edit positions and size","edit_touch_positions")
	add_touch_preview(box)
	text_scroller(box,"Drag controls to move them. Drag the joystick's corner handle to resize it, then choose Done / lock. Each layout keeps its own positions and size. Invisible parts still respond to touch. Tap the joystick center to open a door.",70)
	button(box,"Back","show_control_options").grab_focus()

func add_touch_preview(box):
	var preview = Control.new()
	preview.rect_min_size = Vector2(396,60)
	preview.mouse_filter = Control.MOUSE_FILTER_IGNORE
	box.add_child(preview)
	var joystick = load("res://scripts/touch_joystick.gd").new()
	preview.add_child(joystick)
	joystick.set_process_input(false)
	joystick.configure(Vector2(52,30),28)
	joystick.appearance = config.get_value("touch","joystick_style","full")
	joystick.modulate.a = config.get_value("touch","opacity",0.5)
	var example = load("res://scripts/touch_button.gd").new()
	example.name = "space"
	example.position = Vector2(112,4)
	example.modulate.a = joystick.modulate.a
	preview.add_child(example)
	example.set_process_input(false)
	var label = Label.new()
	label.text = "Control preview"
	label.rect_position = Vector2(188,22)
	label.add_font_override("font",get_font_for("12px Arial"))
	preview.add_child(label)

func cycle_touch_style():
	var styles = ["full","knob","hidden"]
	var current = styles.find(config.get_value("touch","joystick_style","full"))
	config.set_value("touch","joystick_style",styles[(current+1)%3])
	config.save("user://settings.cfg")
	update_touch_layout()
	show_touch_help()

func cycle_touch_opacity():
	var opacity = config.get_value("touch","opacity",0.5) + 0.25
	config.set_value("touch","opacity",0.25 if opacity > 1.0 else opacity)
	config.save("user://settings.cfg")
	update_touch_layout()
	show_touch_help()

func touch_layout_key():
	return "dialogue" if dialogue_active else ("adaptive" if adaptive_active else ("portrait" if OS.window_size.y > OS.window_size.x else "landscape"))

func touch_control_bounds(control):
	var node = touch_strip.get_node(control)
	return Rect2(node.position-Vector2.ONE*node.radius,Vector2.ONE*node.radius*2) if control == "joystick" else Rect2(node.position,Vector2(52,52))

func move_touch_control(control, center):
	var node = touch_strip.get_node(control)
	var half = Vector2.ONE * node.radius if control == "joystick" else Vector2(26,26)
	center = Vector2(clamp(center.x,half.x+4,layout_size.x-half.x-4),clamp(center.y,half.y+48,layout_size.y-half.y-4))
	node.position = center if control == "joystick" else center-half
	var positions = config.get_value("touch_positions",touch_layout_key(),{}).duplicate()
	positions[control] = center/layout_size
	config.set_value("touch_positions",touch_layout_key(),positions)

func resize_touch_joystick(size, bottom_left):
	if not touch_editing:
		return
	var limit = min(96.0,min((layout_size.x-bottom_left.x-4)/2.0,(bottom_left.y-48)/2.0))
	var joystick = touch_strip.get_node("joystick")
	joystick.radius = clamp(size,min(32.0,limit),limit)
	config.set_value("touch_sizes",touch_layout_key(),joystick.radius)
	move_touch_control("joystick",bottom_left+Vector2(joystick.radius,-joystick.radius))
	joystick.update()

func apply_touch_preferences():
	var positions = config.get_value("touch_positions",touch_layout_key(),{})
	var joystick = touch_strip.get_node("joystick")
	if config.has_section_key("touch_sizes",touch_layout_key()):
		joystick.radius = clamp(float(config.get_value("touch_sizes",touch_layout_key(),58.0)),32.0,96.0)
	for control in ["joystick","menu","space","escape","keyboard"]:
		var node = touch_strip.get_node(control)
		if positions is Dictionary and positions.get(control) is Vector2:
			move_touch_control(control,positions[control]*layout_size)
		node.modulate.a = 1.0 if touch_editing else clamp(float(config.get_value("touch","opacity",0.5)),0.25,1.0)
		if control != "joystick":
			node.action = "" if touch_editing else "touch_"+control
	joystick.appearance = config.get_value("touch","joystick_style","full")
	joystick.editing = touch_editing
	joystick.update()

func edit_touch_positions():
	close_modal()
	menu = null
	manual_pause = true
	send({"action":"pause","on":true})
	sync_adaptive_layout()
	touch_editing = true
	clear_interrupted_input()
	update_touch_layout()
	touch_editor = load("res://scripts/touch_editor.gd").new()
	touch_editor.player = self
	touch_editor.connect("finished",self,"finish_touch_positions")
	touch_editor.connect("reset_requested",self,"reset_touch_positions")
	add_child(touch_editor)

func reset_touch_positions():
	config.set_value("touch_positions",touch_layout_key(),{})
	if config.has_section_key("touch_sizes",touch_layout_key()):
		config.erase_section_key("touch_sizes",touch_layout_key())
	update_touch_layout()

func finish_touch_positions():
	touch_editing = false
	if is_instance_valid(touch_editor):
		touch_editor.queue_free()
	touch_editor = null
	config.save("user://settings.cfg")
	clear_interrupted_input()
	update_touch_layout()
	show_touch_help()

func show_controller_settings():
	var box = panel("Controller settings")
	remap_open = true
	var hint = Label.new()
	hint.text = "Choose an action, then press its new button.\nLeft stick moves. Right stick controls the pointer."
	hint.add_font_override("font", get_font_for("12px Arial"))
	box.add_child(hint)
	var scroll = ScrollContainer.new()
	scroll.rect_min_size = Vector2(390, 158)
	scroll.follow_focus = true
	scroll.scroll_horizontal_enabled = false
	box.add_child(scroll)
	var rows = VBoxContainer.new()
	rows.size_flags_horizontal = SIZE_EXPAND_FILL
	scroll.add_child(rows)
	remap_buttons.clear()
	for pair in CONTROLLER_ACTIONS:
		remap_buttons[pair[0]] = button(rows, "", "begin_controller_remap", [pair[0]])
	remap_status = Label.new()
	remap_status.add_font_override("font", get_font_for("12px Arial"))
	remap_status.rect_min_size = Vector2(390, 30)
	remap_status.autowrap = true
	box.add_child(remap_status)
	var actions = HBoxContainer.new()
	box.add_child(actions)
	button(actions, "Cancel binding", "cancel_controller_remap")
	button(actions, "Reset defaults", "reset_controller_bindings")
	button(actions, "Help", "show_controller_help")
	button(actions, "Back", "controller_settings_back")
	update_controller_remap_labels()
	remap_buttons.confirm.grab_focus()

func update_controller_remap_labels():
	var portable = not OS.get_environment("RETANIC_ARCH").empty()
	for pair in CONTROLLER_ACTIONS:
		var names = []
		for token in controller_bindings[pair[0]]:
			if not portable or token.begins_with("k:"):
				names.append(controller_binding_label(token))
		remap_buttons[pair[0]].text = pair[1] + ": " + (PoolStringArray(names).join(" / ") if not names.empty() else "Unassigned")
		if portable and pair[0] in ["mouse", "precision"]:
			remap_buttons[pair[0]].text = pair[1] + ": set by PortMaster"
			remap_buttons[pair[0]].disabled = true
	remap_status.text = "Changes are saved automatically. Shared buttons swap actions."
	if portable:
		remap_status.text = "PortMaster remaps emitted keys. Buttons sharing a key change together."

func begin_controller_remap(action):
	controller_held.clear()
	remap_action = action
	remap_deadline = OS.get_ticks_msec() + 10000
	remap_status.text = "Press a button, trigger or key (10 seconds)."

func cancel_controller_remap():
	var action = remap_action
	remap_action = ""
	if remap_open:
		update_controller_remap_labels()
		if remap_buttons.has(action):
			remap_buttons[action].grab_focus()

func assign_controller_binding(action, token):
	var keyboard = token.begins_with("k:")
	var old = ""
	var retained = []
	for binding in controller_bindings[action]:
		if binding.begins_with("k:") == keyboard:
			if old.empty():
				old = binding
		else:
			retained.append(binding)
	for other in controller_bindings:
		if other != action and token in controller_bindings[other]:
			controller_bindings[other].erase(token)
			if not old.empty():
				controller_bindings[other].append(old)
	retained.append(token)
	controller_bindings[action] = retained
	if keyboard:
		config.set_value(controller_config_section(), "keyboard_remapped", true)
	save_controller_bindings()

func save_controller_bindings():
	config.set_value(controller_config_section(), "bindings", controller_bindings)
	config.save("user://settings.cfg")

func reset_controller_bindings():
	controller_held.clear()
	controller_bindings = default_controller_bindings()
	config.set_value(controller_config_section(), "keyboard_remapped", false)
	save_controller_bindings()
	cancel_controller_remap()

func controller_settings_back():
	close_modal()
	menu = null
	show_menu()

func show_controller_help():
	var box = panel("Controller controls")
	text_scroller(box, "Left stick / D-pad: move, or select dialogue replies and menus.\nA / R1: confirm or click. Hold to drag.\nB: back or skip speech / movies.\nL2 / R2: previous / next clickable target.\nRight stick: mouse pointer. Hold L1 for precision.\nX: door / Space. Y: keyboard.\nL3 (press the left stick): hold to mark what you can click.\nStart: voyage menu.\n\nUse the D-pad and A on the keyboard to enter save names or solve text puzzles.", 230)
	button(box, "Back", "show_controller_settings").grab_focus()

func panel(title):
	if touch_editing:
		touch_editing = false
		if is_instance_valid(touch_editor):
			touch_editor.queue_free()
		config.save("user://settings.cfg")
		update_touch_layout()
	if pointer_pressed:
		send({"action": "pointer", "kind": "release", "x": pointer.x, "y": pointer.y})
		pointer_pressed = false
	close_modal()
	var p = PanelContainer.new()
	p.rect_position = (layout_size - Vector2(420, 310)) / 2
	p.rect_min_size = Vector2(420, 310)
	p.add_font_override("font", get_font_for("14px Arial"))
	p.connect("resized", self, "center_panel", [p])
	add_child(p)
	var margin = MarginContainer.new()
	for side in ["left", "right", "top", "bottom"]:
		margin.add_constant_override("margin_" + side, 12)
	p.add_child(margin)
	var box = VBoxContainer.new()
	box.add_constant_override("separation", 7)
	margin.add_child(box)
	var label = Label.new()
	label.text = title
	label.add_font_override("font", get_font_for("16px Arial"))
	box.add_child(label)
	modal = p
	hotspot_mouse_held = false
	if is_instance_valid(backdrop):
		backdrop.rect_size = layout_size
		backdrop.visible = has_frame
	controller_selection = -1
	dialogue_preselected = false
	cursor_layer.update()
	touch_strip.hide()
	if is_instance_valid(layout_switch):
		layout_switch.hide()
	if runtime == null:
		status.hide()
	return box

func center_panel(p):
	p.rect_position = (layout_size - p.rect_size) / 2

func button(box, title, method, args = []):
	var b = Button.new()
	b.text = title
	b.rect_min_size.y = 25
	b.add_font_override("font", get_font_for("14px Arial"))
	box.add_child(b)
	b.connect("pressed", self, method, args)
	return b

func close_modal():
	remap_open = false
	remap_action = ""
	if is_instance_valid(modal):
		modal.queue_free()
	modal = null
	if is_instance_valid(backdrop):
		backdrop.hide()
	if touch_strip != null:
		touch_strip.visible = touch_enabled

func show_note(text):
	var box = panel("Titanic")
	text_scroller(box, text, 210)
	button(box, "OK", "note_closed").grab_focus()

func note_closed():
	close_modal()
	if current_dialog == -1:
		manual_pause = false
		send({"action": "pause", "on": not focused})
	if current_dialog != -1:
		reply_dialog(null)

func show_menu():
	if menu != null:
		resume_game()
		return
	if modal != null:
		return
	send({"action": "pause", "on": true})
	manual_pause = true
	var box = panel("Titanic — Voyage menu")
	menu = modal
	button(box, "Resume", "resume_game").grab_focus()
	button(box, "Quick save", "menu_command", ["quicksave"])
	button(box, "Save game", "menu_command", ["save"])
	button(box, "Load game", "menu_command", ["load"])
	if achievements_enabled():
		button(box, "Achievements", "show_achievements")
	if scratchpad_enabled():
		button(box, "Scratchpad", "open_scratchpad_from_menu")
	button(box, "Import / export .ti saves", "save_tools")
	button(box, "Settings", "show_settings")
	button(box, "Main menu", "menu_command", ["new"])
	button(box, "Quit", "quit_game")

func show_settings():
	var box = panel("Settings")
	button(box, "Picture / sound", "show_display_options").grab_focus()
	button(box, "Controls / display", "show_control_options")
	button(box, "Game files / mods", "setup_from_menu")
	button(box, "Achievements: %s" % ("On" if achievements_enabled() else "Off"), "show_achievement_choice")
	button(box, "Scratchpad: %s" % scratchpad_summary(), "toggle_scratchpad")
	button(box, "New items: %s" % ("Into the bag, with a notice" if item_notices_enabled() else "In hand (original)"), "toggle_item_notices")
	button(box, "Credits", "show_credits")
	button(box, "Back", "controller_settings_back")

func desktop_window():
	return not OS.get_name() in ["Android", "iOS", "HTML5", "Web"] and OS.get_environment("RETANIC_ARCH").empty()

func set_fullscreen(on):
	OS.window_fullscreen = on
	config.set_value("display", "fullscreen", on)
	config.save("user://settings.cfg")

func default_frame_rate():
	return 30 if not OS.get_environment("RETANIC_ARCH").empty() else 60

func master_volume():
	return clamp(float(config.get_value("audio", "master", 1.0)), 0.0, 1.0)

func apply_master_volume():
	var level = master_volume()
	AudioServer.set_bus_mute(0, level <= 0.001)
	AudioServer.set_bus_volume_db(0, linear2db(max(0.001, level)))

func apply_picture_filter():
	var smooth = config.get_value("graphics", "smooth_picture", false)
	if Engine.get_version_info().major >= 4:
		set("texture_filter", 2 if smooth else 1)
	else:
		frame_texture.set("flags", 4 if smooth else 0)
	update()

func apply_display_settings():
	if desktop_window() and config.get_value("display", "fullscreen", false):
		OS.window_fullscreen = true
	Engine.target_fps = int(config.get_value("display", "frame_rate", default_frame_rate()))
	apply_master_volume()

func hotspot_mode():
	return config.get_value("accessibility", "hotspots", "hold")

func show_display_options(focus = 0):
	var box = panel("Picture / sound")
	var buttons = []
	if desktop_window():
		buttons.append(button(box, "Fullscreen: %s" % ("On" if OS.window_fullscreen else "Off"), "toggle_display_option", ["fullscreen", buttons.size()]))
	buttons.append(button(box, "Picture: %s" % ("Smooth" if config.get_value("graphics", "smooth_picture", false) else "Sharp pixels"), "toggle_display_option", ["smooth", buttons.size()]))
	buttons.append(button(box, "Frame rate: %s" % ("30 (battery saver)" if Engine.target_fps == 30 else "60"), "toggle_display_option", ["frame_rate", buttons.size()]))
	var row = HBoxContainer.new()
	box.add_child(row)
	var quieter = button(row, "Volume -", "toggle_display_option", ["quieter", buttons.size()])
	quieter.size_flags_horizontal = SIZE_EXPAND_FILL
	buttons.append(quieter)
	var level = Label.new()
	level.name = "Volume"
	level.text = "  %d%%  " % int(round(master_volume() * 100))
	level.add_font_override("font", get_font_for("14px Arial"))
	row.add_child(level)
	var louder = button(row, "Volume +", "toggle_display_option", ["louder", buttons.size()])
	louder.size_flags_horizontal = SIZE_EXPAND_FILL
	buttons.append(louder)
	buttons.append(button(box, "Hotspot hints: %s" % {"hold": "While held", "always": "Always", "off": "Off"}[hotspot_mode()], "toggle_display_option", ["hotspots", buttons.size()]))
	var hint = Label.new()
	hint.text = "Hold the right mouse button, Home or the left stick button to mark what you can click. Always suits touchscreens.\nBrightness: F1 darker, F2 brighter, F9 reset."
	hint.autowrap = true
	hint.rect_min_size.x = 390
	hint.add_font_override("font", get_font_for("12px Arial"))
	hint.add_color_override("font_color", Color("a39a84"))
	box.add_child(hint)
	buttons.append(button(box, "Back", "show_settings"))
	buttons[clamp(focus, 0, buttons.size() - 1)].grab_focus()

func toggle_display_option(key, focus):
	match key:
		"fullscreen":
			set_fullscreen(not OS.window_fullscreen)
		"smooth":
			config.set_value("graphics", "smooth_picture", not config.get_value("graphics", "smooth_picture", false))
			apply_picture_filter()
		"frame_rate":
			Engine.target_fps = 60 if Engine.target_fps == 30 else 30
			config.set_value("display", "frame_rate", Engine.target_fps)
		"quieter", "louder":
			config.set_value("audio", "master", clamp(round(master_volume() * 10 + (1 if key == "louder" else -1)) / 10.0, 0.0, 1.0))
			apply_master_volume()
		"hotspots":
			config.set_value("accessibility", "hotspots", {"hold": "always", "always": "off", "off": "hold"}[hotspot_mode()])
	config.save("user://settings.cfg")
	show_display_options(focus)

# Marks are gathered a few times a second, and only while they are wanted.
func hotspots_wanted():
	if modal != null or runtime == null or runtime_failed or not ready or touch_editing or interrupted():
		return false
	var mode = hotspot_mode()
	return mode == "always" or (mode == "hold" and (hotspot_mouse_held or controller_action_held("hints")))

func process_hotspots(delta):
	if not hotspots_wanted():
		if not hotspots.empty() or not hotspot_text.empty():
			hotspots = []
			hotspot_text = ""
			cursor_layer.update()
		hotspot_poll = 0.0
		return
	hotspot_poll -= delta
	if hotspot_poll <= 0:
		hotspot_poll = 0.25 if hotspot_mode() == "hold" else 0.5
		var text = runtime.query("targets")
		if text != hotspot_text:
			hotspot_text = text
			var surface = JSON.parse(text).result
			hotspots = []
			if surface is Dictionary and surface.context in ["room", "panel", "movie"]:
				for target in surface.targets:
					if not target.id in HOTSPOT_TOOLBAR:
						hotspots.append(target)
			cursor_layer.update()
	# Only the held marks breathe; permanent ones stay still, so an idle
	# room still costs nothing to draw.
	if hotspot_mode() == "hold" and not hotspots.empty():
		hotspot_phase = fmod(hotspot_phase + delta, 1.2)
		cursor_layer.update()

func draw_hotspots():
	var breath = 0.5 - 0.5 * cos(hotspot_phase / 1.2 * TAU) if hotspot_mode() == "hold" else 0.0
	for target in hotspots:
		var center = game_to_display(Vector2(target.aim_x, target.aim_y), target.id) - cursor_layer.position
		var radius = 7.0 + 2.5 * breath
		cursor_layer.draw_arc(center, radius + 1.2, 0, TAU, 32, Color(0, 0, 0, 0.55), 2.4, true)
		cursor_layer.draw_arc(center, radius, 0, TAU, 32, Color(0.97, 0.90, 0.64, 0.95 - 0.35 * breath), 1.4, true)
		cursor_layer.draw_circle(center, 1.8, Color(0.97, 0.90, 0.64, 0.9))

func quick_load():
	if modal != null or runtime == null:
		return
	var name = saves.newest()
	send({"action": "pause", "on": true})
	manual_pause = true
	if name.empty():
		var empty = panel("No saved games yet")
		button(empty, "OK", "resume_game").grab_focus()
		return
	var box = panel("Load the latest save?")
	text_scroller(box, saves.display_name(name) + "\n" + saves.describe_time(saves.modified_time(name)) + "\n\nProgress since your last save will be lost.", 90)
	button(box, "Load", "quick_load_confirmed", [name]).grab_focus()
	button(box, "Cancel", "resume_game")

func quick_load_confirmed(name):
	resume_game()
	send({"action": "load", "path": saves.path_for(name)})

func resume_game():
	manual_pause = false
	close_modal()
	menu = null
	if is_instance_valid(close_request_dialog):
		modal = close_request_dialog
		modal.show()
		manual_pause = close_dialog_pause
	close_request_dialog = null
	send({"action": "pause", "on": manual_pause or not focused})

func menu_command(action):
	resume_game()
	send({"action": action})

func quit_game():
	get_tree().quit()

func setup_from_menu():
	resume_game()
	send({"action": "pause", "on": true})
	show_setup()

func show_setup():
	manual_pause = true
	send({"action": "pause", "on": true})
	var box = panel("Titanic: Game files")
	var label = Label.new()
	label.text = "Choose a folder with cd1 and cd2 ISOs, extracted discs,\nor your GOG / Steam game files.\nChanging files restarts the game; save first."
	label.add_font_override("font", get_font_for("13px Arial"))
	box.add_child(label)
	button(box, "Choose game folder", "choose_data", [0]).grab_focus()
	button(box, "Choose discs separately", "choose_data", [1])
	button(box, "M3tox patches", "show_patch_picker", [false])
	button(box, "Autosave checkpoints: %s" % ("On" if config.get_value("saves", "autosave_enabled", true) else "Off"), "toggle_autosave")
	button(box, "HD artwork", "show_hd_settings")
	button(box, "Choose external mod folder", "choose_mods")
	button(box, "Disable external mods", "disable_mods")
	button(box, "Back", "leave_setup")

func leave_setup():
	manual_pause = false
	close_modal()
	send({"action": "pause", "on": not focused})

func file_dialog(mode, callback, args = [], filter_text = ""):
	var d = FileDialog.new()
	d.access = FileDialog.ACCESS_FILESYSTEM
	d.mode = mode
	d.rect_min_size = Vector2(490, 350)
	if not filter_text.empty():
		d.add_filter(filter_text)
	add_child(d)
	d.connect("dir_selected" if mode == FileDialog.MODE_OPEN_DIR else "file_selected", self, callback, args)
	d.connect("popup_hide", d, "queue_free")
	d.popup_centered(Vector2(490, 350))
	return d

func choose_data(disc):
	if OS.get_name() == "Android" and Engine.has_singleton("TitanicFiles"):
		android_selected_disc = disc
		var box = panel("Importing game files")
		text_scroller(box, "Choose the folder containing both cd1 and cd2 ISOs, extracted discs, or GOG / Steam files. Copying can take a few minutes. Keep Titanic open until it finishes.", 210)
		Engine.get_singleton("TitanicFiles").import_game(ProjectSettings.globalize_path("user://"))
		return
	file_dialog(FileDialog.MODE_OPEN_DIR, "data_selected", [disc])

func data_selected(path, disc):
	if disc == 1:
		config.set_value("pending", "disc1", path)
		choose_data(2)
		return
	var roots = files.discover(path) if disc == 0 else [config.get_value("pending", "disc1", ""), path]
	if roots.size() != 2 or not prepare_index(roots):
		show_note(files.error if not files.error.empty() else "Select a folder with cd1 and cd2 ISOs, extracted discs, or GOG / Steam files.")
		return
	config.set_value("game", "root", path if disc == 0 else "")
	config.set_value("game", "disc1", roots[0])
	config.set_value("game", "disc2", roots[1])
	config.save("user://settings.cfg")
	start_runtime()

func show_hd_settings():
	var box = panel("HD artwork")
	var hint = Label.new()
	hint.text = "Keep hdpack beside your ISOs, or choose its folder.\nChanges apply at once, also in a running game."
	hint.add_font_override("font", get_font_for("13px Arial"))
	box.add_child(hint)
	button(box, "Choose HD artwork folder", "choose_hd_folder").grab_focus()
	button(box, "Use automatic folder detection", "reset_hd_folder")
	button(box, "HD artwork: %s" % ("On" if config.get_value("graphics", "hd_enabled", true) else "Off"), "toggle_hd_artwork")
	button(box, "Redrawn interface: %s (compare)" % ("On" if config.get_value("graphics", "redrawn_ui", true) else "Off"), "show_artwork_choice")
	if hd_pack_path.empty():
		var missing = Label.new()
		missing.text = "No HD pack detected. World artwork stays original."
		missing.add_font_override("font", get_font_for("13px Arial"))
		box.add_child(missing)
	button(box, "Back", "show_setup")

func save_file_path(save_path):
	if save_path.begins_with("save:"):
		return saves.path_for(save_path.substr(5))
	return save_path

func achievements_enabled():
	return config.get_value("achievements", "enabled", true)

func needs_achievement_choice():
	return not config.get_value("achievements", "chosen", false)

# Asked once on first start; Settings can change it at any time.
func show_achievement_choice(save_path = "", first_start = false):
	achievement_choice_first = first_start
	achievement_choice_save = save_path
	var box = panel("Achievements")
	achievement_choice_dialog = modal
	status.hide()
	var hint = Label.new()
	hint.text = "Earn achievements for the story's turning points, conversations, puzzles, rooms and endings. A short notice appears in the top right corner when you unlock one.\n\nThey never change the game or your saves. You can change this later in Settings."
	if not first_start:
		hint.text = "Achievements you have already earned are kept either way. Turning them back on awards what your current game has already reached."
	hint.autowrap = true
	hint.rect_min_size = Vector2(396, 110)
	hint.add_font_override("font", get_font_for("13px Arial"))
	box.add_child(hint)
	var on = button(box, "Play with achievements", "choose_achievements", [true])
	var off = button(box, "Play without achievements", "choose_achievements", [false])
	if first_start or achievements_enabled():
		on.grab_focus()
	else:
		off.grab_focus()
	if not first_start:
		button(box, "Back", "show_settings")

func choose_achievements(enabled):
	config.set_value("achievements", "enabled", enabled)
	config.set_value("achievements", "chosen", true)
	config.save("user://settings.cfg")
	achievement_choice_dialog = null
	if achievement_choice_first:
		start_runtime(achievement_choice_save)
		return
	if not enabled:
		achievement_queue.clear()
		if is_instance_valid(achievement_toast):
			achievement_toast.queue_free()
		achievement_toast = null
	send({"action": "achievements", "on": enabled})
	show_settings()

func needs_artwork_choice():
	# Separate from the old on/off setting, so existing players see this once.
	return not config.get_value("graphics", "redrawn_ui_chosen", false)

func show_artwork_choice(save_path = "", first_start = false):
	artwork_choice_first = first_start
	artwork_choice_save = save_path
	var examples = null
	if runtime != null and not game_index.empty():
		examples = JSON.parse(runtime.query("artwork_examples", JSON.print({"index": game_index}))).result
	var box = panel("Choose interface artwork")
	artwork_choice_dialog = modal
	status.hide()
	var preview = preload("res://scripts/artwork_preview.gd").new()
	preview.name = "artwork_preview"
	box.add_child(preview)
	preview.populate(self, examples)
	var hint = Label.new()
	hint.text = "Redraws change the toolbar icons and navigation hint. You can change this later in Game files / mods > HD artwork."
	if not first_start:
		hint.text = "Your choice applies at once, also in a running game."
	if small_display():
		# Redraws make the engine compose every frame at twice the size, which
		# a screen this small cannot show and slower handhelds feel.
		hint.text += "\nOn this small screen the original artwork runs faster and looks much the same."
		hint.rect_min_size = Vector2(396,64)
	hint.autowrap = true
	hint.rect_min_size = Vector2(396,44)
	hint.add_font_override("font", get_font_for("13px Arial"))
	box.add_child(hint)
	button(box, "Keep original artwork", "choose_artwork", [false]).grab_focus()
	button(box, "Enable redrawn artwork", "choose_artwork", [true])

func choose_artwork(enabled):
	config.set_value("graphics", "redrawn_ui", enabled)
	config.set_value("graphics", "redrawn_ui_chosen", true)
	config.save("user://settings.cfg")
	artwork_choice_dialog = null
	if artwork_choice_first:
		start_runtime(artwork_choice_save)
	else:
		apply_artwork()
		show_hd_settings()

func reset_hd_folder():
	config.set_value("graphics", "hd_folder", "")
	config.save("user://settings.cfg")
	hd_pack_path = find_hd_pack()
	apply_artwork()
	show_hd_settings()

func choose_hd_folder():
	if Engine.has_singleton("TitanicFiles"):
		Engine.get_singleton("TitanicFiles").choose_hd_folder()
	else:
		file_dialog(FileDialog.MODE_OPEN_DIR, "hd_folder_selected")

func hd_folder_selected(path):
	if not File.new().file_exists(path.plus_file("manifest.json")):
		show_note("Choose the hdpack folder containing manifest.json and images.")
		return
	config.set_value("graphics", "hd_folder", path)
	config.save("user://settings.cfg")
	hd_pack_path = find_hd_pack()
	apply_artwork()
	show_hd_settings()

func android_hd_selected(path, error):
	if not error.empty():
		show_note(error)
	elif not path.empty():
		config.set_value("graphics", "hd_folder", path)
		config.save("user://settings.cfg")
		hd_pack_path = "saf-hd:" + path
		apply_artwork()
		show_hd_settings()

func android_hd_detected(path, error):
	# Game import finishes immediately afterwards and boots with the detected pack.
	if not error.empty():
		print("HD folder unavailable: ", error)

func choose_mods():
	if Engine.has_singleton("TitanicFiles"):
		Engine.get_singleton("TitanicFiles").import_mods(ProjectSettings.globalize_path("user://"))
		return
	file_dialog(FileDialog.MODE_OPEN_DIR, "mods_selected")

func mods_selected(path):
	active_mods = path
	reload_configured_data()

func disable_mods():
	active_mods = ""
	reload_configured_data()

func reload_configured_data():
	var roots = [config.get_value("game", "disc1", ""), config.get_value("game", "disc2", "")]
	if roots[0].empty() or not prepare_index(roots):
		show_note("Choose the game files first. " + files.error)
		return
	config.set_value("game", "mods", active_mods)
	config.save("user://settings.cfg")
	start_runtime()

func show_next_dialog():
	if queued_dialogs.empty():
		return
	var event = queued_dialogs.pop_front()
	current_dialog = event.id
	if event.kind in ["note", "question"]:
		var box = panel("Titanic")
		text_scroller(box, event.text, 180)
		button(box, "Yes" if event.kind == "question" else "OK", "reply_dialog", [true]).grab_focus()
		if event.kind == "question":
			button(box, "Cancel", "reply_dialog", [false])
	elif event.kind in ["save", "text"]:
		var box = panel(event.text)
		save_name = LineEdit.new()
		save_name.text = event.value
		save_name.add_font_override("font", get_font_for("14px Arial"))
		save_name.connect("text_entered", self, "submit_entered_text", [event.kind])
		box.add_child(save_name)
		button(box, "On-screen keyboard", "show_keyboard", [save_name])
		button(box, "Save" if event.kind == "save" else "OK", "submit_text", [event.kind]).grab_focus()
		button(box, "Cancel", "reply_dialog", [null])
	elif event.kind == "load":
		show_save_list("load")
	elif event.kind == "import":
		if Engine.has_singleton("TitanicFiles"):
			Engine.get_singleton("TitanicFiles").import_save(ProjectSettings.globalize_path("user://"))
			return
		file_dialog(FileDialog.MODE_OPEN_FILE, "reply_dialog", [], "*.ti ; Titanic saved game").connect("popup_hide", self, "cancel_file_reply")

func submit_entered_text(_text, kind):
	submit_text(kind)

func submit_text(kind):
	var text = save_name.text.strip_edges()
	if text.empty():
		return
	if kind == "save" and File.new().file_exists(saves.manual_path_for(text)):
		var box = panel("Replace existing save?")
		button(box, "Replace and archive previous version", "reply_dialog", [text]).grab_focus()
		button(box, "Cancel", "reply_dialog", [null])
	else:
		reply_dialog(text)

func reply_dialog(value):
	if value == null:
		importing_save = false
	var id = current_dialog
	current_dialog = -1
	close_modal()
	if id != -1:
		send({"action": "reply", "id": id, "value": value})
	if not queued_dialogs.empty():
		show_next_dialog()

func cancel_file_reply():
	if current_dialog != -1:
		reply_dialog(null)

func show_save_list(action):
	var box = panel("Saved games")
	var scroll = ScrollContainer.new()
	scroll.rect_min_size = Vector2(390, 200)
	scroll.follow_focus = true
	box.add_child(scroll)
	var list = VBoxContainer.new()
	list.size_flags_horizontal = SIZE_EXPAND_FILL
	scroll.add_child(list)
	var first = null
	for name in saves.list_saves():
		var b = button(list, saves.list_name(name), "save_selected", [name, action])
		b.clip_text = true
		b.hint_tooltip = saves.display_name(name)
		b.set_meta("autosave", saves.is_autosave(name))
		decorate_save_row(b, name)
		if saves.is_autosave(name):
			for state in ["font_color", "font_color_hover", "font_color_pressed", "font_color_focus"]:
				b.add_color_override(state, Color("b5d8ef"))
		if first == null:
			first = b
	button(box, "Cancel", "reply_dialog", [null]).grab_focus()
	if first != null:
		first.grab_focus()

# A picture of the moment and when it was saved, like any current game.
func decorate_save_row(b, name):
	b.rect_min_size.y = 42
	b.set("align", 0)
	b.set("alignment", 0)
	var img = saves.load_thumbnail(name)
	if img != null:
		var texture = ImageTexture.new()
		texture.create_from_image(img, 4)
		b.icon = texture
		b.expand_icon = true
		b.set_meta("thumbnail", true)
	var when = Label.new()
	when.name = "When"
	when.text = saves.describe_time(saves.modified_time(name)) + "  "
	when.mouse_filter = Control.MOUSE_FILTER_IGNORE
	when.set("align", 2)
	when.set("horizontal_alignment", 2)
	when.valign = Label.VALIGN_CENTER
	when.add_font_override("font", get_font_for("11px Arial"))
	when.add_color_override("font_color", Color("a39a84"))
	b.add_child(when)
	when.set_anchors_and_margins_preset(15)

func save_selected(name, action):
	if action == "load":
		reply_dialog(saves.path_for(name))
	else:
		if Engine.has_singleton("TitanicFiles"):
			Engine.get_singleton("TitanicFiles").export_save(ProjectSettings.globalize_path(saves.path_for(name)))
			return
		var d = file_dialog(FileDialog.MODE_SAVE_FILE, "export_selected", [name], "*.ti ; Titanic saved game")
		d.current_file = name.get_file()

func save_tools():
	resume_game()
	var box = panel("Saved games")
	button(box, "Import and load .ti file", "import_save").grab_focus()
	button(box, "Export .ti file", "show_save_list", ["export"])
	button(box, "Open save folder", "open_save_folder")
	button(box, "Back", "close_modal")

func import_save():
	close_modal()
	importing_save = true
	send({"action": "import"})

func export_selected(path, name):
	if Directory.new().copy(saves.path_for(name), path) != OK:
		show_note("Could not export the saved game.")
	else:
		close_modal()

func open_save_folder():
	OS.shell_open(ProjectSettings.globalize_path(saves.root))

func show_keyboard(target):
	if virtual_keyboard != null:
		return
	keyboard_target = target
	saved_keyboard_focus = get_focus_owner()
	saved_dialog = modal
	if saved_dialog != null:
		saved_dialog.hide()
	modal = null
	var box = panel("Keyboard — select a key, or type normally")
	virtual_keyboard = modal
	var grid = GridContainer.new()
	grid.columns = 10
	box.add_child(grid)
	for key in "1234567890qwertyuiopasdfghjklzxcvbnm.,-":
		button(grid, key, "keyboard_key", [key])
	button(box, "Space", "keyboard_key", [" "])
	button(box, "Backspace", "keyboard_key", ["\b"])
	button(box, "Enter", "keyboard_key", ["\r"])
	button(box, "Done", "close_keyboard")
	grid.get_child(0).grab_focus()

func keyboard_key(key):
	if is_instance_valid(keyboard_target):
		if key == "\b":
			keyboard_target.text = keyboard_target.text.substr(0, max(0, keyboard_target.text.length() - 1))
		elif key != "\r":
			keyboard_target.text += key
	else:
		send({"action": "key", "key": key})

func close_keyboard():
	close_modal()
	virtual_keyboard = null
	if is_instance_valid(saved_dialog):
		modal = saved_dialog
		modal.show()
		if is_instance_valid(saved_keyboard_focus):
			saved_keyboard_focus.grab_focus()
	saved_dialog = null
	saved_keyboard_focus = null

# Touch controls occupy a separate strip below the original 512x384 picture.
var touch_strip
var touch_editing = false
var touch_editor
var touch_enabled = false
var touch_click_down = false
var touch_mouse_blocked = false
var touch_nav_timer = 0.0
var touch_nav_direction = ""

func block_touch_mouse(event):
	# Godot emits the mouse copy BEFORE the native touch event. Consuming the
	# joystick's touch event alone cannot stop that copy clicking the world.
	if controller_pointer_event or event.device != -1 or not (event is InputEventMouseButton or event is InputEventMouseMotion):
		return false
	if event is InputEventMouseButton and event.button_index == BUTTON_LEFT:
		if event.pressed:
			touch_mouse_blocked = false
			if not touch_editing and modal == null and is_instance_valid(touch_strip) and touch_strip.is_visible_in_tree():
				var joystick = touch_strip.get_node("joystick")
				touch_mouse_blocked = event.position.distance_to(joystick.position) <= joystick.radius
				for control in ["menu","space","escape","keyboard"]:
					if touch_control_bounds(control).has_point(event.position):
						touch_mouse_blocked = true
		else:
			var was_blocked = touch_mouse_blocked
			touch_mouse_blocked = false
			return was_blocked
	# Keep ownership until release, including drags out of the control, hidden
	# joystick artwork and screen changes caused by the control's own action.
	return touch_mouse_blocked

func update_touch_layout(_device = 0, _connected = false):
	if adaptive_active and (not adaptive_enabled() or not adaptive_widescreen()):
		set_adaptive_active(false)
		return
	touch_enabled = touch_editing or (OS.get_name() == "Android" or OS.has_touchscreen_ui_hint() or "--touch-test" in OS.get_cmdline_args()) and Input.get_connected_joypads().empty()
	var physical = OS.window_size
	var portrait = physical.y > physical.x
	var desired = Vector2(512, 384)
	if (adaptive_enabled() or wide_dialogue_enabled()) and adaptive_widescreen():
		desired = Vector2(384.0 * physical.x / max(1,physical.y),384)
	elif touch_enabled:
		desired = Vector2(512, max(640, 512.0 * physical.y / max(1, physical.x))) if portrait else Vector2(max(640, 384.0 * physical.x / max(1, physical.y)), 384)
	# Keep the picture and its 536-pixel control block within thumb reach.
	game_origin = Vector2(0, max(12, desired.y - 548)) if touch_enabled and portrait else Vector2((desired.x - 512) / 2, 0)
	if desired != layout_size:
		layout_size = desired
		get_tree().set_screen_stretch(SceneTree.STRETCH_MODE_2D, SceneTree.STRETCH_ASPECT_KEEP, layout_size)
	if adaptive_active:
		adaptive.configure(desired, adaptive_metadata, adaptive_roomy())
		adaptive_configured = []
	if dialogue_active:
		dialogue.configure(desired,dialogue_metadata.get("choices",0),touch_enabled)
	status.rect_position = game_origin + Vector2(20, 160)
	if is_instance_valid(checkpoint_toast):
		checkpoint_toast.rect_position = game_origin + Vector2(16, 16)
	if is_instance_valid(backdrop):
		backdrop.rect_size = layout_size
	for child in get_children():
		if child is PanelContainer and child != achievement_toast and child != item_notice:
			child.rect_position = (layout_size - child.rect_size) / 2
	place_achievement_toast()
	place_item_notice()
	if is_instance_valid(scratchpad):
		scratchpad.rect_size = layout_size
		scratchpad.relayout()
	if touch_strip == null:
		return
	touch_strip.visible = touch_enabled and modal == null
	var specs
	if dialogue_active:
		touch_strip.get_node("joystick").configure(Vector2(70,desired.y-70),58)
		specs = [["menu","Menu",Vector2(16,16)],["escape","Skip",Vector2(desired.x-68,desired.y-64)],["space","Door",Vector2(desired.x-68,80)],["keyboard","Keys",Vector2(desired.x-68,140)]]
	elif adaptive_active:
		var rail = adaptive.rail
		touch_strip.get_node("joystick").configure(Vector2(70,desired.y-70),58)
		var right = desired.x-rail/2-26
		specs = [["menu","Menu",Vector2(144,desired.y-64)],["space","Door",Vector2(right,208)],["escape","Skip",Vector2(right,262)],["keyboard","Keys",Vector2(right,316)]]
	elif portrait:
		var y = game_origin.y + 420
		touch_strip.get_node("joystick").configure(Vector2(112, y + 58), 58)
		specs = [["space", "Door", Vector2(264,y+64)], ["escape", "Skip", Vector2(336,y+64)], ["menu", "Menu", Vector2(264,y)], ["keyboard", "Keys", Vector2(336,y)]]
	else:
		var right = game_origin.x + 512 + max(0, (game_origin.x - 52) / 2)
		touch_strip.get_node("joystick").configure(Vector2(game_origin.x / 2, 192), clamp((game_origin.x - 8) / 2, 28, 64))
		specs = [["menu", "Menu", Vector2(right,72)], ["escape", "Skip", Vector2(right,132)], ["space", "Door", Vector2(right,192)], ["keyboard", "Keys", Vector2(right,252)]]
	for spec in specs:
		touch_strip.get_node(spec[0]).position = spec[2]
	apply_touch_preferences()
	update_layout_switch()
	update()

func make_touch_controls():
	touch_strip = Node2D.new()
	add_child(touch_strip)
	var joystick = load("res://scripts/touch_joystick.gd").new()
	joystick.name = "joystick"
	touch_strip.add_child(joystick)
	joystick.connect("door_tapped", self, "touch_door")
	joystick.connect("direction_pressed", self, "touch_direction")
	for spec in [["space", "Door"], ["escape", "Skip"], ["menu", "Menu"], ["keyboard", "Keys"]]:
		var action = "touch_" + spec[0]
		if not InputMap.has_action(action):
			InputMap.add_action(action)
		var t = load("res://scripts/touch_button.gd").new()
		t.name = spec[0]
		t.action = action
		var img = Image.new()
		img.create(52, 52, false, Image.FORMAT_RGBA8)
		img.fill(Color(0,0,0,0))
		var texture = ImageTexture.new()
		texture.create_from_image(img, 0)
		t.normal = texture
		touch_strip.add_child(t)
		var text = Label.new()
		text.text = spec[1]
		text.rect_position = Vector2(1,34)
		text.rect_size = Vector2(50,15)
		text.align = Label.ALIGN_CENTER
		text.add_font_override("font", get_font_for("10px Arial"))
		text.mouse_filter = Control.MOUSE_FILTER_IGNORE
		t.add_child(text)
	Input.connect("joy_connection_changed", self, "update_touch_layout")
	# Resizing content inside this signal re-enters the engine's viewport resize.
	# Let the current resize finish before updating the logical canvas.
	get_viewport().connect("size_changed", self, "update_touch_layout", [], CONNECT_DEFERRED)
	update_touch_layout()

func touch_door():
	if ready and modal == null and not touch_editing:
		refresh_controller_surface()
		if controller_surface.context in ["dialogue","panel","movie"] and not controller_surface.targets.empty():
			controller_confirm(true)
			controller_confirm(false)
		else:
			send({"action": "key", "key": " "})

func update_touch_context():
	if not is_instance_valid(touch_strip):
		return
	var selecting = controller_surface.context in ["dialogue","panel","movie"] and not controller_surface.targets.empty()
	var joystick = touch_strip.get_node("joystick")
	var confirm = touch_strip.get_node("space")
	if joystick.confirm_mode != selecting:
		joystick.confirm_mode = selecting
		joystick.update()
		confirm.confirm_mode = selecting
		confirm.get_child(0).text = "Select" if selecting else "Door"
		confirm.update()

func touch_direction(direction):
	if ready and modal == null and not touch_editing:
		touch_nav_direction = direction
		touch_nav_timer = 0.55
		controller_direction(direction + "arrow")

func process_touch(delta):
	touch_strip.visible = touch_enabled and modal == null
	if not touch_enabled or modal != null or interrupted() or touch_editing:
		touch_nav_direction = ""
		touch_nav_timer = 0.0
		return
	if Input.is_action_just_pressed("touch_space"):
		touch_door()
	if Input.is_action_just_pressed("touch_escape"):
		send({"action": "key", "key": ".", "special": true})
	if Input.is_action_just_pressed("touch_menu"):
		show_menu()
	if Input.is_action_just_pressed("touch_keyboard") and virtual_keyboard == null:
		show_keyboard(save_name if current_dialog != -1 and is_instance_valid(save_name) else null)
	var direction = ""
	if modal == null:
		for candidate in ["up", "down", "left", "right"]:
			if Input.is_action_pressed("touch_" + candidate):
				direction = candidate
				break
	if direction != touch_nav_direction:
		touch_nav_direction = direction
		touch_nav_timer = 0.55
		if not direction.empty():
			controller_direction(direction + "arrow")
	elif not direction.empty():
		touch_nav_timer -= delta
		if touch_nav_timer <= 0:
			controller_direction(direction + "arrow")
			touch_nav_timer = 0.40

func android_import_finished(path, error):
	print("Android game import callback received")
	if path.empty() and error.empty():
		show_setup()
		return
	if not error.empty():
		show_note(error)
		return
	if Engine.has_singleton("TitanicFiles"):
		config.set_value("game", "android_tree", Engine.get_singleton("TitanicFiles").selected_game_tree())
	data_selected(path, android_selected_disc)

func android_mod_finished(path, error):
	if not error.empty():
		show_note(error)
	elif not path.empty():
		mods_selected(path)

func android_save_finished(path, error):
	if not error.empty():
		reply_dialog(null)
		show_note(error)
	else:
		reply_dialog(path if not path.empty() else null)

func android_export_finished(path, error):
	if not error.empty():
		show_note(error)
	elif not path.empty():
		show_note("Saved game exported.")

func show_patch_picker(first_start = false):
	manual_pause = true
	send({"action": "pause", "on": true})
	var box = panel("M3tox patches 1.0.3")
	var intro = Label.new()
	intro.text = "Optional fixes for Steam / GOG game files.\n" + ("You can change this later in Game files / mods." if first_start else "Applying changes restarts the game. Save first.")
	intro.add_font_override("font", get_font_for("12px Arial"))
	box.add_child(intro)
	var scroll = ScrollContainer.new()
	scroll.rect_min_size = Vector2(390, 100 if not patches_available() else 135)
	scroll.follow_focus = true
	scroll.scroll_horizontal_enabled = false
	box.add_child(scroll)
	var list = VBoxContainer.new()
	list.size_flags_horizontal = SIZE_EXPAND_FILL
	list.add_constant_override("separation", 8)
	scroll.add_child(list)
	patch_boxes.clear()
	var selected = config.get_value("patches", "enabled", [])
	for group in patch_manifest.get("groups", []):
		var check = CheckBox.new()
		check.text = group.title
		check.pressed = group.id in selected
		check.rect_min_size.y = 30
		check.add_icon_override("checked", preload("res://icons/checkbox_checked.svg"))
		check.add_icon_override("unchecked", preload("res://icons/checkbox_unchecked.svg"))
		check.add_font_override("font", get_font_for("14px Arial"))
		list.add_child(check)
		patch_boxes[group.id] = check
		var explanation = Label.new()
		explanation.text = group.description
		explanation.autowrap = true
		explanation.rect_min_size = Vector2(350, 44)
		explanation.add_font_override("font", get_font_for("12px Arial"))
		list.add_child(explanation)
	patch_buttons.clear()
	patch_status_label = Label.new()
	patch_status_label.autowrap = true
	patch_status_label.rect_min_size = Vector2(390, 34)
	patch_status_label.add_font_override("font", get_font_for("12px Arial"))
	box.add_child(patch_status_label)
	if not patches_available():
		patch_status_label.text = "Download about 300 MiB, choose the FULL ZIP, or continue with no patches selected."
		var sources = HBoxContainer.new()
		box.add_child(sources)
		patch_buttons.append(button(sources, "Download from GitHub", "patch_download"))
		patch_buttons.append(button(sources, "Choose patch ZIP", "patch_choose_zip"))
		button(sources, "Stop", "patch_cancel")
	else:
		patch_status_label.text = "Patch files are ready. Choose which fixes to enable."
	var actions = HBoxContainer.new()
	box.add_child(actions)
	button(actions, "All", "patch_select_all", [true])
	button(actions, "None", "patch_select_all", [false])
	button(actions, "Continue" if first_start else "Apply and restart", "patch_apply").grab_focus()
	if not first_start:
		button(actions, "Cancel", "show_setup")

func patch_select_all(enabled):
	for check in patch_boxes.values():
		check.pressed = enabled

func patch_apply():
	if patch_busy:
		patch_status_label.text = "Wait for the patch transfer to finish, or stop it first."
		return
	var enabled = []
	for id in patch_boxes:
		if patch_boxes[id].pressed:
			enabled.append(id)
	if not enabled.empty() and not patches_available():
		patch_status_label.text = "Download or import the FULL ZIP first, or select None to play without patches."
		return
	config.set_value("patches", "enabled", enabled)
	config.set_value("patches", "seen", patch_manifest.get("version", ""))
	if config.save("user://settings.cfg") != OK:
		show_note("Could not save your patch choices.")
		return
	close_modal()
	boot_configured_game()

func files_dropped(paths, _screen = 0):
	if paths.size() == 1 and paths[0].to_lower().ends_with(".ti") and runtime != null:
		send({"action": "import", "path": paths[0]})

func text_scroller(box, text, height):
	var scroll = ScrollContainer.new()
	scroll.rect_min_size = Vector2(390, height)
	scroll.scroll_horizontal_enabled = false
	box.add_child(scroll)
	var label = Label.new()
	label.text = text
	label.autowrap = true
	label.rect_min_size.x = 365
	label.size_flags_horizontal = SIZE_EXPAND_FILL
	label.add_font_override("font", get_font_for("13px Arial"))
	scroll.add_child(label)
	return scroll

func show_credits():
	menu = null
	var file = File.new()
	var text = "See CREDITS.md for the authors and projects that made this player possible."
	if file.open("res://notices/CREDITS.txt", File.READ) == OK:
		text = file.get_as_text()
		file.close()
	show_note(text)

func patch_cache():
	return "user://Patches/" + patch_manifest.get("sha256", "unknown")

func patch_path(name):
	var bundled = "res://patches/files/" + name
	if File.new().file_exists(bundled):
		return bundled
	var personal = OS.get_environment("RETANIC_PATCH_DIR").plus_file(name)
	if not OS.get_environment("RETANIC_PATCH_DIR").empty() and File.new().file_exists(personal):
		return personal
	return patch_cache().plus_file(name)

func patches_available():
	for name in patch_manifest.get("files", {}):
		if not File.new().file_exists(patch_path(name)):
			return false
	return not patch_manifest.empty()

func patch_download():
	patch_start("")

func patch_choose_zip():
	if patch_busy:
		return
	if Engine.has_singleton("TitanicFiles"):
		Engine.get_singleton("TitanicFiles").import_patches(ProjectSettings.globalize_path("user://"))
	else:
		file_dialog(FileDialog.MODE_OPEN_FILE, "patch_start", [], "*.zip ; M3tox FULL patch ZIP")

func android_patch_finished(path, error):
	if not error.empty():
		if is_instance_valid(patch_status_label):
			patch_status_label.text = error
		return
	if not path.empty():
		patch_import_copy = path
		patch_start(path)

func patch_start(archive):
	if patch_busy:
		return
	if patch_runtime == null:
		patch_runtime = create_runtime()
	if patch_runtime == null:
		return
	var error = patch_runtime.execute("patch_start", JSON.print({"archive": archive, "target": ProjectSettings.globalize_path(patch_cache())}))
	if not error.empty():
		if is_instance_valid(patch_status_label):
			patch_status_label.text = error
		return
	patch_busy = true
	for b in patch_buttons:
		if is_instance_valid(b):
			b.disabled = true
	if is_instance_valid(patch_status_label):
		patch_status_label.text = "Preparing patches…"

func patch_cancel():
	if patch_runtime != null:
		patch_runtime.execute("patch_cancel")

func poll_patches(delta):
	if not patch_busy or patch_runtime == null:
		return
	patch_poll_timer += delta
	if patch_poll_timer < 0.2:
		return
	patch_poll_timer = 0.0
	var progress = JSON.parse(patch_runtime.query("patch_status")).result
	var message = progress.state.capitalize()
	if progress.state in ["downloading", "verifying"]:
		message += ": %.1f MiB" % (float(progress.bytes) / 1048576.0)
	elif progress.state == "extracting":
		message = "Installing patches: %d / %d" % [progress.bytes, progress.total]
	if progress.state in ["ready", "error"]:
		patch_busy = false
		message = "Patch files are ready. Choose your fixes and continue." if progress.state == "ready" else progress.error
		patch_runtime = null
		for b in patch_buttons:
			if is_instance_valid(b):
				b.disabled = false
		if not patch_import_copy.empty():
			var directory = Directory.new()
			directory.remove(patch_import_copy)
			directory.remove(patch_import_copy.get_base_dir())
			patch_import_copy = ""
	if is_instance_valid(patch_status_label):
		patch_status_label.text = message

func wide_dialogue_enabled():
	return config.get_value("dialogue","widescreen",false)

func needs_dialogue_choice():
	return not config.get_value("dialogue","presentation_chosen",false)

func show_dialogue_options(save_path = "", first_start = false):
	dialogue_choice_first = first_start
	dialogue_choice_save = save_path
	var box = panel("Dialogue presentation")
	dialogue_choice_dialog = modal
	dialogue_preview = preload("res://scripts/dialogue_preview.gd").new()
	dialogue_preview.player = self
	box.add_child(dialogue_preview)
	button(box,"Layout: " + ("Widescreen" if wide_dialogue_enabled() else "Classic"),"toggle_dialogue_option",["widescreen"]).disabled = classic_format_device()
	button(box,"Speech-timed words: " + ("On" if config.get_value("dialogue","timed_subtitles",false) else "Off"),"toggle_dialogue_option",["timed_subtitles"]).grab_focus()
	text_scroller(box,"Words gently fade in over the length of each spoken line. Timing is approximate. Change either option later in Settings > Controls / display.\n" + ("This 4:3 display uses Classic dialogue." if classic_format_device() else "Widescreen uses translucent answers in landscape; narrow screens keep Classic."),65)
	button(box,"Continue" if first_start else "Back","finish_dialogue_options")

func toggle_dialogue_option(key):
	config.set_value("dialogue",key,not config.get_value("dialogue",key,false))
	config.save("user://settings.cfg")
	send({"action":"timed_subtitles","on":config.get_value("dialogue","timed_subtitles",false)})
	update_touch_layout()
	show_dialogue_options(dialogue_choice_save,dialogue_choice_first)

func finish_dialogue_options():
	config.set_value("dialogue","presentation_chosen",true)
	config.save("user://settings.cfg")
	dialogue_choice_dialog = null
	close_modal()
	if dialogue_choice_first:
		start_runtime(dialogue_choice_save)
	else:
		show_control_options()

func sync_dialogue_layout():
	var metadata = {}
	if wide_dialogue_enabled() and adaptive_widescreen():
		# As with the side panels, the answer is the same on most frames.
		var text = runtime.query("dialogue_layout")
		if text != dialogue_text:
			dialogue_text = text
			dialogue_parsed = JSON.parse(text).result
		metadata = dialogue_parsed
	var active = metadata is Dictionary and metadata.get("eligible",false) and wide_dialogue_enabled() and adaptive_widescreen() and not runtime_failed and has_frame
	if metadata is Dictionary:
		dialogue_metadata = metadata
	if active:
		dialogue.configure(layout_size,metadata.get("choices",0),touch_enabled)
	if dialogue_active != active:
		if pointer_pressed:
			send({"action":"pointer","kind":"release","x":-1,"y":-1})
			pointer_pressed = false
		dialogue_active = active
		controller_selection = -1
		dialogue_preselected = false
		if active and controller_surface.context == "dialogue" and not controller_surface.targets.empty():
			controller_selection = 0
			dialogue_preselected = true
		update_touch_layout()
		pointer = display_to_game(display_pointer)
		refresh_cursor()
		update()

func draw_wide_dialogue():
	var hd = frame_image.get_width()/512.0
	draw_texture_rect_region(frame_texture,Rect2(Vector2.ZERO,layout_size),Rect2(0,0,512*hd,264*hd))
	for bounds in dialogue.choices:
		draw_style_box(UIStyle.plate(Color(0,0,0,0.32),Color(0.78,0.68,0.45,0.22),3),bounds)
	for command in overlays:
		if command.op == "rect" and command.h == 384:
			draw_set_transform(Vector2.ZERO,0,Vector2.ONE)
			draw_rect(Rect2(Vector2.ZERO,layout_size),ink(command.color))
			continue
		if command.y >= 264:
			var index = int((command.y-264)/24)
			if index >= dialogue.choices.size():
				continue
			var bounds = dialogue.choices[index]
			var scale = min(1.0,bounds.size.x/512.0)
			draw_set_transform(bounds.position+Vector2(0,12-12*scale)-Vector2(0,264+index*24)*scale,0,Vector2(scale,scale))
		elif command.y >= 224:
			draw_set_transform(dialogue.subtitle.position-Vector2(0,224),0,Vector2.ONE)
		else:
			draw_set_transform(Vector2.ZERO,0,layout_size/Vector2(512,264))
		if command.op == "text":
			draw_string(get_font_for(command.font),Vector2(command.x,command.y),command.text,ink(command.color))
		elif command.op == "rect":
			draw_rect(Rect2(command.x,command.y,command.w,command.h),Color(0,0,0,0.25))
		elif command.op == "stroke":
			draw_rect(Rect2(command.x,command.y,command.w,command.h),Color(0.78,0.68,0.45,0.45),false,1)
	draw_set_transform(Vector2.ZERO,0,Vector2.ONE)
