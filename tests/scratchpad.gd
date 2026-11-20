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
	player.scratchpad.finish_animations()
	var before = scrap.get_position()
	var start = scrap.get_global_rect().position + Vector2(30, 24)
	screen_press(start, true)
	for i in range(1, 11):
		screen_move(start + Vector2(16 * i, 9 * i))
		yield(self, "idle_frame")
	screen_press(start + Vector2(160, 90), false)
	yield(self, "idle_frame")
	var note = player.scratchpad.notes.frank_cabin
	check(note.pinned, "dragging a new clue pins it")
	check(Vector2(note.x, note.y).distance_to(before + Vector2(160, 90)) < 2, "the clue stays where it was dropped")
	check(pointer_presses(recorder) == 0, "touching a note never clicks the game behind it")
	check(player.scratchpad.pinned.has("frank_cabin") and player.scratchpad.popups.empty(), "the pinned note replaces the popup")

	# Move the mouse off the note: hovering makes it opaque to read it.
	screen_mouse(Vector2(4, player.layout_size.y - 4))
	yield(self, "idle_frame")
	var paper = player.scratchpad.pinned.frank_cabin
	check(paper.modulate.a < 1.0 and paper.modulate.a > 0.7, "notes are slightly see-through paper")
	check(abs(paper.get_rotation_degrees()) <= 2.7, "notes sit at a slight angle, like scraps")
	check(player.scratchpad.tray.visible, "the notes tray appears once there are notes")
	var tray_box = player.scratchpad.tray.get_global_rect()
	check(tray_box.position.x >= player.note_area().position.x and tray_box.end.x <= player.note_area().end.x, "the tray sits on the picture, clear of the side panels")

	# A popup's pin keeps it on screen; a pinned note's pin puts it away.
	player.handle_event({"type": "clue", "id": "penny_cabin", "text": "Penny Pringle: cabin F-34."})
	yield(self, "idle_frame")
	var popup = player.scratchpad.popups[0]
	check(popup.find_node("Heading", true, false).text.find("pin") >= 0, "a new clue says how to keep it")
	check(popup.find_node("Pin", true, false) != null, "a new clue has a pin")
	player.scratchpad.press_pin(popup)
	yield(self, "idle_frame")
	check(player.scratchpad.notes.penny_cabin.pinned and player.scratchpad.pinned.has("penny_cabin"), "the pin keeps a clue on screen")
	player.scratchpad.press_pin(player.scratchpad.pinned.penny_cabin)
	player.scratchpad.finish_animations()
	yield(self, "idle_frame")
	check(not player.scratchpad.notes.penny_cabin.pinned and not player.scratchpad.pinned.has("penny_cabin"), "a pinned note can be put away")
	player.handle_event({"type": "clue", "id": "smokestack_man", "text": "Willie was seen on the last smokestack."})
	check(player.scratchpad.popups.size() == 1, "another clue pops up")
	player.scratchpad.expire_popup(player.scratchpad.popups[0])
	player.scratchpad.finish_animations()
	yield(self, "idle_frame")
	check(player.scratchpad.popups.empty() and player.scratchpad.notes.has("smokestack_man"), "an unpinned clue flies into the tray and stays in the scratchpad")
	check(player.scratchpad.unseen == 1, "the tray counts clues not looked at yet")

	# The tray gathers pinned notes into itself and lays them out again.
	var home = player.scratchpad.pinned.frank_cabin.get_position()
	player.scratchpad.activate_tray()
	player.scratchpad.finish_animations()
	check(player.scratchpad.collapsed and not player.scratchpad.pinned.frank_cabin.visible, "the tray gathers the notes")
	player.scratchpad.activate_tray()
	player.scratchpad.finish_animations()
	check(not player.scratchpad.collapsed and player.scratchpad.pinned.frank_cabin.visible and player.scratchpad.pinned.frank_cabin.get_position().distance_to(home) < 0.5, "the notes come back where they were")
	player.scratchpad.set_collapsed(true)
	player.scratchpad.finish_animations()
	var settings = ConfigFile.new()
	settings.load(path)
	check(settings.get_value("scratchpad", "collapsed", false), "a gathered tray stays gathered")
	player.scratchpad.set_collapsed(false)
	player.scratchpad.finish_animations()

	# A note can be pushed partly off the screen, and fetched back.
	var paper_note = player.scratchpad.pinned.frank_cabin
	start = paper_note.get_global_rect().position + Vector2(20, 24)
	var edge = Vector2(player.layout_size.x + 200, start.y + 400)
	screen_press(start, true)
	for i in range(1, 11):
		screen_move(start + (edge - start) * (i / 10.0))
		yield(self, "idle_frame")
	screen_press(edge, false)
	yield(self, "idle_frame")
	var hanging = Rect2(paper_note.get_position(), paper_note.get_size())
	var screen = Rect2(Vector2.ZERO, player.layout_size)
	check(hanging.end.x > screen.end.x and hanging.end.y > screen.end.y, "a note can hang off the screen's edge")
	var top_left = Vector2(max(screen.position.x, hanging.position.x), max(screen.position.y, hanging.position.y))
	var bottom_right = Vector2(min(screen.end.x, hanging.end.x), min(screen.end.y, hanging.end.y))
	var showing = Rect2(top_left, bottom_right - top_left)
	check(showing.size.x >= 39 and showing.size.y >= 27, "enough of it stays on screen to pick it up")
	# Pick it up by its outer corner, clear of the notes button.
	start = showing.end - Vector2(8, 6)
	screen_press(start, true)
	for i in range(1, 11):
		screen_move(start + (Vector2(200, 160) - start) * (i / 10.0))
		yield(self, "idle_frame")
	screen_press(Vector2(200, 160), false)
	yield(self, "idle_frame")
	check(screen.encloses(Rect2(paper_note.get_position(), paper_note.get_size())), "the note can be fetched back")
	player.scratchpad.set_pinned("frank_cabin", true, home)

	# Dropping a note on the tray puts it away.
	var lifted = player.scratchpad.pinned.frank_cabin
	start = lifted.get_global_rect().position + Vector2(30, 24)
	var target = player.scratchpad.tray_centre()
	screen_press(start, true)
	for i in range(1, 11):
		screen_move(start + (target - start) * (i / 10.0))
		yield(self, "idle_frame")
	screen_press(target, false)
	player.scratchpad.finish_animations()
	yield(self, "idle_frame")
	check(not player.scratchpad.notes.frank_cabin.pinned, "dropping a note on the tray puts it away")
	player.scratchpad.set_pinned("frank_cabin", true, home)

	# With nothing pinned, the tray opens the scratchpad, and Back returns to the game.
	for id in player.scratchpad.order:
		player.scratchpad.notes[id].pinned = false
	player.scratchpad.refresh()
	player.scratchpad.activate_tray()
	yield(self, "idle_frame")
	check(player.modal != null and player.scratchpad.unseen == 0, "the tray opens the scratchpad and clears its count")
	button_named(player.modal, "Back").emit_signal("pressed")
	yield(self, "idle_frame")
	check(player.modal == null and player.menu == null and not player.manual_pause, "Back from the tray returns to the game")
	player.scratchpad.set_pinned("frank_cabin", true, home)

	# Clues belong to the save they were heard in.
	var save_path = "user://scratchpad-test-voyage.ti"
	check(player.scratchpad.save_voyage(save_path), "clues are written beside a save")
	player.scratchpad.write("Remember the lifeboats")
	player.scratchpad.begin_voyage("")
	var only_own = true
	for id in player.scratchpad.notes:
		only_own = only_own and player.scratchpad.notes[id].own
	check(only_own and player.scratchpad.notes.size() == 1, "a new game starts without clues but keeps your own notes")
	player.scratchpad.begin_voyage(save_path)
	check(player.scratchpad.notes.has("frank_cabin") and player.scratchpad.notes.frank_cabin.pinned and player.scratchpad.pinned.frank_cabin.get_position().distance_to(home) < 1, "loading the save brings its clues back, pinned where they were")
	check(player.scratchpad.notes.has("smokestack_man") and not player.scratchpad.notes.smokestack_man.pinned, "unpinned clues come back into the tray")
	var reloaded = load("res://scripts/scratchpad.gd").new()
	reloaded.path = path
	reloaded.load_store()
	var stored_clue = false
	for id in reloaded.notes:
		stored_clue = stored_clue or not reloaded.notes[id].own
	check(not stored_clue and reloaded.notes.size() == 1, "only your own notes are kept outside the saves")
	reloaded.free()
	for id in player.scratchpad.order.duplicate():
		if player.scratchpad.notes[id].own:
			player.scratchpad.remove(id)
	# Every save the engine writes carries the voyage's clues.
	player.bridge_call("write", JSON.print({"path": "save:Scratchpad Test"}), PoolByteArray([1, 2, 3]))
	var written = player.saves.last_written
	var stored = ConfigFile.new()
	check(stored.load(written + ".notes") == OK and stored.get_value("voyage", "order", []).has("frank_cabin"), "a game save stores its clues beside it")
	check(player.save_file_path("save:Scratchpad Test") == written, "loading a save finds its clues")
	for leftover in [written, written + ".notes", written + ".png", save_path + ".notes"]:
		Directory.new().remove(leftover)

	# A controller can reach the tray.
	recorder.surface = {"context": "room", "key": "room", "targets": [{"id": "prop:door", "label": "door", "x": 100, "y": 100, "w": 40, "h": 40, "aim_x": 120, "aim_y": 120}]}
	player.refresh_controller_surface(true)
	var ids = []
	for t in player.controller_surface.targets:
		ids.append(t.id)
	check(ids.has("ui:notes"), "controllers can select the notes tray")
	recorder.surface = {"context": "room", "key": "room", "targets": []}

	# The scratchpad menu lists everything and takes the player's own notes.
	player.show_menu()
	button_named(player.modal, "Scratchpad").emit_signal("pressed")
	yield(self, "idle_frame")
	var listed = player.modal.find_node("Notes", true, false)
	check(listed != null and listed.get_child_count() == 3, "the scratchpad lists every clue")
	check(button_named(player.modal, "Gather pinned notes") != null, "the list can gather the notes, for keyboards and controllers")
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
	button_named(player.modal, "Scratchpad: ").emit_signal("pressed")
	yield(self, "idle_frame")
	check(player.modal == player.scratchpad_options_dialog, "Settings open the scratchpad options")
	player.modal.find_node("mode_off", true, false).emit_signal("pressed")
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
	player.set_scratchpad_mode("clues")
	check(player.scratchpad.pinned.has("frank_cabin"), "turning it back on restores pinned notes")

	# My notes only: game clues are kept but hidden, and come back.
	var mine = player.scratchpad.write("Remember the trunk")
	player.show_scratchpad_options()
	player.modal.find_node("mode_own", true, false).emit_signal("pressed")
	yield(self, "idle_frame")
	check(player.scratchpad_mode() == "own" and player.scratchpad_enabled(), "the scratchpad can hold only my notes")
	check(recorder.commands.back().action == "clues" and recorder.commands.back().on, "the game keeps telling the scratchpad what was said")
	check(player.scratchpad.pinned.has(mine) and not player.scratchpad.pinned.has("frank_cabin"), "my notes stay, game clues hide")
	check(not player.scratchpad.listed().has("frank_cabin") and player.scratchpad.listed().has(mine), "the list shows only my notes")
	check(player.modal.find_node("kind_places", true, false).disabled, "clue kinds only apply with game clues")
	player.handle_event({"type": "clue", "id": "burns_cabin", "kind": "places", "text": "The Burnses: cabin C-78."})
	check(player.scratchpad.popups.empty() and player.scratchpad.notes.has("burns_cabin"), "a clue heard meanwhile is kept, not shown")
	player.modal.find_node("mode_clues", true, false).emit_signal("pressed")
	yield(self, "idle_frame")
	check(player.scratchpad.pinned.has("frank_cabin") and player.scratchpad.listed().has("burns_cabin"), "switching back shows every clue heard")

	# Clue kinds: a kind switched off hides its clues, also pinned ones.
	player.scratchpad.notes.frank_cabin["kind"] = "places"
	player.modal.find_node("kind_places", true, false).emit_signal("pressed")
	yield(self, "idle_frame")
	check(not "places" in player.clue_kinds() and not player.scratchpad.pinned.has("frank_cabin"), "turning off a kind hides its pinned clues")
	player.handle_event({"type": "clue", "id": "cafe_where", "kind": "places", "text": "Café Parisien: aft."})
	check(player.scratchpad.popups.empty(), "no popup for a kind that is off")
	player.handle_event({"type": "clue", "id": "pipe_humid", "kind": "puzzles", "text": "Trask: steam up the bath."})
	check(player.scratchpad.popups.empty(), "puzzle hints are off by default")
	player.modal.find_node("kind_places", true, false).emit_signal("pressed")
	yield(self, "idle_frame")
	check(player.scratchpad.pinned.has("frank_cabin") and player.scratchpad.listed().has("cafe_where"), "turning the kind back on brings its clues back")
	check(Rect2(Vector2.ZERO, player.layout_size).encloses(player.modal.get_global_rect()), "the scratchpad options fit the screen")
	player.controller_back()
	check(button_named(player.modal, "Scratchpad: ") != null, "Back returns to Settings")
	player.close_modal()
	ConfigFile.new().save(path)
	# The settings toggles saved settings.cfg; leave it as a fresh install.
	player.config.clear()
	player.config.save("user://settings.cfg")
	print("SCRATCHPAD ", "FAIL" if failed else "PASS")
	quit(1 if failed else 0)
