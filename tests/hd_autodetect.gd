extends SceneTree
var failed = false

func check(ok, label):
	if not ok:
		failed = true
		printerr("FAIL: ", label)

func _init():
	var player = load("res://scripts/player.gd").new()
	var expected = OS.get_environment("RETANIC_TEST_HD_PATH")
	check(not expected.empty(), "test supplies an expected pack path")
	player.hd_pack_path = player.find_hd_pack()
	check(player.hd_pack_path == expected, "find pack beside player or package")
	check(player.active_hd_pack() == expected, "enable detected pack with fresh settings")
	player.config.set_value("graphics", "hd_enabled", false)
	check(player.active_hd_pack().empty(), "explicit off overrides detected pack")
	check(player.config.save("user://hd-autodetect-test.cfg") == OK, "save preference")
	player.config = ConfigFile.new()
	check(player.config.load("user://hd-autodetect-test.cfg") == OK, "reload preference")
	check(player.active_hd_pack().empty(), "off persists after restarting")
	player.config.set_value("graphics", "hd_enabled", true)
	check(player.active_hd_pack() == expected, "settings can enable pack again")
	player.free()
	print("HD AUTODETECT ", "FAIL" if failed else "PASS")
	quit(1 if failed else 0)
