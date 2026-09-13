extends Control

const GameFiles = preload("res://scripts/game_files.gd")
const SaveStore = preload("res://scripts/save_store.gd")
var runtime
var files = GameFiles.new()
var saves = SaveStore.new()
var config = ConfigFile.new()
var game_index = {}
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
var current_dialog = -1
var queued_dialogs = []
var focused = true
var manual_pause = false
var ready = false
var runtime_failed = false
var nav_timer = 0.0
var last_nav = ""
var controller_surface = {"context": "busy", "key": "", "targets": []}
var controller_selection = -1
var controller_poll = 0.0
var controller_triggers = [false, false]
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
	Engine.target_fps = 30 if not OS.get_environment("RETANIC_ARCH").empty() else 60
	OS.low_processor_usage_mode = true
	get_tree().set_auto_accept_quit(false)
	get_tree().set_quit_on_go_back(false)
	get_tree().connect("files_dropped", self, "files_dropped")
	config.load("user://settings.cfg")
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
		for candidate in ["user://gamedata", base.plus_file("gamedata"), ProjectSettings.globalize_path("res://../gamedata")]:
			if not files.discover(candidate).empty():
				root = candidate
				break
	var roots = files.discover(root) if not root.empty() else []
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
	runtime = create_runtime()
	if runtime == null:
		return
	var error = ""
	runtime.execute("profile", JSON.print({"on": OS.is_debug_build()}))
	error = runtime.execute("boot", JSON.print({"index": game_index, "save": save_path, "testing": "--integration-test" in OS.get_cmdline_args()}))
	if not error.empty():
		show_note(error)
		return
	status.show()
	status.text = "Preparing your voyage…"

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

func bridge_call(method, args_json, bytes):
	var args = JSON.parse(args_json).result
	match method:
		"read":
			var path = args.path
			if path.begins_with("save:"):
				path = saves.path_for(path.substr(5))
			var f = File.new()
			if f.open(path, File.READ) != OK:
				return null
			var data = f.get_buffer(f.get_len())
			f.close()
			return data
		"write":
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

func _draw():
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
		cursor_layer.draw_rect(Rect2(target.x + 1, target.y + 1, max(1, target.w - 2), max(1, target.h - 2)), Color("f7d777"), false, 2)
	if not pointer_visible or (pointer_name == "none" and modal == null):
		return
	var color = Color("f7e6a4") if pointer_name in ["touch", "hand", "fist"] else Color.white
	var points = PoolVector2Array([pointer, pointer + Vector2(0, 15), pointer + Vector2(4, 11), pointer + Vector2(8, 18), pointer + Vector2(11, 16), pointer + Vector2(7, 9), pointer + Vector2(13, 9)])
	cursor_layer.draw_colored_polygon(points, Color.black)
	cursor_layer.draw_polyline(PoolVector2Array([pointer + Vector2(1, 2), pointer + Vector2(1, 12), pointer + Vector2(4, 9), pointer + Vector2(9, 16)]), color, 1.5, true)
	if pointer_name.begins_with("go"):
		cursor_layer.draw_circle(pointer + Vector2(15, 4), 3, Color("f7e6a4"))

func send(command):
	if runtime_failed and command.get("action", "") in ["key", "pointer"]:
		return
	if runtime != null:
		var started = OS.get_ticks_usec()
		runtime.execute("command", JSON.print(command))
		var elapsed = OS.get_ticks_usec() - started
		if OS.is_debug_build() and elapsed > 50000:
			print("PERF command ", command.get("action", ""), " ", elapsed / 1000.0, " ms")

func _process(delta):
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
		if bytes.size() == 512 * 384 * 4:
			frame_image.create_from_data(512, 384, false, Image.FORMAT_RGBA8, bytes)
			if not has_frame:
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
	if cursor_layer.position != game_origin:
		cursor_layer.position = game_origin
	if drawn_pointer != pointer or drawn_pointer_name != pointer_name:
		drawn_pointer = pointer
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
				player.stream_paused = audio_paused
		"quit":
			get_tree().quit()
		"restart":
			pending_restart = event.get("save", "")
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
	player.stream_paused = audio_paused

func sound_finished(id):
	stop_sound(id)
	send({"action": "audio_done", "id": id})

func stop_sound(id):
	if sounds.has(id):
		sounds[id].stop()
		sounds[id].queue_free()
		sounds.erase(id)

func _notification(what):
	if what == MainLoop.NOTIFICATION_WM_FOCUS_OUT:
		focused = false
		send({"action": "pause", "on": true})
	elif what == MainLoop.NOTIFICATION_WM_FOCUS_IN:
		focused = true
		send({"action": "pause", "on": manual_pause})
	elif what == MainLoop.NOTIFICATION_WM_GO_BACK_REQUEST:
		if virtual_keyboard != null:
			close_keyboard()
		elif menu != null:
			resume_game()
		elif modal == null:
			show_menu()
	elif what == MainLoop.NOTIFICATION_WM_QUIT_REQUEST:
		show_menu()

func _input(event):
	if event is InputEventMouseMotion or event is InputEventMouseButton:
		if not controller_pointer_event:
			set_pointer_visible(true)
		var local = event.position - game_origin
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
	if event is InputEventKey and not OS.get_environment("RETANIC_ARCH").empty():
		if event.pressed:
			set_pointer_visible(false)
		if event.scancode == KEY_ENTER:
			controller_confirm(event.pressed)
			get_tree().set_input_as_handled()
			return
		if event.pressed and event.scancode in [KEY_UP, KEY_DOWN, KEY_LEFT, KEY_RIGHT]:
			if modal == null:
				controller_direction({KEY_UP: "uparrow", KEY_DOWN: "downarrow", KEY_LEFT: "leftarrow", KEY_RIGHT: "rightarrow"}[event.scancode])
				get_tree().set_input_as_handled()
				return
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
	if event is InputEventJoypadButton:
		if event.pressed:
			set_pointer_visible(false)
		if event.button_index == JOY_START and event.pressed:
			show_menu()
			get_tree().set_input_as_handled()
		elif event.button_index == JOY_R3:
			if event.pressed:
				set_pointer_visible(true)
			mouse_button(event.pressed)
			get_tree().set_input_as_handled()
		elif event.button_index in [JOY_BUTTON_0, JOY_R]:
			controller_confirm(event.pressed)
			get_tree().set_input_as_handled()
		elif event.button_index == JOY_BUTTON_1:
			if event.pressed:
				controller_back()
			get_tree().set_input_as_handled()
		elif event.button_index == JOY_BUTTON_3:
			if event.pressed:
				controller_keyboard()
			get_tree().set_input_as_handled()
		elif event.button_index == JOY_BUTTON_2:
			if event.pressed and modal == null:
				send({"action": "key", "key": " "})
			get_tree().set_input_as_handled()
		elif event.button_index in [JOY_DPAD_UP, JOY_DPAD_DOWN, JOY_DPAD_LEFT, JOY_DPAD_RIGHT]:
			# The held-input loop owns repeat, including in Godot dialogs.
			get_tree().set_input_as_handled()

	if event is InputEventJoypadMotion and event.axis in [JOY_AXIS_6, JOY_AXIS_7]:
		var index = 0 if event.axis == JOY_AXIS_6 else 1
		var pressed = event.axis_value > 0.5
		if pressed and not controller_triggers[index]:
			controller_cycle(-1 if index == 0 else 1)
		controller_triggers[index] = pressed

func _unhandled_input(event):
	if modal != null or not ready:
		return
	if event is InputEventMouseMotion:
		send({"action": "pointer", "kind": "move", "x": pointer.x, "y": pointer.y})
	elif event is InputEventMouseButton and event.button_index == BUTTON_LEFT:
		if event.pressed and not Rect2(game_origin, Vector2(512, 384)).has_point(event.position):
			return
		if not event.pressed and not pointer_pressed:
			return
		pointer_pressed = event.pressed
		send({"action": "pointer", "kind": "press" if event.pressed else "release", "x": pointer.x, "y": pointer.y, "shift": event.shift})
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

func mouse_button(pressed):
	var event = InputEventMouseButton.new()
	event.button_index = BUTTON_LEFT
	event.pressed = pressed
	event.position = pointer + game_origin
	event.global_position = pointer + game_origin
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
	var pads = Input.get_connected_joypads()
	controller_poll -= delta
	if controller_poll <= 0 and (not pads.empty() or not OS.get_environment("RETANIC_ARCH").empty()):
		refresh_controller_surface()
		controller_poll = 0.1
	if pads.empty():
		return
	var pad = pads[0]
	var motion = Vector2(Input.get_joy_axis(pad, JOY_AXIS_2), Input.get_joy_axis(pad, JOY_AXIS_3))
	var deadzone = 0.2
	if motion.length() > deadzone:
		controller_selection = -1
		var speed = 65.0 if Input.is_joy_button_pressed(pad, JOY_L) else 240.0
		var step = motion.normalized() * pow(min(1.0, (motion.length() - deadzone) / (1.0 - deadzone)), 1.5) * speed * delta
		pointer = Vector2(clamp(pointer.x + step.x, 0, 511), clamp(pointer.y + step.y, 0, 383))
		var event = InputEventMouseMotion.new()
		event.position = pointer + game_origin
		event.global_position = pointer + game_origin
		event.relative = step
		dispatch_pointer(event)
	var movement = Vector2(Input.get_joy_axis(pad, JOY_AXIS_0), Input.get_joy_axis(pad, JOY_AXIS_1))
	var key = ""
	if movement.length() > 0.4:
		if abs(movement.x) > abs(movement.y):
			key = "leftarrow" if movement.x < 0 else "rightarrow"
		else:
			key = "uparrow" if movement.y < 0 else "downarrow"
	for pair in [[JOY_DPAD_UP, "uparrow"], [JOY_DPAD_DOWN, "downarrow"], [JOY_DPAD_LEFT, "leftarrow"], [JOY_DPAD_RIGHT, "rightarrow"]]:
		if Input.is_joy_button_pressed(pad, pair[0]):
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
	var same_place = controller_surface.context == surface.context and controller_surface.key == surface.key
	if same_place and surface.context == "room" and surface.targets.empty():
		return
	var old_id = ""
	if same_place and controller_selection >= 0 and controller_selection < controller_surface.targets.size():
		old_id = controller_surface.targets[controller_selection].id
	var changed = JSON.print(controller_surface) != JSON.print(surface)
	controller_surface = surface
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

func focus_controller_target(index):
	if index < 0 or index >= controller_surface.targets.size():
		return
	controller_selection = index
	set_pointer_visible(false)
	var target = controller_surface.targets[index]
	pointer = Vector2(target.aim_x, target.aim_y)
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
			var step = -1 if key in ["uparrow", "leftarrow"] else 1
			focus_controller_target(0 if controller_selection < 0 else (controller_selection + step + count) % count)
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

func show_controller_help():
	var box = panel("Controller controls")
	text_scroller(box, "Left stick / D-pad: move, or select dialogue replies and menus.\nA / R1: confirm or click. Hold to drag.\nB: back or skip speech / movies.\nL2 / R2: previous / next clickable target.\nRight stick: mouse pointer. Hold L1 for precision.\nX: door / Space. Y: keyboard.\nStart: voyage menu.\n\nUse the D-pad and A on the keyboard to enter save names or solve text puzzles.", 230)
	button(box, "Back", "resume_game").grab_focus()

func panel(title):
	if pointer_pressed:
		send({"action": "pointer", "kind": "release", "x": pointer.x, "y": pointer.y})
		pointer_pressed = false
	close_modal()
	var p = PanelContainer.new()
	p.rect_position = (layout_size - Vector2(420, 310)) / 2
	p.rect_min_size = Vector2(420, 310)
	p.add_font_override("font", get_font_for("14px Arial"))
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
	if runtime == null:
		status.hide()
	return box

func button(box, title, method, args = []):
	var b = Button.new()
	b.text = title
	b.rect_min_size.y = 25
	b.add_font_override("font", get_font_for("14px Arial"))
	box.add_child(b)
	b.connect("pressed", self, method, args)
	return b

func close_modal():
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
	button(box, "Controller controls", "show_controller_help")
	button(box, "Credits", "show_credits")
	button(box, "Quit", "quit_game")

func resume_game():
	manual_pause = false
	close_modal()
	menu = null
	send({"action": "pause", "on": not focused})

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
	label.text = "Choose prepared cd1 and cd2 folders.\nUse tools/prepare-game-data.py for a LOCAL install.\nChanging files restarts the game; save first."
	label.add_font_override("font", get_font_for("13px Arial"))
	box.add_child(label)
	button(box, "Choose game folder", "choose_data", [0]).grab_focus()
	button(box, "Choose discs separately", "choose_data", [1])
	button(box, "M3tox patches", "show_patch_picker", [false])
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
		text_scroller(box, "Choose your prepared game folder. Copying the files can take a few minutes. Keep Titanic open until it finishes.", 210)
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
		show_note(files.error if not files.error.empty() else "Select a parent containing cd1 and cd2.")
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
	if kind == "save" and File.new().file_exists(saves.path_for(text)):
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
		var b = button(list, name, "save_selected", [name, action])
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
		d.current_file = name

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
var touch_enabled = false
var touch_click_down = false
var touch_nav_timer = 0.0
var touch_nav_direction = ""

func update_touch_layout(_device = 0, _connected = false):
	touch_enabled = (OS.get_name() == "Android" or OS.has_touchscreen_ui_hint() or "--touch-test" in OS.get_cmdline_args()) and Input.get_connected_joypads().empty()
	var physical = OS.window_size
	var portrait = physical.y > physical.x
	var desired = Vector2(512, 384)
	if touch_enabled:
		desired = Vector2(512, max(640, 512.0 * physical.y / max(1, physical.x))) if portrait else Vector2(max(640, 384.0 * physical.x / max(1, physical.y)), 384)
	# Keep the picture and its 536-pixel control block within thumb reach.
	game_origin = Vector2(0, max(12, desired.y - 548)) if touch_enabled and portrait else Vector2((desired.x - 512) / 2, 0)
	if desired != layout_size:
		layout_size = desired
		get_tree().set_screen_stretch(SceneTree.STRETCH_MODE_2D, SceneTree.STRETCH_ASPECT_KEEP, layout_size)
	status.rect_position = game_origin + Vector2(20, 160)
	for child in get_children():
		if child is PanelContainer:
			child.rect_position = (layout_size - child.rect_size) / 2
	if touch_strip == null:
		return
	touch_strip.visible = touch_enabled and modal == null
	var specs
	if portrait:
		var y = game_origin.y + 420
		touch_strip.get_node("joystick").configure(Vector2(112, y + 58), 58)
		specs = [["space", "Door", Vector2(264,y+64)], ["escape", "Skip", Vector2(336,y+64)], ["menu", "Menu", Vector2(264,y)], ["keyboard", "Keys", Vector2(336,y)]]
	else:
		var right = game_origin.x + 512 + max(0, (game_origin.x - 52) / 2)
		touch_strip.get_node("joystick").configure(Vector2(game_origin.x / 2, 192), clamp((game_origin.x - 8) / 2, 28, 64))
		specs = [["menu", "Menu", Vector2(right,72)], ["escape", "Skip", Vector2(right,132)], ["space", "Door", Vector2(right,192)], ["keyboard", "Keys", Vector2(right,252)]]
	for spec in specs:
		touch_strip.get_node(spec[0]).position = spec[2]
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
		var t = TouchScreenButton.new()
		t.name = spec[0]
		t.action = action
		var img = Image.new()
		img.create(52, 52, false, Image.FORMAT_RGBA8)
		img.fill(Color("263a4d"))
		var texture = ImageTexture.new()
		texture.create_from_image(img, 0)
		t.normal = texture
		touch_strip.add_child(t)
		var text = Label.new()
		text.text = spec[1]
		text.rect_position = Vector2(1,17)
		text.rect_size = Vector2(50,20)
		text.align = Label.ALIGN_CENTER
		text.add_font_override("font", get_font_for("14px Arial"))
		text.mouse_filter = Control.MOUSE_FILTER_IGNORE
		t.add_child(text)
	Input.connect("joy_connection_changed", self, "update_touch_layout")
	# Resizing content inside this signal re-enters the engine's viewport resize.
	# Let the current resize finish before updating the logical canvas.
	get_viewport().connect("size_changed", self, "update_touch_layout", [], CONNECT_DEFERRED)
	update_touch_layout()

func touch_door():
	if ready and modal == null:
		send({"action": "key", "key": " "})

func touch_direction(direction):
	if ready and modal == null:
		touch_nav_direction = direction
		touch_nav_timer = 0.55
		send({"action": "key", "key": direction + "arrow"})

func process_touch(delta):
	touch_strip.visible = touch_enabled and modal == null
	if not touch_enabled or modal != null:
		touch_nav_direction = ""
		touch_nav_timer = 0.0
		return
	for pair in [["space", " "], ["escape", "."]]:
		if Input.is_action_just_pressed("touch_" + pair[0]):
			send({"action": "key", "key": pair[1], "special": pair[0] == "escape"})
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
			send({"action": "key", "key": direction + "arrow"})
	elif not direction.empty():
		touch_nav_timer -= delta
		if touch_nav_timer <= 0:
			send({"action": "key", "key": direction + "arrow"})
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
