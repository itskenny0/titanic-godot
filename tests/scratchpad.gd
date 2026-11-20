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
const ICON = "iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAYAAAAf8/9hAAAAGklEQVR4nGM4scDmPyWYYdSAUQNGDRguBgAAS+WjH2cHwdcAAAAASUVORK5CYII="
var failed = false
var player
var mouse_held = false
func check(ok, message):
	if not ok:
		failed = true
		printerr("FAIL: ", message)
func _init(): call_deferred("begin")

func screen_mouse(point, pressed = null):
	var event = InputEventMouseMotion.new() if pressed == null else InputEventMouseButton.new()
	event.position = point * Vector2(OS.window_size) / player.layout_size
	event.global_position = event.position
	if pressed != null:
		event.button_index = 1
		event.pressed = pressed
		mouse_held = pressed
	elif mouse_held:
		event.button_mask = 1
	Input.parse_input_event(event)
	Input.flush_buffered_events()

# Touchscreens drag notes with a finger, through Godot's mouse emulation.
func screen_press(point, pressed):
	if not "--touch-test" in OS.get_cmdline_args():
		screen_mouse(point, pressed)
		return
	var touch = InputEventScreenTouch.new()
	touch.index = 0
	touch.position = point * Vector2(OS.window_size) / player.layout_size
	touch.pressed = pressed
	Input.parse_input_event(touch)
	Input.flush_buffered_events()

func screen_move(point):
	if not "--touch-test" in OS.get_cmdline_args():
		screen_mouse(point)
		return
	var drag = InputEventScreenDrag.new()
	drag.index = 0
	drag.position = point * Vector2(OS.window_size) / player.layout_size
	Input.parse_input_event(drag)
	Input.flush_buffered_events()

func find_buttons(node, found):
	if node is Button:
		found.append(node)
	for child in node.get_children():
		find_buttons(child, found)
	return found

func button_named(root, prefix):
	for b in find_buttons(root, []):
		if b.text.begins_with(prefix):
			return b
	return null

func scraps():
	var out = []
	for child in player.scratchpad.get_children():
		if child is PanelContainer and not child.is_queued_for_deletion():
			out.append(child)
	return out

func pointer_presses(recorder):
	var count = 0
	for command in recorder.commands:
		if command.get("action", "") == "pointer" and command.get("kind", "") == "press":
			count += 1
	return count

func begin():
	player = load("res://Main.tscn").instance()
	get_root().add_child(player)
	player.set_process(false)
	player.close_modal()
	var recorder = Recorder.new()
	player.runtime = recorder
	player.ready = true
	player.has_frame = true
	player.config.clear()
	var path = "user://scratchpad-test.cfg"
	var fresh = ConfigFile.new()
	fresh.save(path)
	player.scratchpad.path = path
	player.scratchpad.load_store()
	player.scratchpad.refresh()
	check(player.scratchpad_enabled() and player.item_notices_enabled(), "scratchpad and item notices are on by default")
	check(scraps().empty(), "a new scratchpad is empty")

	# A clue pops up once, as a scrap of paper.
	player.handle_event({"type": "clue", "id": "frank_cabin", "text": "My cabin: C-73."})
	player.handle_event({"type": "clue", "id": "frank_cabin", "text": "My cabin: C-73."})
	yield(self, "idle_frame")
	check(scraps().size() == 1 and player.scratchpad.popups.size() == 1, "a clue pops up once")
	var scrap = scraps()[0]
	check(scrap.get_global_rect().position.x >= 0 and scrap.get_global_rect().end.y <= player.layout_size.y, "the scrap fits on screen")
	check(player.scratchpad.notes.has("frank_cabin") and not player.scratchpad.notes.frank_cabin.pinned, "the clue is kept in the scratchpad")

	# Dragging moves the scrap, pins it where it is dropped, and never clicks the game.
	var start = scrap.get_global_rect().position + Vector2(30, 24)
	screen_press(start, true)
	for i in range(1, 11):
		screen_move(start + Vector2(16 * i, 9 * i))
		yield(self, "idle_frame")
	screen_press(start + Vector2(160, 90), false)
	yield(self, "idle_frame")
	var note = player.scratchpad.notes.frank_cabin
	check(note.pinned, "dragging a new clue pins it")
	check(abs(note.x - (start.x - 30 + 160)) < 2 and abs(note.y - (start.y - 24 + 90)) < 2, "the clue stays where it was dropped")
	check(pointer_presses(recorder) == 0, "touching a note never clicks the game behind it")
	check(player.scratchpad.pinned.has("frank_cabin") and player.scratchpad.popups.empty(), "the pinned note replaces the popup")

	# Pinned notes survive a restart, at their place.
	var reloaded = load("res://scripts/scratchpad.gd").new()
	reloaded.path = path
	reloaded.load_store()
	check(reloaded.notes.has("frank_cabin") and reloaded.notes.frank_cabin.pinned and abs(reloaded.notes.frank_cabin.x - note.x) < 0.5, "pinned notes are saved")
	reloaded.free()

	# A popup's Pin button keeps it on screen; unpinned popups fade away.
	player.handle_event({"type": "clue", "id": "penny_cabin", "text": "Penny Pringle: cabin F-34."})
	yield(self, "idle_frame")
	var popup = player.scratchpad.popups[0]
	popup.find_node("Pin", true, false).emit_signal("pressed")
	yield(self, "idle_frame")
	check(player.scratchpad.notes.penny_cabin.pinned and player.scratchpad.pinned.has("penny_cabin"), "Pin keeps a clue on screen")
	player.scratchpad.pinned.penny_cabin.find_node("Unpin", true, false).emit_signal("pressed")
	yield(self, "idle_frame")
	check(not player.scratchpad.notes.penny_cabin.pinned and not player.scratchpad.pinned.has("penny_cabin"), "a pinned note can be taken off the screen")
	player.handle_event({"type": "clue", "id": "smokestack_man", "text": "Willie was seen on the last smokestack."})
	check(player.scratchpad.popups.size() == 1, "another clue pops up")
	player.scratchpad.expire_popup(player.scratchpad.popups[0])
	yield(self, "idle_frame")
	check(player.scratchpad.popups.empty() and player.scratchpad.notes.has("smokestack_man"), "an unpinned clue leaves the screen but stays in the scratchpad")

	# The scratchpad menu lists everything and takes the player's own notes.
	player.show_menu()
	button_named(player.modal, "Scratchpad").emit_signal("pressed")
	yield(self, "idle_frame")
	var listed = player.modal.find_node("Notes", true, false)
	check(listed != null and listed.get_child_count() == 3, "the scratchpad lists every clue")
	check(player.modal.get_global_rect().end.y <= player.layout_size.y + 1, "the scratchpad fits the screen")
	button_named(player.modal, "Write a note").emit_signal("pressed")
	yield(self, "idle_frame")
	check(player.get_focus_owner() == player.note_editor, "writing a note focuses the text field")
	check(button_named(player.modal, "On-screen keyboard") != null, "notes can be typed without a keyboard")
	player.note_editor.text = "  Ask the Purser about the cargo hold  "
	button_named(player.modal, "Pin note").emit_signal("pressed")
	yield(self, "idle_frame")
	var own = ""
	for id in player.scratchpad.notes:
		if player.scratchpad.notes[id].own:
			own = id
	check(not own.empty() and player.scratchpad.notes[own].text == "Ask the Purser about the cargo hold", "the player's note is kept")
	check(player.scratchpad.pinned.has(own), "a new note is pinned on screen")
	listed = player.modal.find_node("Notes", true, false)
	check(listed.get_child(0).get_meta("note") == own, "the newest note is listed first")
	listed.get_child(0).get_node("Delete").emit_signal("pressed")
	yield(self, "idle_frame")
	check(not player.scratchpad.notes.has(own) and not player.scratchpad.pinned.has(own), "a note can be deleted")
	player.close_modal()
	player.menu = null

	# Received items: five seconds, with picture and name, at the top.
	player.handle_event({"type": "item", "id": "ring", "name": "Willie's Ring", "bag": true, "icon": ICON})
	yield(self, "idle_frame")
	var notice = player.item_notice
	check(is_instance_valid(notice) and notice.find_node("Title", true, false).text == "Item received: Willie's Ring", "the notice names the item")
	var icon = notice.find_node("Icon", true, false)
	check(icon != null and icon.texture != null and icon.texture.get_width() >= 32, "the notice shows the item's picture")
	var bounds = notice.get_global_rect()
	check(bounds.position.y < 60 and abs(bounds.get_center().x - player.layout_size.x / 2) < 2, "the notice sits at the top centre")
	player.handle_event({"type": "item", "id": "cufflink", "name": "Cufflink", "bag": true, "icon": ""})
	check(player.item_notice == notice and player.item_queue.size() == 1, "a second item waits its turn")
	yield(create_timer(5.3), "timeout")
	check(not is_instance_valid(notice) and is_instance_valid(player.item_notice), "the notice leaves after five seconds and the next one shows")
	check(player.item_notice.find_node("Icon", true, false) == null, "a missing picture leaves just the name")

	# Both features can be turned off, also in the running game.
	player.show_settings()
	button_named(player.modal, "New items").emit_signal("pressed")
	check(not player.item_notices_enabled() and recorder.commands.back().action == "item_notices" and not recorder.commands.back().on, "item notices can be turned off")
	check(not is_instance_valid(player.item_notice) and player.item_queue.empty(), "turning them off clears notices")
	player.handle_event({"type": "item", "id": "ring", "name": "Willie's Ring", "bag": true, "icon": ICON})
	check(player.item_notice == null, "no notice while off")
	button_named(player.modal, "Scratchpad clues").emit_signal("pressed")
	yield(self, "idle_frame")
	check(not player.scratchpad_enabled() and recorder.commands.back().action == "clues" and not recorder.commands.back().on, "the scratchpad can be turned off")
	check(scraps().empty(), "no notes on screen while off")
	player.handle_event({"type": "clue", "id": "trask_cabin", "text": "Mr. Trask: cabin D-33."})
	check(scraps().empty(), "no popups while off")
	player.close_modal()
	player.show_menu()
	check(button_named(player.modal, "Scratchpad") == null, "the menu hides the scratchpad while off")
	player.close_modal()
	player.menu = null
	player.config.set_value("scratchpad", "enabled", true)
	player.scratchpad.refresh()
	check(player.scratchpad.pinned.has("frank_cabin"), "turning it back on restores pinned notes")
	ConfigFile.new().save(path)
	# The settings toggles saved settings.cfg; leave it as a fresh install.
	player.config.clear()
	player.config.save("user://settings.cfg")
	print("SCRATCHPAD ", "FAIL" if failed else "PASS")
	quit(1 if failed else 0)
