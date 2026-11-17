extends SceneTree
var failed = false
func check(ok, message):
	if not ok:
		failed = true
		printerr("FAIL: ", message)
func _init(): call_deferred("begin")
func begin():
	var player = load("res://Main.tscn").instance()
	get_root().add_child(player)
	player.runtime = null
	player.close_modal()
	player.status.hide()
	yield(create_timer(0.4), "timeout")
	var before = Engine.get_frames_drawn()
	yield(create_timer(1.0), "timeout")
	var idle_draws = Engine.get_frames_drawn() - before
	check(idle_draws <= 2, "unchanged screen keeps redrawing: " + str(idle_draws))
	before = Engine.get_frames_drawn()
	player.pointer += Vector2(10, 10)
	yield(create_timer(0.2), "timeout")
	check(Engine.get_frames_drawn() > before, "pointer movement did not redraw")
	before = Engine.get_frames_drawn()
	player.pointer_name = "touch"
	yield(create_timer(0.2), "timeout")
	check(Engine.get_frames_drawn() > before, "cursor style did not redraw")
	# The pointer must visibly change over things that respond, as it does in
	# the original; unknown names keep the plain arrow.
	var arrow = player.cursor_shape("arrow")
	check(arrow.size() >= 3 and player.cursor_shape("") == arrow and player.cursor_shape("unknown") == arrow, "plain cursor is not the arrow")
	var seen = [arrow]
	for name in ["touch", "hand", "fist", "watch", "sight", "goup", "godown", "goleft", "goright"]:
		var shape = player.cursor_shape(name)
		check(shape.size() >= 3 and not shape in seen, "cursor " + name + " has no shape of its own")
		seen.append(shape)
	check(player.cursor_shape("gostrait") == player.cursor_shape("goup"), "straight ahead does not point up")
	print("IDLE ", "FAIL" if failed else "PASS", " draws=", idle_draws)
	quit(1 if failed else 0)
