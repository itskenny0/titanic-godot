extends SceneTree
class Recorder:
	extends Reference
	var commands = []
	func execute(code):
		if code.begins_with("titanicCommand("):
			commands.append(JSON.parse(code.substr(15, code.length()-16)).result)
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
	print("CONTROLLER ", "FAIL" if failed else "PASS")
	quit(1 if failed else 0)
