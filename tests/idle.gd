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
	print("IDLE ", "FAIL" if failed else "PASS", " draws=", idle_draws)
	quit(1 if failed else 0)
