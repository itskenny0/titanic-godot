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
	print("CONTROLLER ", "FAIL" if failed else "PASS")
	quit(1 if failed else 0)

func joy_button(player, index, pressed):
	var event = InputEventJoypadButton.new()
	event.button_index = index
	event.pressed = pressed
	player._input(event)
