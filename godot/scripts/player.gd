extends Control

const GameFiles = preload("res://scripts/game_files.gd")
const UIStyle = preload("res://scripts/ui_style.gd")
const AdaptiveLayout = preload("res://scripts/adaptive_layout.gd")
const SaveStore = preload("res://scripts/save_store.gd")
var runtime
var files = GameFiles.new()
var saves = SaveStore.new()
var config = ConfigFile.new()
var game_index = {}
var hd_pack_path = ""
var adaptive = AdaptiveLayout.new()
var adaptive_panel_style = UIStyle.plate(Color("1b202140"),Color("645b4660"),6)
var adaptive_button_style = UIStyle.plate(Color("24292960"),Color("4f504580"),4)
var adaptive_hover_style = UIStyle.plate(Color("35372f90"),Color("9b875dc0"),4)
var touch_surround_style = UIStyle.plate(Color(0,0,0),Color(0,0,0),0)
var adaptive_active = false
var adaptive_available = false
var prefer_classic_now = false
var layout_switch
var adaptive_metadata = {}
var adaptive_atlas_image = Image.new()
var adaptive_atlas_texture = ImageTexture.new()
var adaptive_atlas_revision = -1
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
	["previous", "Previous target"], ["next", "Next target"], ["precision", "Slow pointer"],
	["up", "Up"], ["down", "Down"], ["left", "Left"], ["right", "Right"]]
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
	OS.low_processor_usage_mode = true
	get_tree().set_auto_accept_quit(false)
	get_tree().set_quit_on_go_back(false)
	get_tree().connect("files_dropped", self, "files_dropped")
	config.load("user://settings.cfg")
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
	set_adaptive_active(false)
	adaptive_atlas_revision = -1
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
	queued_dialogs.clear()
	current_dialog = -1
	close_modal()
	hd_pack_path = find_hd_pack()
	runtime = create_runtime()
	if runtime == null:
		return
	var error = ""
	runtime.execute("profile", JSON.print({"on": OS.is_debug_build()}))
	error = runtime.execute("boot", JSON.print({"index": game_index, "save": save_path, "hd_pack": active_hd_pack(), "disable_autosave": not config.get_value("saves", "autosave_enabled", true), "testing": "--integration-test" in OS.get_cmdline_args()}))
	if not error.empty():
		show_note(error)
		return
	status.show()
	status.text = "Preparing your voyage…"
	send({"action": "pause", "on": interrupted()})

func find_hd_pack():
	var base = OS.get_executable_path().get_base_dir()
	var candidates = ["res://hdpack", "user://hdpack", base.plus_file("hdpack")]
	var game_dir = OS.get_environment("RETANIC_GAME_DIR")
	if not game_dir.empty():
		candidates.append(game_dir.plus_file("hdpack"))
	var appimage = OS.get_environment("APPIMAGE")
	if not appimage.empty():
		candidates.append(appimage.get_base_dir().plus_file("hdpack"))
	if OS.get_name() in ["OSX", "macOS"]:
		candidates.append(base.get_base_dir().get_base_dir().get_base_dir().plus_file("hdpack"))
	for arg in OS.get_cmdline_args():
		if arg.begins_with("--hd-pack="):
			return arg.substr(10)
	for candidate in candidates:
		if File.new().file_exists(candidate.plus_file("manifest.json")):
			return candidate
	return ""

func active_hd_pack():
	return hd_pack_path if config.get_value("graphics", "hd_enabled", true) else ""

func toggle_hd_artwork():
	var enabled = not config.get_value("graphics", "hd_enabled", true)
	config.set_value("graphics", "hd_enabled", enabled)
	config.save("user://settings.cfg")
	# Apply on next startup; preserve unsaved progress in the running game.
	show_setup()

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

func toggle_autosave():
	var enabled = not config.get_value("saves", "autosave_enabled", true)
	config.set_value("saves", "autosave_enabled", enabled)
	config.save("user://settings.cfg")
	send({"action": "autosave_enabled", "on": enabled})
	show_setup()

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
		"read":
			var path = args.path
			if path.begins_with("save:"):
				path = saves.path_for(path.substr(5))
			return files.read_asset(path)
		"write":
			if args.path.begins_with("autosave:"):
				var saved = saves.write_autosave(args.path.substr(9), bytes)
				return JSON.print({"ok": true} if saved else {"error": saves.error})
			if not args.path.begins_with("save:"):
				return JSON.print({"error": "Invalid save destination"})
			var ok = saves.write(args.path.substr(5), bytes)
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
		font.font_data = load("res://fonts/LiberationMono-Regular.ttf" if "Courier" in key else "res://fonts/LiberationSans-Regular.ttf")
		fonts[key] = font
	return fonts[key]

func ink(text):
	if text.begins_with("rgb"):
		var start = text.find("(") + 1
		var values = text.substr(start, text.length() - start - 1).split(",")
		return Color(float(values[0]) / 255, float(values[1]) / 255, float(values[2]) / 255, float(values[3]) if values.size() == 4 else 1.0)
	return Color(text)

func adaptive_enabled():
	return config.get_value("graphics", "adaptive_exploration", false)

func adaptive_roomy():
	return config.get_value("graphics","adaptive_roomy",false)

func classic_format(size):
	return max(size.x,size.y) < min(size.x,size.y)*1.6

func classic_format_device():
	# Desktop windows start at 4:3 even on wide monitors. Classify the device,
	# while the active layout below always uses the actual window size.
	var size = OS.get_screen_size(OS.current_screen)
	if size.x <= 0 or size.y <= 0:
		size = OS.window_size
	return classic_format(size)

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
	update_touch_layout()
	display_pointer = game_to_display(pointer)
	refresh_cursor()
	update()

func sync_adaptive_layout():
	adaptive_available = false
	if not adaptive_enabled() or not adaptive_widescreen() or modal != null or runtime == null or runtime_failed or not has_frame:
		set_adaptive_active(false)
		update_layout_switch()
		return
	var metadata = JSON.parse(runtime.query("adaptive_layout")).result
	if not metadata is Dictionary or not metadata.get("eligible", false) or not adaptive_overlays_fit():
		set_adaptive_active(false)
		update_layout_switch()
		return
	adaptive_available = true
	if prefer_classic_now:
		set_adaptive_active(false)
		update_layout_switch()
		return
	adaptive_metadata = metadata
	adaptive.configure(layout_size, metadata, adaptive_roomy())
	if metadata.revision != adaptive_atlas_revision:
		var bytes = runtime.buffer("adaptive_atlas")
		if bytes.size() != 640 * 128 * 4:
			adaptive_available = false
			set_adaptive_active(false)
			update_layout_switch()
			return
		adaptive_atlas_image.create_from_data(640,128,false,Image.FORMAT_RGBA8,bytes)
		if adaptive_atlas_revision < 0:
			adaptive_atlas_texture.create_from_image(adaptive_atlas_image,0)
		else:
			adaptive_atlas_texture.set_data(adaptive_atlas_image)
		adaptive_atlas_revision = metadata.revision
		update()
	set_adaptive_active(true)
	update_layout_switch()

func update_layout_switch():
	if not is_instance_valid(layout_switch):
		return
	layout_switch.visible = adaptive_available and modal == null and not touch_editing and adaptive_widescreen()
	layout_switch.text = "Classic" if adaptive_active else "Side panels"
	layout_switch.rect_position = Vector2(6,210) if adaptive_active else Vector2(6,24)
	layout_switch.rect_size = Vector2(adaptive.rail-12 if adaptive_active else max(60,game_origin.x-12),26)

func toggle_exploration_layout():
	if not adaptive_available or not adaptive_widescreen():
		return
	prefer_classic_now = not prefer_classic_now
	sync_adaptive_layout()

func game_to_display(point, target_id = ""):
	return adaptive.game_to_screen(point, target_id) if adaptive_active else game_origin + point

func display_to_game(point):
	return adaptive.screen_to_game(point) if adaptive_active else point - game_origin

func pointer_in_picture(point):
	var local = display_to_game(point)
	return Rect2(0,0,512,384).has_point(local)

func selection_circle(target):
	if target.id == "ui:layout":
		return {"center":layout_switch.rect_position+layout_switch.rect_size/2-(Vector2.ZERO if adaptive_active else game_origin),"radius":16}
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
		draw_texture_rect_region(adaptive_atlas_texture,destination,Rect2(c.slot*128,0,c.w,c.h))
		var font = get_font_for("11px Arial")
		draw_string(font,c.display.position+Vector2(6,74),c.label,UIStyle.INK)
	# Keep the entire arrow visible at the bottom of the expanded world.
	if not adaptive.navigation.empty():
		var n = adaptive.navigation
		draw_texture_rect_region(adaptive_atlas_texture,adaptive.navigation_display,Rect2(n.slot*128,0,n.w,n.h))

func _draw():
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

func draw_cursor():
	if touch_enabled and modal != null:
		return
	if modal == null and controller_selection >= 0 and controller_selection < controller_surface.targets.size():
		var target = controller_surface.targets[controller_selection]
		var circle = selection_circle(target)
		cursor_layer.draw_arc(circle.center, circle.radius, 0, TAU, 64, Color(0.78,0.68,0.45,0.50), 1.4, true)
	if not pointer_visible or (pointer_name == "none" and modal == null):
		return
	var cursor = display_pointer if adaptive_active else pointer
	var color = Color("f7e6a4") if pointer_name in ["touch", "hand", "fist"] else Color.white
	var points = PoolVector2Array([cursor, cursor + Vector2(0, 15), cursor + Vector2(4, 11), cursor + Vector2(8, 18), cursor + Vector2(11, 16), cursor + Vector2(7, 9), cursor + Vector2(13, 9)])
	cursor_layer.draw_colored_polygon(points, Color(0,0,0))
	cursor_layer.draw_polyline(PoolVector2Array([cursor + Vector2(1, 2), cursor + Vector2(1, 12), cursor + Vector2(4, 9), cursor + Vector2(9, 16)]), color, 1.5, true)
	if pointer_name.begins_with("go"):
		cursor_layer.draw_circle(cursor + Vector2(15, 4), 3, Color("f7e6a4"))

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
	refresh_cursor()
	if runtime != null:
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
				has_frame = true
			else:
				frame_texture.set_data(frame_image)
			overlays = JSON.parse(runtime.query("overlay")).result
			update()
		var present_finished = OS.get_ticks_usec()
		var events = JSON.parse(runtime.query("events")).result
		for event in events:
			handle_event(event)
		if pending_restart != null:
			start_runtime(pending_restart)
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
	cursor_layer.visible = not ((touch_enabled and modal != null) or (pointer_name == "none" and modal == null))
	var origin = Vector2.ZERO if adaptive_active else game_origin
	if cursor_layer.position != origin:
		cursor_layer.position = origin
	if drawn_pointer != pointer or drawn_pointer_name != pointer_name or drawn_display_pointer != display_pointer:
		drawn_pointer = pointer
		drawn_display_pointer = display_pointer
		drawn_pointer_name = pointer_name
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
		"saved":
			print("Saved: ", event.name)

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
		if Rect2(0, 0, 512, 384).has_point(local):
			pointer = local
		if event is InputEventMouseMotion and controller_selection >= 0:
			controller_selection = -1
			cursor_layer.update()
	if event is InputEventKey and event.pressed and not event.echo:
		if event.scancode == KEY_F11 or (event.scancode == KEY_F and event.control and event.meta):
			OS.window_fullscreen = not OS.window_fullscreen
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
		var keys = {KEY_UP: "uparrow", KEY_DOWN: "downarrow", KEY_LEFT: "leftarrow", KEY_RIGHT: "rightarrow", KEY_ESCAPE: ".", KEY_ENTER: "\r", KEY_BACKSPACE: "\b", KEY_TAB: "\t"}
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
	event.position = display_pointer if adaptive_active else pointer + game_origin
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
		var speed = 65.0 if controller_action_held("precision") else 240.0
		var step = motion.normalized() * pow(min(1.0, (motion.length() - deadzone) / (1.0 - deadzone)), 1.5) * speed * delta
		if adaptive_active:
			display_pointer = Vector2(clamp(display_pointer.x + step.x,0,layout_size.x-1),clamp(display_pointer.y + step.y,0,layout_size.y-1))
		else:
			pointer = Vector2(clamp(pointer.x + step.x, 0, 511), clamp(pointer.y + step.y, 0, 383))
		var event = InputEventMouseMotion.new()
		event.position = display_pointer if adaptive_active else pointer + game_origin
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
	var surface = JSON.parse(runtime.query("targets" if include_room else "controls")).result
	if surface is Dictionary:
		set_controller_surface(surface)

func set_controller_surface(surface):
	if adaptive_available and surface.context == "room" and not surface.targets.empty():
		surface.targets = surface.targets.duplicate()
		surface.targets.append({"id":"ui:layout","aim_x":-1,"aim_y":-1,"w":32,"h":26})
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
	var changed = JSON.print(controller_surface) != JSON.print(surface)
	controller_surface = surface
	update_touch_context()
	if not changed:
		return
	controller_selection = -1
	for i in range(surface.targets.size()):
		if surface.targets[i].id == old_id:
			controller_selection = i
	if controller_selection < 0 and surface.context != "room" and not surface.targets.empty():
		controller_selection = 0
	if controller_selection >= 0 and not pointer_pressed:
		focus_controller_target(controller_selection)
	cursor_layer.update()

func controller_target_position(target):
	if target.id == "ui:layout":
		return layout_switch.rect_position+layout_switch.rect_size/2
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
	set_pointer_visible(false)
	var target = controller_surface.targets[index]
	if target.id == "ui:layout":
		display_pointer = layout_switch.rect_position+layout_switch.rect_size/2
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
		set_pointer_visible(controller_selection < 0)
		if controller_selection >= 0 and controller_surface.targets[controller_selection].id == "ui:layout":
			toggle_exploration_layout()
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
		"confirm": ["b:" + str(JOY_BUTTON_0), "b:" + str(JOY_R), "k:" + str(KEY_ENTER)],
		"back": ["b:" + str(JOY_BUTTON_1), "k:" + str(KEY_ESCAPE)],
		"menu": ["b:" + str(JOY_START), "k:" + str(KEY_F10)],
		"door": ["b:" + str(JOY_BUTTON_2), "k:" + str(KEY_SPACE)],
		"keyboard": ["b:" + str(JOY_BUTTON_3), "k:" + str(KEY_F12)],
		"mouse": ["b:" + str(JOY_R3)], "precision": ["b:" + str(JOY_L)],
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
		# Keep normal typing and keyboard shortcuts in text fields.
		if get_focus_owner() is LineEdit or event.control or event.meta or event.alt:
			return false
		# Keyboard arrows also cover Android controllers that report a D-pad as keys.
		var portable = not OS.get_environment("RETANIC_ARCH").empty() or OS.get_name() == "Android"
		var navigation = event.scancode in [KEY_UP, KEY_DOWN, KEY_LEFT, KEY_RIGHT]
		var custom = config.get_value(controller_config_section(), "keyboard_remapped", false)
		if not portable and not navigation and not custom:
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
		return event.scancode in [KEY_UP, KEY_DOWN, KEY_LEFT, KEY_RIGHT, KEY_ENTER, KEY_ESCAPE, KEY_F10, KEY_F12, KEY_SPACE, KEY_PAGEUP, KEY_PAGEDOWN]
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
	hint.text = "Side panels work on wide screens. Conversations and held items use Classic. Change your choice later in Menu > Controls / display."
	hint.autowrap = true
	hint.rect_min_size = Vector2(390,64)
	hint.add_font_override("font",get_font_for("13px Arial"))
	box.add_child(hint)
	button(box,"Classic (default)","choose_first_layout",[0]).grab_focus()
	button(box,"Side panels: Compact","choose_first_layout",[1]).disabled = classic_format_device()
	button(box,"Side panels: Roomy","choose_first_layout",[2]).disabled = classic_format_device()

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
		text_scroller(box,"Side panels need a wide screen. Conversations, held items and special screens use Classic.",40)
	var bindings = button(box,"Controller bindings","show_controller_settings")
	if classic_format_device():
		bindings.grab_focus()
	button(box,"Touch controls","show_touch_help")
	button(box,"Back","controller_settings_back")

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
	return "adaptive" if adaptive_active else ("portrait" if OS.window_size.y > OS.window_size.x else "landscape")

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
	text_scroller(box, "Left stick / D-pad: move, or select dialogue replies and menus.\nA / R1: confirm or click. Hold to drag.\nB: back or skip speech / movies.\nL2 / R2: previous / next clickable target.\nRight stick: mouse pointer. Hold L1 for precision.\nX: door / Space. Y: keyboard.\nStart: voyage menu.\n\nUse the D-pad and A on the keyboard to enter save names or solve text puzzles.", 230)
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
	controller_selection = -1
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
	button(box, "Save game", "menu_command", ["save"])
	button(box, "Load game", "menu_command", ["load"])
	button(box, "Import / export .ti saves", "save_tools")
	button(box, "Game files / mods", "setup_from_menu")
	button(box, "Main menu", "menu_command", ["new"])
	button(box, "Controls / display", "show_control_options")
	button(box, "Credits", "show_credits")
	button(box, "Quit", "quit_game")

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
	if not hd_pack_path.empty():
		button(box, "HD artwork: %s (next start)" % ("On" if config.get_value("graphics", "hd_enabled", true) else "Off"), "toggle_hd_artwork")
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
		var b = button(list, saves.display_name(name), "save_selected", [name, action])
		b.clip_text = true
		b.hint_tooltip = saves.display_name(name)
		b.set_meta("autosave", saves.is_autosave(name))
		if saves.is_autosave(name):
			for state in ["font_color", "font_color_hover", "font_color_pressed", "font_color_focus"]:
				b.add_color_override(state, Color("b5d8ef"))
		if first == null:
			first = b
	button(box, "Cancel", "reply_dialog", [null]).grab_focus()
	if first != null:
		first.grab_focus()

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
	if adaptive_enabled() and adaptive_widescreen():
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
	status.rect_position = game_origin + Vector2(20, 160)
	if is_instance_valid(checkpoint_toast):
		checkpoint_toast.rect_position = game_origin + Vector2(16, 16)
	for child in get_children():
		if child is PanelContainer:
			child.rect_position = (layout_size - child.rect_size) / 2
	if touch_strip == null:
		return
	touch_strip.visible = touch_enabled and modal == null
	var specs
	if adaptive_active:
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
