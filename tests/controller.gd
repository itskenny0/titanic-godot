extends SceneTree
class Recorder:
	extends Reference
	var commands = []
	var surface = {"context": "room", "key": "room", "targets": []}
	func query(_method):
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
func begin():
	var player = load("res://Main.tscn").instance()
	get_root().add_child(player)
	player.set_process(false)
	player.close_modal()
	var recorder = Recorder.new()
	player.runtime = recorder
	player.ready = true
	yield(self, "idle_frame")
	player.pointer = Vector2(100, 150)
	player.mouse_button(true)
	for step in range(120):
		var expected = Vector2(101 + step, 150)
		var motion = InputEventMouseMotion.new()
		motion.position = player.game_origin + expected
		motion.global_position = motion.position
		motion.relative = Vector2.RIGHT
		player.dispatch_pointer(motion)
		yield(self, "idle_frame")
		check(player.pointer.distance_to(expected) < 0.01, "held controller pointer keeps viewport coordinates")
	player.mouse_button(false)
	check(recorder.commands.front().kind == "press", "controller drag starts with press")
	check(recorder.commands.back().kind == "release" and recorder.commands.back().x == 220, "controller drag ends at final pointer")
	check(not player.pointer_pressed, "controller release clears held click")
	recorder.commands.clear()
	recorder.surface = {"context": "dialogue", "key": "conversation", "targets": [
		{"id": "first", "label": "First reply", "x": 0, "y": 264, "w": 512, "h": 24, "aim_x": 256, "aim_y": 276},
		{"id": "second", "label": "Second reply", "x": 0, "y": 288, "w": 512, "h": 24, "aim_x": 256, "aim_y": 300}]}
	player.refresh_controller_surface()
	check(player.controller_selection == 0, "dialogue focuses first reply")
	check(not player.pointer_visible, "controller focus hides mouse pointer")
	player.controller_direction("downarrow")
	check(player.controller_selection == 1, "D-pad selects next reply")
	player.controller_confirm(true)
	player.controller_confirm(false)
	check(recorder.commands.back().kind == "release" and recorder.commands.back().y == 300, "confirm clicks selected reply through mouse dispatch")
	for command in recorder.commands:
		check(command.action != "key", "dialogue directions never leak into room movement")
	player.controller_direction("downarrow")
	check(player.controller_selection == 0, "dialogue wraps at last reply")
	# Modifier shortcuts must bypass controller mapping on both engine versions.
	for modifier in ["alt", "control", "meta"]:
		var shortcut = InputEventKey.new()
		shortcut.scancode = KEY_DOWN
		shortcut.pressed = true
		var property = modifier
		if Engine.get_version_info().major >= 4:
			property = {"alt": "alt_pressed", "control": "ctrl_pressed", "meta": "meta_pressed"}[modifier]
		shortcut.set(property, true)
		check(not player.controller_binding_input(shortcut), modifier + " shortcut bypasses controller mapping")
	# PortMaster sends keys through gptokeyb instead of native joypad events.
	var previous_arch = OS.get_environment("RETANIC_ARCH")
	OS.set_environment("RETANIC_ARCH", "aarch64")
	var mapped = InputEventKey.new()
	mapped.scancode = KEY_DOWN
	mapped.pressed = true
	Input.parse_input_event(mapped)
	yield(self, "idle_frame")
	check(player.controller_selection == 1, "PortMaster direction selects dialogue reply")
	mapped.scancode = KEY_ENTER
	Input.parse_input_event(mapped)
	yield(self, "idle_frame")
	mapped.pressed = false
	Input.parse_input_event(mapped)
	yield(self, "idle_frame")
	check(recorder.commands.back().kind == "release" and recorder.commands.back().y == 300, "PortMaster A confirms selected reply")
	OS.set_environment("RETANIC_ARCH", previous_arch)
	# Mouse movement must take over without focus snapping back on the next poll.
	var motion = InputEventMouseMotion.new()
	motion.position = player.game_origin + Vector2(70, 80)
	motion.relative = Vector2(1, 0)
	player.dispatch_pointer(motion)
	player.refresh_controller_surface()
	check(player.controller_selection == -1 and player.pointer == Vector2(70, 80), "free pointer overrides dialogue focus")
	check(player.pointer_visible, "pointer movement reveals mouse pointer")
	recorder.surface = {"context": "busy", "key": "conversation", "targets": []}
	player.refresh_controller_surface()
	check(player.controller_selection == -1, "old reply disappears during speech")
	# Inventory and puzzle directions follow visible positions, not file order.
	recorder.surface = {"context":"panel","key":"inventory-grid","targets":[
		{"id":"bottom-right","aim_x":200,"aim_y":210,"w":24,"h":24},
		{"id":"top-right","aim_x":200,"aim_y":90,"w":24,"h":24},
		{"id":"bottom-left","aim_x":80,"aim_y":200,"w":24,"h":24},
		{"id":"top-left","aim_x":80,"aim_y":80,"w":24,"h":24}]}
	player.refresh_controller_surface()
	check(player.controller_surface.targets[player.controller_selection].id=="top-left","initial focus starts at top left")
	for move in [["downarrow","bottom-left"],["rightarrow","bottom-right"],["uparrow","top-right"],["leftarrow","top-left"],["uparrow","top-left"]]:
		player.controller_direction(move[0])
		check(player.controller_surface.targets[player.controller_selection].id==move[1],"spatial inventory selection: "+move[0])
	recorder.surface.targets.invert()
	player.refresh_controller_surface()
	check(player.controller_surface.targets[player.controller_selection].id=="top-left","source order cannot move selection")
	player.controller_cycle(1)
	check(player.controller_surface.targets[player.controller_selection].id=="top-right","shoulder cycles in visible reading order")
	player.controller_confirm(true)
	player.controller_confirm(false)
	check(recorder.commands.back().x==200 and recorder.commands.back().y==90,"spatially selected item clicks original position")
	# Native controller buttons navigate Godot dialogs, without a mouse.
	player.handle_event({"type": "dialog", "kind": "question", "id": 21, "text": "Continue?"})
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	player.controller_direction("downarrow")
	yield(self, "idle_frame")
	check(player.get_focus_owner().text == "Cancel", "D-pad changes dialog focus")
	joy_button(player, JOY_BUTTON_0, true)
	joy_button(player, JOY_BUTTON_0, false)
	yield(self, "idle_frame")
	check(player.current_dialog == -1, "A confirms focused dialog control")
	check(recorder.commands.back().action == "reply" and recorder.commands.back().value == false, "question cancellation is sent once")
	player.handle_event({"type": "dialog", "kind": "save", "id": 22, "text": "Save game", "value": "Voyage"})
	yield(self, "idle_frame")
	joy_button(player, JOY_BUTTON_3, true)
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	check(player.virtual_keyboard != null, "Y opens save-name keyboard")
	joy_button(player, JOY_BUTTON_0, true)
	joy_button(player, JOY_BUTTON_0, false)
	yield(self, "idle_frame")
	check(player.save_name.text == "Voyage1", "A types focused on-screen key")
	joy_button(player, JOY_BUTTON_1, true)
	check(player.virtual_keyboard == null and player.current_dialog == 22, "B returns to save dialog")
	check(player.get_focus_owner().text == "Save", "keyboard restores dialog focus")
	joy_button(player, JOY_BUTTON_1, true)
	check(player.current_dialog == -1, "B cancels save without writing")
	# Bindings must work from events even if the OS lists another joypad first.
	player.controller_held.clear()
	recorder.surface = {"context": "room", "key": "room", "targets": []}
	recorder.commands.clear()
	var dpad = InputEventJoypadButton.new()
	dpad.device = 7
	dpad.button_index = JOY_DPAD_DOWN
	dpad.pressed = true
	player._input(dpad)
	check(recorder.commands.back().key == "downarrow", "D-pad event moves without a connected analog device")
	player.process_controller(0.31)
	check(recorder.commands.back().key == "downarrow", "held D-pad repeats")
	dpad.pressed = false
	player._input(dpad)
	check(not player.controller_action_held("down"), "D-pad release stops repeat")
	# Android D-pads can arrive as ordinary arrow keys.
	mapped.scancode = KEY_UP
	mapped.pressed = true
	player._input(mapped)
	check(recorder.commands.back().key == "uparrow", "keyboard D-pad moves outside PortMaster")
	mapped.pressed = false
	player._input(mapped)
	player.show_menu()
	player.show_controller_settings()
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	var bounds = player.modal.get_global_rect()
	check(bounds.position.x >= 0 and bounds.position.y >= 0 and bounds.end.x <= player.layout_size.x and bounds.end.y <= player.layout_size.y, "remapping menu fits handheld viewport")
	VisualServer.force_draw()
	var screenshot = get_root().get_texture().get_data()
	screenshot.flip_y()
	screenshot.save_png("user://controller-remap.png")
	player.begin_controller_remap("confirm")
	joy_button(player, JOY_BUTTON_3, true)
	check(player.remap_action.empty(), "button capture completes")
	check(player.modal != null and player.virtual_keyboard == null, "captured Y never opens keyboard")
	joy_button(player, JOY_BUTTON_3, false)
	var bindings = player.controller_bindings.duplicate(true)
	player.config = ConfigFile.new()
	check(player.config.load("user://settings.cfg") == OK, "saved bindings file loads")
	player.load_controller_bindings()
	check(JSON.print(player.controller_bindings) == JSON.print(bindings), "remapping survives config reload")
	check("b:" + str(JOY_BUTTON_3) in player.controller_bindings.confirm, "Y now confirms")
	check("b:" + str(JOY_BUTTON_0) in player.controller_bindings.keyboard, "conflicting keyboard action swaps to A")
	player.begin_controller_remap("up")
	dpad.button_index = 25
	dpad.pressed = true
	player._input(dpad)
	dpad.pressed = false
	player._input(dpad)
	player.controller_settings_back()
	player.resume_game()
	dpad.pressed = true
	player._input(dpad)
	check(recorder.commands.back().key == "uparrow", "unusual D-pad button can be remapped")
	dpad.pressed = false
	player._input(dpad)
	player.show_menu()
	player.show_controller_settings()
	player.reset_controller_bindings()
	check(JSON.print(player.controller_bindings) == JSON.print(player.default_controller_bindings()), "reset restores every default")
	player.begin_controller_remap("back")
	player.cancel_controller_remap()
	check(player.remap_action.empty(), "capture can be cancelled")
	player.begin_controller_remap("up")
	mapped.scancode = KEY_Z
	mapped.pressed = true
	player._input(mapped)
	mapped.pressed = false
	player._input(mapped)
	player.controller_settings_back()
	player.resume_game()
	mapped.pressed = true
	player._input(mapped)
	check(recorder.commands.back().key == "uparrow", "keyboard-style D-pad can be remapped")
	mapped.pressed = false
	player._input(mapped)
	mapped.scancode = KEY_T
	mapped.pressed = true
	check(not player.controller_binding_input(mapped), "remapping leaves ordinary puzzle typing available")
	player.show_menu()
	player.show_controller_settings()
	player.reset_controller_bindings()
	player.controller_settings_back()
	check(player.menu == player.modal, "Back returns to voyage menu")
	print("CONTROLLER ", "FAIL" if failed else "PASS")
	quit(1 if failed else 0)

func joy_button(player, index, pressed):
	var event = InputEventJoypadButton.new()
	event.button_index = index
	event.pressed = pressed
	player._input(event)
