extends Reference
const INK = Color("e4d8bc")
const BRASS = Color("a38b5d")
const PANEL = Color("1c2021")

static func plate(fill = PANEL, border = BRASS, radius = 4):
	var style = StyleBoxFlat.new()
	style.bg_color = fill
	style.border_color = border
	for side in ["left", "top", "right", "bottom"]:
		style.set("border_width_" + side, 1)
	for corner in ["top_left", "top_right", "bottom_left", "bottom_right"]:
		style.set("corner_radius_" + corner, radius)
	return style

static func create_theme():
	var theme = Theme.new()
	theme.set_stylebox("panel", "PanelContainer", plate(Color("191d1f"), Color("766548"), 6))
	for kind in ["Button", "LineEdit"]:
		for state in ["normal", "hover", "pressed", "focus", "disabled"]:
			var fill = Color("3b382d") if state in ["hover", "pressed"] else Color("262929")
			var border = Color("d0b983") if state in ["hover", "focus"] else Color("625d4d")
			var style = plate(fill, border)
			if state == "focus":
				style.bg_color = Color(0,0,0,0)
			theme.set_stylebox(state, kind, style)
		for state in ["font_color", "font_color_hover", "font_color_pressed", "font_color_focus"]:
			theme.set_color(state, kind, INK)
	theme.set_color("font_color", "Label", INK)
	return theme
