extends SceneTree
var failed = false
func check(ok, label):
	if not ok:
		failed = true
		printerr("FAIL: ", label)
func _init():
	call_deferred("begin")
func begin():
	var player = load("res://Main.tscn").instance()
	get_root().add_child(player)
	yield(create_timer(0.3), "timeout")
	player.close_modal()
	check(not player.window_close_requested(0), "first close opens menu")
	var menu = player.menu
	check(menu != null and player.modal == menu and player.manual_pause, "close shows engine menu")
	check(not player.window_close_requested(50), "double-close does not exit")
	check(player.modal == menu and player.manual_pause, "second close does not toggle menu off")
	player.window_close_requested(100)
	check(not player.window_close_requested(200), "rapid clicks alone do not force exit")
	check(player.window_close_requested(2600), "persistent close requests force exit")
	check(not player.window_close_requested(9000), "old requests expire")
	player.resume_game()
	check(player.modal == null and not player.manual_pause, "resume unpauses after window close")
	player.show_setup()
	var setup = player.modal
	player.window_close_requested(12000)
	check(player.modal == player.menu and not setup.visible, "close also opens menu over a dialog")
	player.window_close_requested(12050)
	player.resume_game()
	check(player.modal == setup and setup.visible, "resume restores interrupted dialog")
	print("WINDOW CLOSE TEST ", "FAIL" if failed else "PASS")
	quit(1 if failed else 0)
