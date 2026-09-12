extends SceneTree
class Recorder:
	extends Reference
	var commands = []
	func execute(code):
		if code.begins_with("titanicCommand("):
			commands.append(JSON.parse(code.substr(15, code.length()-16)).result)
		return ""
var player
var recorder = Recorder.new()
var failed = false
func check(ok, message):
	if not ok:
		failed = true
		printerr("FAIL: ", message)
func _init(): call_deferred("begin")
func mouse(position, pressed):
	var e = InputEventMouseButton.new()
	e.button_index = BUTTON_LEFT
	e.position = position
	e.global_position = position
	e.pressed = pressed
	get_root().input(e)
	if not get_root().is_input_handled():
		get_root().unhandled_input(e)
func begin():
	player = load("res://Main.tscn").instance()
	get_root().add_child(player)
	player.set_process(false)
	player.runtime = recorder
	player.ready = true
	player.close_modal()
	yield(self, "idle_frame")
	# Tap must use its own position, even without a preceding hover/motion.
	var point = player.game_origin + Vector2(137, 219)
	mouse(point, true)
	mouse(point, false)
	check(recorder.commands.size() == 2, "tap sends press and release")
	if recorder.commands.size() == 2:
		check(recorder.commands[0].x == 137 and recorder.commands[0].y == 219, "tap coordinates follow game origin")
		check(recorder.commands[0].kind == "press" and recorder.commands[1].kind == "release", "tap order")
	recorder.commands.clear()
	# Side controls must not click the original picture.
	mouse(Vector2(5, 5), true)
	check(recorder.commands.empty(), "outside picture does not click")
	# Verify drag continuation and release coordinates.
	mouse(point, true)
	var move = InputEventMouseMotion.new()
	move.position = point + Vector2(20, 15)
	move.relative = Vector2(20, 15)
	move.button_mask = BUTTON_MASK_LEFT
	get_root().input(move)
	if not get_root().is_input_handled():
		get_root().unhandled_input(move)
	mouse(move.position, false)
	check(recorder.commands.size() == 3, "drag sends three ordered events")
	if recorder.commands.size() == 3:
		check(recorder.commands[1].kind == "move" and recorder.commands[2].x == 157, "drag preserves new pointer position")
	# Touch navigation repeats while held, without requiring any keyboard.
	recorder.commands.clear()
	Input.action_press("touch_up")
	player.process_touch(0.21)
	Input.action_release("touch_up")
	check(recorder.commands.size() == 1 and recorder.commands[0].key == "uparrow", "onscreen movement")
	player.process_touch(0.01)
	recorder.commands.clear()
	Input.action_press("touch_up")
	player.process_touch(0.01)
	player.process_touch(0.54)
	check(recorder.commands.size() == 1, "short joystick hold moves only once")
	player.process_touch(0.02)
	check(recorder.commands.size() == 2, "held joystick repeats after initial delay")
	player.process_touch(0.39)
	check(recorder.commands.size() == 2, "repeat waits between steps")
	player.process_touch(0.02)
	check(recorder.commands.size() == 3, "held repeat is limited")
	Input.action_release("touch_up")
	player.process_touch(1.0)
	check(recorder.commands.size() == 3, "release cancels repeats")
	player.show_keyboard(null)
	player.keyboard_key("a")
	check(recorder.commands.back().key == "a", "onscreen typing")
	player.close_keyboard()
	var joystick = player.touch_strip.get_node("joystick")
	joystick.move_stick(Vector2.RIGHT * joystick.radius * 0.35)
	check(joystick.direction.empty(), "small thumb displacement does not move")
	joystick.move_stick(Vector2.RIGHT * joystick.radius * 0.5)
	check(joystick.direction == "right", "deliberate displacement engages")
	joystick.move_stick(Vector2(0.4, 0.45) * joystick.radius)
	check(joystick.direction == "right", "diagonal jitter keeps selected axis")
	joystick.move_stick(Vector2(0.2, 0.65) * joystick.radius)
	check(joystick.direction == "down", "deliberate axis change works")
	joystick.move_stick(Vector2.DOWN * joystick.radius * 0.30)
	check(joystick.direction == "down", "engaged stick tolerates dead zone edge jitter")
	joystick.move_stick(Vector2.DOWN * joystick.radius * 0.25)
	check(joystick.direction.empty(), "returning toward center releases")
	joystick.reset()
	var touch = InputEventScreenTouch.new()
	touch.index = 2
	touch.position = joystick.position
	touch.pressed = true
	joystick._input(touch)
	check(joystick.direction.empty(), "joystick center is a dead zone")
	var drag = InputEventScreenDrag.new()
	drag.index = 2
	for pair in [[Vector2.UP, "up"], [Vector2.RIGHT, "right"], [Vector2.DOWN, "down"], [Vector2.LEFT, "left"]]:
		drag.position = joystick.position + pair[0] * joystick.radius * 2
		joystick._input(drag)
		check(joystick.direction == pair[1], "joystick maps " + pair[1])
		check(Input.is_action_pressed("touch_" + pair[1]), "joystick presses direction")
		check(joystick.offset.length() <= joystick.radius, "joystick knob stays inside base")
	touch.pressed = false
	touch.index = 3
	joystick._input(touch)
	check(joystick.direction == "left", "second finger cannot release joystick")
	touch.index = 2
	joystick._input(touch)
	check(not Input.is_action_pressed("touch_left"), "releasing joystick outside base stops movement")
	recorder.commands.clear()
	touch.pressed = true
	joystick._input(touch)
	touch.pressed = false
	joystick._input(touch)
	check(recorder.commands.size() == 1 and recorder.commands[0].key == " ", "joystick tap activates door")
	recorder.commands.clear()
	touch.pressed = true
	joystick._input(touch)
	drag.position = joystick.position + Vector2.RIGHT * joystick.radius
	joystick._input(drag)
	drag.position = joystick.position
	joystick._input(drag)
	touch.pressed = false
	joystick._input(touch)
	check(recorder.commands.size() == 1 and recorder.commands[0].key == "rightarrow", "drag sends one direction without triggering door on release")
	for pair in [[Vector2.UP, "uparrow"], [Vector2.RIGHT, "rightarrow"], [Vector2.DOWN, "downarrow"], [Vector2.LEFT, "leftarrow"]]:
		recorder.commands.clear()
		touch.position = joystick.position + pair[0] * joystick.radius * 0.8
		touch.pressed = true
		joystick._input(touch)
		touch.pressed = false
		joystick._input(touch)
		check(recorder.commands.size() == 1 and recorder.commands[0].key == pair[1], "quick edge tap moves exactly once: " + pair[1])
	touch.position = joystick.position
	touch.pressed = true
	joystick._input(touch)
	joystick.move_stick(Vector2.UP * joystick.radius)
	player.show_menu()
	check(not Input.is_action_pressed("touch_up") and joystick.finger == -1, "opening menu releases joystick")
	print("CONTROLS ", "FAIL" if failed else "PASS")
	quit(1 if failed else 0)
