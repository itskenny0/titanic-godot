extends Control
const Style = preload("res://scripts/ui_style.gd")
var player
var elapsed = 0.0

func _ready():
	rect_min_size = Vector2(396,126)
	mouse_filter = Control.MOUSE_FILTER_IGNORE

func _process(delta):
	elapsed = fmod(elapsed+delta,5.0)
	update()

func _draw():
	var font = player.get_font_for("11px Arial")
	for index in range(2):
		var origin = Vector2(198*index+4,4)
		var selected = (index == 1) == player.wide_dialogue_enabled()
		draw_style_box(Style.plate(Color("151b1d"),Style.BRASS if selected else Color("44483f"),3),Rect2(origin,Vector2(188,104)))
		var picture = Rect2(origin+Vector2(4,4),Vector2(180,72))
		if index == 0:
			picture = Rect2(origin+Vector2(45,4),Vector2(98,51))
		draw_rect(picture,Color("34464a"))
		# A neutral silhouette illustrates the layout without shipping game art.
		draw_circle(picture.position+Vector2(picture.size.x/2,17),8,Color("b6a085"))
		draw_rect(Rect2(picture.position+Vector2(picture.size.x/2-12,25),Vector2(24,26)),Color("36322c"))
		for line in range(2):
			var r = Rect2(origin+Vector2(45 if index == 0 else 20,56+line*9),Vector2(98 if index == 0 else 148,8))
			draw_rect(r,Color("615a47") if index == 0 else Color(0,0,0,0.32))
			draw_line(r.position+Vector2(5,4),r.position+Vector2(r.size.x-16-line*20,4),Style.INK,1)
		var caption = "Classic" if index == 0 else "Widescreen"
		draw_string(font,origin+Vector2(8,96),caption,Style.INK)
	var words = ["Welcome", "aboard", "the", "Titanic."]
	var x = 128.0
	for i in range(words.size()):
		var alpha = clamp((elapsed-i*0.65)/0.12,0.0,1.0) if player.config.get_value("dialogue","timed_subtitles",false) else 1.0
		var caption = words[i]+" "
		draw_string(font,Vector2(x,122),caption,Color(0.94,0.90,0.78,alpha))
		x += player.get_font_for("11px Arial").get_string_size(caption).x
