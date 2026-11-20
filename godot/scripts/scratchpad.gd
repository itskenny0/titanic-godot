extends Control
# The player's scratchpad: clue scraps that pop up when a conversation
# mentions a detail worth keeping, such as a cabin number, and notes the
# player writes. A scrap shows briefly; pinned ones stay where they are
# dragged. Notes live in user://scratchpad.cfg, apart from the saves.
const PAPER = Color("ebdcb5")
const PAPER_EDGE = Color("8c7448")
const PEN = Color("2a2216")
const HEADING = Color("7b5f33")
const WIDTH = 168.0
const POPUP_SECONDS = 7.0

var path = "user://scratchpad.cfg"
var notes = {}
var order = []
# The player: fonts, the logical screen size and whether the feature is on.
var host = null
var popups = []
var pinned = {}
var dragging = null
var drag_offset = Vector2.ZERO
var drag_moved = false
var serial = 0

func _ready():
	mouse_filter = MOUSE_FILTER_IGNORE

func load_store():
	notes = {}
	order = []
	var file = ConfigFile.new()
	if file.load(path) != OK:
		return
	for id in file.get_value("scratchpad", "order", []):
		var note = file.get_value("notes", id, null)
		if note is Dictionary and note.has("text"):
			notes[id] = note
			order.append(id)

func save_store():
	var file = ConfigFile.new()
	file.set_value("scratchpad", "order", order)
	for id in order:
		file.set_value("notes", id, notes[id])
	return file.save(path) == OK

func enabled():
	return host != null and host.scratchpad_enabled()

# A clue is kept once; hearing the line again does not pop it up again.
func learn(id, text):
	if notes.has(id):
		return false
	notes[id] = {"text": text, "own": false, "pinned": false, "x": -1.0, "y": -1.0, "time": OS.get_unix_time()}
	order.append(id)
	save_store()
	return true

func write(text):
	text = text.strip_edges()
	if text.empty():
		return ""
	var id = ""
	while id.empty() or notes.has(id):
		serial += 1
		id = "own_%d_%d" % [OS.get_unix_time(), serial]
	notes[id] = {"text": text, "own": true, "pinned": true, "x": -1.0, "y": -1.0, "time": OS.get_unix_time()}
	order.append(id)
	save_store()
	refresh()
	return id

func remove(id):
	notes.erase(id)
	order.erase(id)
	save_store()
	refresh()

func set_pinned(id, on, at = null):
	if not notes.has(id):
		return
	notes[id].pinned = on
	# A note pinned from the list replaces its own popup.
	for scrap in popups.duplicate():
		if is_instance_valid(scrap) and scrap.get_meta("note") == id:
			popups.erase(scrap)
			scrap.queue_free()
	if at != null:
		notes[id].x = at.x
		notes[id].y = at.y
	save_store()
	refresh()

func clear_clues():
	for id in order.duplicate():
		if not notes[id].own:
			notes.erase(id)
			order.erase(id)
	save_store()
	refresh()

# Newest first, for the scratchpad list.
func listed():
	var out = []
	for i in range(order.size() - 1, -1, -1):
		out.append(order[i])
	return out

func screen_size():
	return host.layout_size if host != null else rect_size

# The picture between any side panels, so new notes never cover controls.
func area():
	return host.note_area() if host != null else Rect2(Vector2.ZERO, rect_size)

func clamp_spot(spot, size):
	var screen = screen_size()
	return Vector2(clamp(spot.x, 0, max(0, screen.x - size.x)), clamp(spot.y, 0, max(0, screen.y - size.y)))

func make_scrap(id, heading, popup):
	var scrap = PanelContainer.new()
	scrap.name = "Scrap_" + id
	scrap.set_meta("note", id)
	scrap.mouse_filter = MOUSE_FILTER_STOP
	scrap.mouse_default_cursor_shape = CURSOR_MOVE
	var style = StyleBoxFlat.new()
	style.bg_color = PAPER
	style.border_color = PAPER_EDGE
	for side in ["left", "top", "right", "bottom"]:
		style.set("border_width_" + side, 1)
	for side in ["left", "right"]:
		style.set("content_margin_" + side, 8)
	for side in ["top", "bottom"]:
		style.set("content_margin_" + side, 5)
	style.shadow_color = Color(0, 0, 0, 0.35)
	style.shadow_size = 3
	scrap.add_stylebox_override("panel", style)
	var box = VBoxContainer.new()
	box.mouse_filter = MOUSE_FILTER_IGNORE
	box.add_constant_override("separation", 2)
	scrap.add_child(box)
	var top = HBoxContainer.new()
	top.mouse_filter = MOUSE_FILTER_IGNORE
	box.add_child(top)
	var title = Label.new()
	title.text = heading
	title.size_flags_horizontal = SIZE_EXPAND_FILL
	title.mouse_filter = MOUSE_FILTER_IGNORE
	title.add_font_override("font", host.get_font_for("10px Arial"))
	title.add_color_override("font_color", HEADING)
	top.add_child(title)
	var action = Button.new()
	action.name = "Pin" if popup else "Unpin"
	action.text = "Pin" if popup else "x"
	action.flat = true
	action.focus_mode = FOCUS_NONE
	action.hint_tooltip = "Keep this note on screen" if popup else "Take this note off the screen"
	action.add_font_override("font", host.get_font_for("10px Arial"))
	action.add_color_override("font_color", HEADING)
	action.add_color_override("font_color_hover", PEN)
	action.connect("pressed", self, "pin_popup" if popup else "unpin", [id, scrap])
	top.add_child(action)
	var label = Label.new()
	label.name = "Text"
	label.text = notes[id].text
	label.autowrap = true
	label.rect_min_size.x = WIDTH
	label.mouse_filter = MOUSE_FILTER_IGNORE
	label.add_font_override("font", host.get_font_for("12px Arial"))
	label.add_color_override("font_color", PEN)
	box.add_child(label)
	scrap.connect("gui_input", self, "scrap_input", [scrap])
	add_child(scrap)
	scrap.rect_size = Vector2(WIDTH + 16, 0)
	scrap.rect_size = scrap.get_combined_minimum_size()
	return scrap

# A new clue appears briefly at the picture's left, below other new ones.
func popup(id):
	if not enabled() or not notes.has(id):
		return null
	var scrap = make_scrap(id, "Clue", true)
	popups.append(scrap)
	place_popups()
	get_tree().create_timer(POPUP_SECONDS).connect("timeout", self, "expire_popup", [scrap])
	return scrap

func place_popups():
	var place = area()
	var y = place.position.y + 64
	for scrap in popups:
		if is_instance_valid(scrap):
			scrap.rect_position = clamp_spot(Vector2(place.position.x + 8, y), scrap.rect_size)
			y += scrap.rect_size.y + 8

func expire_popup(scrap):
	if not is_instance_valid(scrap) or scrap == dragging or not popups.has(scrap):
		return
	popups.erase(scrap)
	scrap.queue_free()
	place_popups()

func pin_popup(id, scrap):
	var at = scrap.rect_position if is_instance_valid(scrap) else null
	popups.erase(scrap)
	if is_instance_valid(scrap):
		scrap.queue_free()
	set_pinned(id, true, at)
	place_popups()

func unpin(id, _scrap):
	set_pinned(id, false)

func refresh():
	for scrap in pinned.values():
		if is_instance_valid(scrap):
			scrap.queue_free()
	pinned.clear()
	if not enabled():
		for scrap in popups:
			if is_instance_valid(scrap):
				scrap.queue_free()
		popups.clear()
		return
	var place = area()
	var stack_y = place.position.y + 64
	for id in order:
		var note = notes[id]
		if not note.pinned:
			continue
		var scrap = make_scrap(id, "My note" if note.own else "Clue", false)
		var spot = Vector2(note.x, note.y)
		if note.x < 0 or note.y < 0:
			# Unplaced notes stack at the picture's right, apart from new clues.
			spot = Vector2(place.end.x - scrap.rect_size.x - 8, stack_y)
			stack_y += scrap.rect_size.y + 8
		scrap.rect_position = clamp_spot(spot, scrap.rect_size)
		pinned[id] = scrap

# Notes keep their place when the window changes shape, inside the screen.
func relayout():
	for scrap in pinned.values():
		if is_instance_valid(scrap):
			scrap.rect_position = clamp_spot(scrap.rect_position, scrap.rect_size)
	place_popups()

func scrap_input(event, scrap):
	if event is InputEventMouseButton and event.button_index == BUTTON_LEFT:
		if event.pressed:
			dragging = scrap
			drag_moved = false
			# Positions in the event are local to the scrap being dragged.
			drag_offset = event.position
			move_child(scrap, get_child_count() - 1)
		elif dragging == scrap:
			dragging = null
			if drag_moved:
				var id = scrap.get_meta("note")
				# Dragging a new clue pins it where it was dropped.
				if popups.has(scrap):
					pin_popup(id, scrap)
				elif notes.has(id):
					notes[id].x = scrap.rect_position.x
					notes[id].y = scrap.rect_position.y
					save_store()
		accept_event()
	elif event is InputEventMouseMotion and dragging == scrap:
		var spot = clamp_spot(scrap.rect_position + event.position - drag_offset, scrap.rect_size)
		if spot.distance_to(scrap.rect_position) > 0.5:
			drag_moved = true
		scrap.rect_position = spot
		accept_event()
