extends SceneTree
var player
var ticks = 0
var phase = 0
var failure = false
var saved_state
var nav_view = ""

func _init():
	call_deferred("begin")

func begin():
	player = load("res://Main.tscn").instance()
	get_root().add_child(player)
	player.set_process(false)
	if player.runtime == null:
		print("FAIL: runtime missing")
		quit(1)
		return
	step()

func state():
	return JSON.parse(player.runtime.query("titanicState()")).result

func js(code):
	var error = player.runtime.execute(code)
	if not error.empty():
		failure = true
		print("FAIL JS: ", error)

func step():
	for _i in range(10):
		ticks += 1
		player._process(0.05)
		if ticks % 20 == 0:
			var s = state()
			if phase == 0:
				if s.get("movie", "") == "playmode.mov":
					print("MAIN MENU: ", JSON.print(s.regions))
					player.send({"action": "pointer", "kind": "press", "x": 266, "y": 254})
					player.send({"action": "pointer", "kind": "release", "x": 266, "y": 254})
					phase = 1
				else:
					player.send({"action": "key", "key": ".", "special": true})
			elif phase == 1:
				if s.get("movie") != null:
					player.send({"action": "key", "key": ".", "special": true})
				else:
					print("EXPLORING: ", JSON.print(s))
					js("titanicTesting.host.session.interp.globals.set('retanic_roundtrip', 'saved correctly'); globalThis.testSnapshot=titanicTesting.host.session.snapshotSave();")
					var bytes = player.runtime.buffer("testSnapshot.buffer")
					if bytes.size() < 1536 or not player.saves.write("retanic-integration", bytes):
						failure = true
					saved_state = s
					nav_view = s.view
					player.send({"action": "key", "key": "rightarrow"})
					phase = 2
			elif phase == 2:
				if s.view == nav_view:
					player.send({"action": "key", "key": "rightarrow"})
				else:
					print("NAVIGATED: ", s.view)
					player.send({"action": "pause", "on": true})
					phase = 3
			elif phase == 3:
				if not s.paused:
					failure = true
				player.send({"action": "pause", "on": false})
				player.start_runtime(player.saves.path_for("retanic-integration"))
				phase = 4
			elif phase == 4:
				if s.ready and s.view == saved_state.view:
					var value = player.runtime.query("titanicTesting.host.session.interp.globals.get('retanic_roundtrip')")
					if value != "saved correctly":
						failure = true
					print("RESTORED: ", JSON.print(s), " sentinel=", value)
					player.frame_image.save_png("user://integration.png")
					js("globalThis.titanicTick = () => { throw Error('expected tick failure test'); };")
					player._process(0.05)
					if not player.runtime_failed or player.modal == null:
						failure = true
					player.resume_game()
					player.send({"action": "load", "path": player.saves.path_for("retanic-integration")})
					phase = 5
			elif phase == 5:
				if s.ready and not player.runtime_failed and s.view == saved_state.view:
					if player.runtime.query("titanicTesting.host.session.interp.globals.get('retanic_roundtrip')") != "saved correctly":
						failure = true
					print("RESTORED AFTER TICK FAILURE")
					print("INTEGRATION ", "FAIL" if failure else "PASS")
					quit(1 if failure else 0)
					return
		if ticks > 2500 or failure:
			print("FAIL timeout/state: ", JSON.print(state()))
			quit(1)
			return
	call_deferred("step")
