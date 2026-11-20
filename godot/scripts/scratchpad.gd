extends Control
# The player's scratchpad: torn paper scraps, like the notes Willie leaves,
# for details a conversation mentions (a cabin number, where someone waits)
# and for notes the player writes. A new clue shows briefly; its pin keeps
# it on screen, and pinned scraps can be dragged anywhere. Everything that
# is not pinned rests in the notes tray, whose button also gathers all
# pinned scraps into it and lays them out again.
#
# Clues belong to a voyage: they are stored beside each save in a ".notes"
# file and restored with it, and a new game starts without any. The
# player's own notes belong to the player and live in user://scratchpad.cfg.
const PAPER = Color("efe7d2")
const PAPER_SHADE = Color("cbbd99")
const PAPER_EDGE = Color(0.36, 0.29, 0.19, 0.55)
const PEN = Color("2b2216")
const HEADING = Color("7a5a2e")
const PIN = Color("b3261e")
const WIDTH = 152.0
const POPUP_SECONDS = 7.0
const REST_ALPHA = 0.86
const TRAY_SIZE = Vector2(44, 38)

var path = "user://scratchpad.cfg"
var notes = {}
var order = []
# Set by the player: fonts, the free picture area and whether this is on.
var host = null
var popups = []
var pinned = {}
var collapsed = false
var unseen = 0
var tray = null
var dragging = null
var drag_offset = Vector2.ZERO
var drag_start = Vector2.ZERO
var drag_moved = false
var serial = 0
var animations = {}

func _ready():
	mouse_filter = MOUSE_FILTER_IGNORE
	set_process(false)
	tray = Control.new()
	tray.name = "Tray"
	tray.rect_size = TRAY_SIZE
	tray.mouse_filter = MOUSE_FILTER_STOP
	tray.mouse_default_cursor_shape = CURSOR_POINTING_HAND
	tray.hint_tooltip = "Notes"
	tray.connect("draw", self, "draw_tray")
	tray.connect("gui_input", self, "tray_input")
	tray.connect("mouse_entered", self, "hover_tray", [true])
	tray.connect("mouse_exited", self, "hover_tray", [false])
	tray.hide()
	add_child(tray)

# ---------------------------------------------------------------- storage

func load_store():
	var keep = {}
	for id in order:
		if not notes[id].own:
			keep[id] = notes[id]
	notes = {}
	order = []
	var file = ConfigFile.new()
	if file.load(path) == OK:
		collapsed = file.get_value("scratchpad", "collapsed", false)
		for id in file.get_value("scratchpad", "order", []):
			var note = file.get_value("notes", id, null)
			# Older builds kept clues here too; they now belong to saves.
			if note is Dictionary and note.has("text") and note.get("own", false):
				notes[id] = note
				order.append(id)
	for id in keep:
		notes[id] = keep[id]
		order.append(id)

func save_store():
	var file = ConfigFile.new()
	var own = []
	for id in order:
		if notes[id].own:
			own.append(id)
			file.set_value("notes", id, notes[id])
	file.set_value("scratchpad", "order", own)
	file.set_value("scratchpad", "collapsed", collapsed)
	return file.save(path) == OK

func voyage_path(save_path):
	return save_path + ".notes"

# A save was written: keep this voyage's clues beside it.
func save_voyage(save_path):
	var file = ConfigFile.new()
	var ids = []
	for id in order:
		if not notes[id].own:
			ids.append(id)
			file.set_value("clues", id, notes[id])
	file.set_value("voyage", "order", ids)
	return file.save(voyage_path(save_path)) == OK

# A game starts: from a save, with the clues stored beside it, or new.
func begin_voyage(save_path = ""):
	for id in order.duplicate():
		if not notes[id].own:
			notes.erase(id)
			order.erase(id)
	unseen = 0
	var file = ConfigFile.new()
	if not save_path.empty() and file.load(voyage_path(save_path)) == OK:
		for id in file.get_value("voyage", "order", []):
			var note = file.get_value("clues", id, null)
			if note is Dictionary and note.has("text") and not notes.has(id):
				note.own = false
				notes[id] = note
				order.append(id)
	clear_popups()
	refresh()

func enabled():
	return host != null and host.scratchpad_enabled()

# A clue is kept once per voyage; hearing the line again changes nothing.
func learn(id, text):
	if notes.has(id):
		return false
	notes[id] = {"text": text, "own": false, "pinned": false, "x": -1.0, "y": -1.0, "time": OS.get_unix_time()}
	order.append(id)
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
	collapsed = false
	save_store()
	refresh()
	return id

func remove(id):
	if not notes.has(id):
		return
	var own = notes[id].own
	notes.erase(id)
	order.erase(id)
	if own:
		save_store()
	refresh()

func set_pinned(id, on, at = null):
	if not notes.has(id):
		return
	notes[id].pinned = on
	for scrap in popups.duplicate():
		if is_instance_valid(scrap) and scrap.get_meta("note") == id:
			popups.erase(scrap)
			scrap.queue_free()
	if at != null:
		notes[id].x = at.x
		notes[id].y = at.y
	if on:
		collapsed = false
	save_store()
	refresh()
	place_popups()

func clear_clues():
	for id in order.duplicate():
		if not notes[id].own:
			notes.erase(id)
			order.erase(id)
	clear_popups()
	refresh()

# Newest first, for the scratchpad list.
func listed():
	var out = []
	for i in range(order.size() - 1, -1, -1):
		out.append(order[i])
	return out

func pinned_count():
	var count = 0
	for id in order:
		if notes[id].pinned:
			count += 1
	return count

# ---------------------------------------------------------------- layout

# The picture between any side panels, so notes never start over controls.
func area():
	return host.note_area() if host != null else Rect2(Vector2.ZERO, rect_size)

func screen_size():
	return host.layout_size if host != null else rect_size

func clamp_spot(spot, size):
	var screen = screen_size()
	return Vector2(clamp(spot.x, 0, max(0, screen.x - size.x)), clamp(spot.y, 0, max(0, screen.y - size.y)))

func tray_spot():
	var place = area()
	return Vector2(place.end.x - TRAY_SIZE.x - 8, place.position.y + 8)

func tray_centre():
	return tray.rect_position + TRAY_SIZE / 2

func tray_rect():
	return Rect2(tray.rect_position, TRAY_SIZE).grow(6)

func font(css):
	return host.get_font_for(css)

# Each note keeps its own tear and tilt, whatever the engine version.
func seed_of(id):
	return id.hash() & 0x7fffffff

# ---------------------------------------------------------------- scraps

func make_scrap(id, popup):
	var note = notes[id]
	var scrap = PanelContainer.new()
	scrap.name = "Scrap_" + id
	scrap.set_meta("note", id)
	scrap.set_meta("popup", popup)
	scrap.mouse_filter = MOUSE_FILTER_STOP
	scrap.mouse_default_cursor_shape = CURSOR_MOVE
	scrap.hint_tooltip = "Drag to move it. Drop it on the notes tray to put it away."
	var style = StyleBoxEmpty.new()
	style.content_margin_left = 13
	style.content_margin_right = 13
	style.content_margin_top = 14
	style.content_margin_bottom = 10
	scrap.add_stylebox_override("panel", style)
	var box = VBoxContainer.new()
	box.mouse_filter = MOUSE_FILTER_IGNORE
	box.add_constant_override("separation", 1)
	scrap.add_child(box)
	var heading = Label.new()
	heading.name = "Heading"
	heading.text = "New clue — pin it to keep it" if popup else ("My note" if note.own else "Clue")
	heading.mouse_filter = MOUSE_FILTER_IGNORE
	heading.add_font_override("font", font("9px Serif Italic"))
	heading.add_color_override("font_color", HEADING)
	box.add_child(heading)
	var label = Label.new()
	label.name = "Text"
	label.text = note.text
	label.autowrap = true
	label.rect_min_size.x = WIDTH
	label.mouse_filter = MOUSE_FILTER_IGNORE
	label.add_font_override("font", font("12px Serif"))
	label.add_color_override("font_color", PEN)
	box.add_child(label)
	scrap.connect("draw", self, "draw_paper", [scrap])
	scrap.connect("gui_input", self, "scrap_input", [scrap])
	scrap.connect("mouse_entered", self, "hover_scrap", [scrap, true])
	scrap.connect("mouse_exited", self, "hover_scrap", [scrap, false])
	add_child(scrap)
	move_child(tray, get_child_count() - 1)
	scrap.rect_size = Vector2(WIDTH + 26, 0)
	scrap.rect_size = scrap.get_combined_minimum_size()
	scrap.rect_pivot_offset = scrap.rect_size / 2
	# The pin sits above the paper, outside the container's layout.
	var pin = Control.new()
	pin.name = "Pin"
	pin.mouse_filter = MOUSE_FILTER_STOP
	pin.mouse_default_cursor_shape = CURSOR_POINTING_HAND
	pin.hint_tooltip = "Pin this clue on the screen" if popup else "Unpin it and put it away in the notes tray"
	pin.connect("draw", self, "draw_pin", [pin, scrap])
	pin.connect("gui_input", self, "pin_input", [pin, scrap])
	pin.connect("mouse_entered", self, "hover_pin", [pin, true])
	pin.connect("mouse_exited", self, "hover_pin", [pin, false])
	pin.set_as_toplevel(false)
	scrap.add_child(pin)
	place_pin(scrap)
	var rng = RandomNumberGenerator.new()
	rng.seed = seed_of(id)
	scrap.rect_rotation = rng.randf_range(-2.6, 2.6)
	scrap.modulate.a = REST_ALPHA
	scrap.connect("sort_children", self, "place_pin", [scrap])
	return scrap

# A container lays out every child; put the pin back after each layout.
func place_pin(scrap):
	if not is_instance_valid(scrap):
		return
	var pin = scrap.get_node_or_null("Pin")
	if pin != null:
		pin.rect_position = Vector2(scrap.rect_size.x - 30, -6)
		pin.rect_size = Vector2(26, 26)

# A torn scrap: nearly straight top and bottom, ragged torn sides. Each
# side's points move strictly along it and stay clear of the corners, so
# the outline never crosses itself.
func paper_outline(id, size):
	var rng = RandomNumberGenerator.new()
	rng.seed = seed_of(id) + int(size.x) * 7 + int(size.y)
	var w = size.x
	var h = size.y
	var points = PoolVector2Array()
	points.append(Vector2(rng.randf_range(0, 1.5), rng.randf_range(0, 1.0)))
	var x = 10.0
	while x < w - 10:
		points.append(Vector2(x, rng.randf_range(0, 1.4)))
		x += rng.randf_range(9, 20)
	points.append(Vector2(w - rng.randf_range(0, 1.5), rng.randf_range(0, 1.0)))
	var y = 7.0
	while y < h - 7:
		points.append(Vector2(w - rng.randf_range(0, 5.0), y))
		y += rng.randf_range(3, 6)
	points.append(Vector2(w - rng.randf_range(0, 1.5), h - rng.randf_range(0, 1.0)))
	x = w - 10.0
	while x > 10:
		points.append(Vector2(x, h - rng.randf_range(0, 1.8)))
		x -= rng.randf_range(9, 20)
	points.append(Vector2(rng.randf_range(0, 1.5), h - rng.randf_range(0, 1.0)))
	y = h - 7.0
	while y > 7:
		points.append(Vector2(rng.randf_range(0, 5.0), y))
		y -= rng.randf_range(3, 6)
	return points

func draw_paper(scrap):
	if not is_instance_valid(scrap):
		return
	var id = scrap.get_meta("note")
	var size = scrap.rect_size
	var outline = paper_outline(id, size)
	var lifted = scrap == dragging
	var shadow = PoolVector2Array()
	var offset = Vector2(3, 5) if lifted else Vector2(1.5, 2.5)
	for p in outline:
		shadow.append(p + offset)
	scrap.draw_colored_polygon(shadow, Color(0, 0, 0, 0.42 if lifted else 0.3))
	scrap.draw_colored_polygon(outline, PAPER)
	# Age: faint stains and a darker torn rim.
	var rng = RandomNumberGenerator.new()
	rng.seed = seed_of(id) + 11
	for i in range(3):
		var at = Vector2(rng.randf_range(0.15, 0.85) * size.x, rng.randf_range(0.2, 0.8) * size.y)
		scrap.draw_circle(at, rng.randf_range(8, 18), Color(PAPER_SHADE.r, PAPER_SHADE.g, PAPER_SHADE.b, 0.12))
	var rim = PoolVector2Array(outline)
	rim.append(outline[0])
	scrap.draw_polyline(rim, PAPER_EDGE, 1.0, true)

func draw_pin(pin, scrap):
	if not is_instance_valid(scrap):
		return
	var hovered = pin.has_meta("hover") and pin.get_meta("hover")
	var centre = Vector2(13, 13)
	if scrap.get_meta("popup"):
		# A ghost pin: press it to keep the clue.
		pin.draw_circle(centre + Vector2(1, 1.5), 6, Color(0, 0, 0, 0.18 if hovered else 0.0))
		pin.draw_circle(centre, 6, Color(PIN.r, PIN.g, PIN.b, 0.9 if hovered else 0.32))
		pin.draw_arc(centre, 6, 0, TAU, 24, Color(PIN.r, PIN.g, PIN.b, 0.9), 1.2, true)
		return
	pin.draw_circle(centre + Vector2(1.2, 2.0), 6, Color(0, 0, 0, 0.35))
	pin.draw_circle(centre, 6, PIN.lightened(0.18) if hovered else PIN)
	pin.draw_circle(centre + Vector2(-1.8, -1.8), 1.9, Color(1, 1, 1, 0.55))

func hover_pin(pin, on):
	pin.set_meta("hover", on)
	pin.update()

# A mouse makes the note under it fully opaque, to read it; a finger
# leaves no hover behind.
func hover_scrap(scrap, on):
	if host != null and host.touch_enabled:
		return
	if is_instance_valid(scrap) and not animations.has(scrap):
		scrap.modulate.a = 1.0 if on or scrap == dragging else REST_ALPHA

func pin_input(event, pin, scrap):
	if not (event is InputEventMouseButton and event.button_index == BUTTON_LEFT):
		return
	pin.accept_event()
	if event.pressed:
		return
	press_pin(scrap)

func press_pin(scrap):
	var id = scrap.get_meta("note")
	if scrap.get_meta("popup"):
		pin_popup(id, scrap)
	else:
		put_away(id, scrap)

# ---------------------------------------------------------------- popups

# A new clue appears briefly at the picture's left, below other new ones.
func popup(id):
	if not enabled() or not notes.has(id):
		return null
	var scrap = make_scrap(id, true)
	popups.append(scrap)
	place_popups()
	animate(scrap, scrap.rect_position - Vector2(0, 14), scrap.rect_position, 0.0, REST_ALPHA, 0.25)
	get_tree().create_timer(POPUP_SECONDS).connect("timeout", self, "expire_popup", [scrap])
	update_tray()
	return scrap

func place_popups():
	var place = area()
	var y = place.position.y + 64
	for scrap in popups:
		if is_instance_valid(scrap):
			var spot = clamp_spot(Vector2(place.position.x + 10, y), scrap.rect_size)
			if animations.has(scrap):
				animations[scrap].to = spot
			else:
				scrap.rect_position = spot
			y += scrap.rect_size.y + 10

# A clue nobody pinned is not lost: it flies into the notes tray.
func expire_popup(scrap):
	if not is_instance_valid(scrap) or scrap == dragging or not popups.has(scrap):
		return
	popups.erase(scrap)
	unseen += 1
	fly_to_tray(scrap)
	place_popups()
	update_tray()

func pin_popup(id, scrap):
	var at = scrap.rect_position if is_instance_valid(scrap) else null
	popups.erase(scrap)
	if is_instance_valid(scrap):
		scrap.queue_free()
	set_pinned(id, true, at)

func put_away(id, scrap):
	if not notes.has(id):
		return
	notes[id].pinned = false
	save_store()
	pinned.erase(id)
	if is_instance_valid(scrap):
		fly_to_tray(scrap)
	update_tray()

func clear_popups():
	for scrap in popups:
		if is_instance_valid(scrap):
			animations.erase(scrap)
			scrap.queue_free()
	popups.clear()

# ---------------------------------------------------------------- pinned

func refresh():
	for scrap in pinned.values():
		if is_instance_valid(scrap):
			animations.erase(scrap)
			scrap.queue_free()
	pinned.clear()
	if not enabled():
		clear_popups()
		update_tray()
		return
	var place = area()
	var stack_y = place.position.y + 56
	for id in order:
		var note = notes[id]
		if not note.pinned:
			continue
		var scrap = make_scrap(id, false)
		var spot = Vector2(note.x, note.y)
		if note.x < 0 or note.y < 0:
			# Unplaced notes stack down the picture's right, below the tray.
			spot = Vector2(place.end.x - scrap.rect_size.x - 12, stack_y)
			stack_y += scrap.rect_size.y + 10
		scrap.rect_position = clamp_spot(spot, scrap.rect_size)
		scrap.set_meta("home", scrap.rect_position)
		scrap.visible = not collapsed
		pinned[id] = scrap
	update_tray()

# Notes keep their place when the window changes shape, inside the screen.
func relayout():
	for scrap in pinned.values():
		if is_instance_valid(scrap) and not animations.has(scrap):
			scrap.rect_position = clamp_spot(scrap.get_meta("home") if scrap.has_meta("home") else scrap.rect_position, scrap.rect_size)
			scrap.set_meta("home", scrap.rect_position)
	place_popups()
	update_tray()

# ---------------------------------------------------------------- tray

func update_tray():
	if tray == null:
		return
	tray.visible = enabled() and not order.empty()
	tray.rect_position = tray_spot()
	var count = pinned_count()
	if collapsed and count > 0:
		tray.hint_tooltip = "Lay out your %d pinned note%s again" % [count, "" if count == 1 else "s"]
	elif count > 0:
		tray.hint_tooltip = "Gather your notes into the tray"
	else:
		tray.hint_tooltip = "Open the scratchpad"
	tray.update()

func hover_tray(on):
	tray.set_meta("hover", on)
	tray.update()

func draw_tray():
	var hovered = tray.has_meta("hover") and tray.get_meta("hover")
	# A little stack of scraps; it fills up while notes are gathered.
	var sheets = [[Vector2(9, 12), -9.0], [Vector2(8, 10), 6.0], [Vector2(9, 8), -1.5]]
	for sheet in sheets:
		var size = Vector2(26, 19)
		var centre = sheet[0] + size / 2
		var angle = deg2rad(sheet[1])
		var corners = PoolVector2Array()
		for c in [Vector2(-1, -1), Vector2(1, -1), Vector2(1, 1), Vector2(-1, 1)]:
			corners.append(centre + (c * size / 2).rotated(angle))
		var shadow = PoolVector2Array()
		for p in corners:
			shadow.append(p + Vector2(1, 1.5))
		tray.draw_colored_polygon(shadow, Color(0, 0, 0, 0.35))
		tray.draw_colored_polygon(corners, PAPER if hovered else PAPER.darkened(0.08))
		var rim = PoolVector2Array(corners)
		rim.append(corners[0])
		tray.draw_polyline(rim, PAPER_EDGE, 1.0, true)
	for i in range(3):
		var y = 13.5 + i * 4.0
		tray.draw_line(Vector2(13, y), Vector2(29 - i * 3, y), Color(PEN.r, PEN.g, PEN.b, 0.45), 1.0)
	# A pin on the stack while notes are laid out on the screen.
	if not collapsed and pinned_count() > 0:
		tray.draw_circle(Vector2(31, 9), 3, PIN)
	if collapsed and pinned_count() > 0:
		var f = font("9px Arial")
		var label = str(pinned_count())
		tray.draw_string(f, Vector2(22 - f.get_string_size(label).x / 2, 35), label, Color(1, 1, 1, 0.9))
	if unseen > 0:
		var centre = Vector2(TRAY_SIZE.x - 6, 6)
		tray.draw_circle(centre, 6.5, PIN)
		var badge = font("9px Arial")
		var count = str(min(unseen, 9))
		tray.draw_string(badge, centre + Vector2(-badge.get_string_size(count).x / 2, 3.5), count, Color(1, 1, 1))

func tray_input(event):
	if not event is InputEventMouseButton:
		return
	tray.accept_event()
	if event.pressed:
		return
	if event.button_index == BUTTON_RIGHT:
		open_list()
	elif event.button_index == BUTTON_LEFT:
		activate_tray()

# Gather pinned notes, lay them out again, or, with none pinned, open the list.
func activate_tray():
	if pinned_count() > 0:
		set_collapsed(not collapsed)
	else:
		open_list()

func open_list():
	unseen = 0
	update_tray()
	if host != null:
		host.open_scratchpad()

func set_collapsed(on):
	collapsed = on
	save_store()
	for id in pinned:
		var scrap = pinned[id]
		if not is_instance_valid(scrap):
			continue
		var home = scrap.get_meta("home") if scrap.has_meta("home") else scrap.rect_position
		var gathered = tray_centre() - scrap.rect_size / 2
		if on:
			animate(scrap, home, gathered, scrap.modulate.a, 0.0, 0.22, "hide")
		else:
			scrap.show()
			animate(scrap, gathered, home, 0.0, REST_ALPHA, 0.26, "grow")
	update_tray()

func fly_to_tray(scrap):
	tray.visible = enabled()
	tray.rect_position = tray_spot()
	animate(scrap, scrap.rect_position, tray_centre() - scrap.rect_size / 2, scrap.modulate.a, 0.0, 0.32, "free")

# ---------------------------------------------------------------- motion

func animate(node, from, to, alpha_from, alpha_to, seconds, finish = ""):
	node.rect_position = from
	node.modulate.a = alpha_from
	animations[node] = {"from": from, "to": to, "a0": alpha_from, "a1": alpha_to, "t": 0.0, "d": seconds, "finish": finish}
	set_process(true)

func _process(delta):
	for node in animations.keys():
		if not is_instance_valid(node):
			animations.erase(node)
			continue
		var a = animations[node]
		a.t = min(a.d, a.t + delta)
		var k = 1.0 - pow(1.0 - a.t / a.d, 3)
		node.rect_position = a.from.linear_interpolate(a.to, k)
		node.modulate.a = lerp(a.a0, a.a1, k)
		var scale = 1.0
		if a.finish in ["free", "hide"]:
			scale = lerp(1.0, 0.2, k)
		elif a.finish == "grow":
			scale = lerp(0.2, 1.0, k)
		node.rect_scale = Vector2(scale, scale)
		if a.t >= a.d:
			animations.erase(node)
			node.rect_scale = Vector2.ONE
			if a.finish == "free":
				node.queue_free()
			elif a.finish == "hide":
				node.hide()
				node.rect_position = a.from
	if animations.empty():
		set_process(false)

func finish_animations():
	for node in animations.keys():
		animations[node].t = animations[node].d
	_process(0.0)

# ---------------------------------------------------------------- dragging

# The pointer in this layer's space; events on a scrap come in its tilted space.
func pointer_of(event):
	return event.global_position - get_global_rect().position

func scrap_input(event, scrap):
	if event is InputEventMouseButton and event.button_index == BUTTON_LEFT:
		scrap.accept_event()
		if event.pressed:
			dragging = scrap
			drag_moved = false
			animations.erase(scrap)
			scrap.rect_scale = Vector2.ONE
			# Events arrive in the tilted scrap's space; work in ours.
			drag_start = pointer_of(event)
			drag_offset = drag_start - scrap.rect_position
			move_child(scrap, get_child_count() - 1)
			move_child(tray, get_child_count() - 1)
			scrap.modulate.a = 1.0
			scrap.update()
		elif dragging == scrap:
			dragging = null
			scrap.update()
			hover_tray(false)
			var id = scrap.get_meta("note")
			if drag_moved and tray.visible and tray_rect().has_point(pointer_of(event)):
				# Dropped on the tray: put it away.
				if popups.has(scrap):
					popups.erase(scrap)
					place_popups()
				put_away(id, scrap)
			elif drag_moved:
				# Dragging a new clue pins it where it was dropped.
				if popups.has(scrap):
					pin_popup(id, scrap)
				elif notes.has(id):
					notes[id].x = scrap.rect_position.x
					notes[id].y = scrap.rect_position.y
					scrap.set_meta("home", scrap.rect_position)
					save_store()
			if is_instance_valid(scrap):
				scrap.modulate.a = REST_ALPHA
	elif event is InputEventMouseMotion and dragging == scrap:
		scrap.accept_event()
		var mouse = pointer_of(event)
		if mouse.distance_to(drag_start) > 3:
			drag_moved = true
		scrap.rect_position = clamp_spot(mouse - drag_offset, scrap.rect_size)
		var over = tray.visible and tray_rect().has_point(mouse)
		if over != (tray.has_meta("hover") and tray.get_meta("hover")):
			hover_tray(over)
