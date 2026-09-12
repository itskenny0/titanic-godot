package save

type SavedVar struct {
	Name string  `json:"name"`
	Type int     `json:"type"`
	Num  float64 `json:"num"`
	Str  *string `json:"str"`
}
type varSlot struct {
	Name      string
	ValueSlot int
}

func nodeVtable(d bytesView) (uint32, bool) {
	counts := map[uint32]int{}
	order := []uint32{}
	for o := 0; o+32 <= len(d); o++ {
		if d[o+8] > 11 || checkedString(d, o+8, 1, 11) == nil {
			continue
		}
		vt := d.u32(o + 20)
		if _, ok := counts[vt]; !ok {
			order = append(order, vt)
		}
		counts[vt]++
	}
	var best uint32
	n := 0
	for _, vt := range order {
		if counts[vt] > n {
			best, n = vt, counts[vt]
		}
	}
	return best, n >= 8
}
func decodeVarSlots(d bytesView) []varSlot {
	vt, ok := nodeVtable(d)
	out := []varSlot{}
	if !ok {
		return out
	}
	matches := func(slot int) bool {
		if slot < 0 || slot+28 > len(d) {
			return false
		}
		skip := max(0, int(d[slot+8])-11)
		for i := skip; i < 4; i++ {
			if d[slot+20+i] != byte(vt>>uint(8*i)) {
				return false
			}
		}
		return true
	}
	base := -1
	for o := 0; o+32 <= len(d); o++ {
		if checkedString(d, o+8, 1, 15) != nil && matches(o) {
			base = o
			break
		}
	}
	if base < 0 {
		return out
	}
	if name := checkedString(d, base+8, 1, 15); name != nil && base-32+24 >= 0 {
		out = append(out, varSlot{*name, base - 32})
	}
	for slot := base + 32; slot+32 <= len(d); slot += 32 {
		if name := checkedString(d, slot+8, 1, 15); name != nil && matches(slot-32) {
			out = append(out, varSlot{*name, slot - 32})
		}
	}
	return out
}
func decodeVars(d bytesView, pool []byte) []SavedVar {
	out := []SavedVar{}
	for _, slot := range decodeVarSlots(d) {
		v := SavedVar{Name: slot.Name, Type: int(d.u16(slot.ValueSlot + 24))}
		if v.Type == 3 {
			v.Num = float64(d.u16(slot.ValueSlot + 26))
			v.Str = checkedString(pool, int(v.Num), 0, 255)
		} else {
			v.Num = d.i32(slot.ValueSlot + 26)
		}
		out = append(out, v)
	}
	return out
}
