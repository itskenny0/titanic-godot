extends SceneTree
var failed = false
func check(value, label):
	if not value:
		failed = true
		printerr("FAIL: ", label)
func bridge_call(method, args, bytes):
	if method == "read":
		return PoolByteArray([42, 0, 255])
	if method == "echo":
		return bytes
	return "null"
func _init():
	var engine = ClassDB.instance("DreamRuntime") if ClassDB.class_exists("DreamRuntime") else load("res://native/runtime.gdns").new()
	check(engine != null, "native runtime loads")
	if engine == null:
		quit(1)
		return
	check(engine.initialize(self, "globalThis.answer=0; Promise.resolve(42).then(v=>answer=v);") == "", "promise initialization")
	check(engine.query("String(answer)") == "42", "promise jobs run")
	var bytes = engine.buffer("__native('echo','{}',new Uint8Array([0,127,128,255]).buffer)")
	check(bytes == PoolByteArray([0,127,128,255]), "binary bridge preserves bytes")
	check(engine.buffer("__native('read','{}')") == PoolByteArray([42,0,255]), "reads preserve a nil third callback argument")
	var pixel_test = """(() => {
 const input=new Uint8Array(4099),colors=new Uint8ClampedArray(1033),output=new Uint8ClampedArray(16404).fill(17);
 const palette=colors.subarray(5,1029),indices=input.subarray(3),dst=output.subarray(7,16391);
 for(let i=0;i<256;i++){palette[i*4]=i;palette[i*4+1]=255-i;palette[i*4+2]=(i*37)&255;palette[i*4+3]=0;}
 for(let i=0;i<indices.length;i++)indices[i]=(i*71+19)&255;
 if(__indexedRGBA(indices,palette,dst,4096)!==dst)return 'wrong output object';
 for(let i=0;i<4096;i++)for(let c=0;c<4;c++)if(dst[i*4+c]!== (c===3?255:palette[indices[i]*4+c]))return 'pixel mismatch';
 if(output[6]!==17||output[16391]!==17)return 'wrote outside view';
 for(const args of [[indices,palette,dst,5000],[indices,palette.subarray(1),dst,1],[new Uint16Array(8),palette,dst,1],[indices,palette,dst,-1]]){
  let rejected=false;try{__indexedRGBA(...args);}catch(e){rejected=true;}if(!rejected)return 'accepted invalid input';
 }
 return 'pixels match';
})()"""
	check(engine.query(pixel_test) == "pixels match", "native pixels match reference, preserve subarrays, and reject invalid buffers")
	if "--go-codecs" in OS.get_cmdline_args() or engine.query("typeof __decodeFrame") == "function":
		var codec_test = """(() => {
 const source=new Uint8Array([99,2,0,2,0,4,1,2,4,3,4,99]),pixels=new Uint8Array(6).fill(99),z=new Uint8Array(6).fill(99);
 if(__decodeFrame(source.subarray(1,11),pixels.subarray(1,5),z.subarray(1,5),false)!==-1)return 'depth offset';
 if(String(pixels)!=='99,1,2,3,4,99'||z[0]!==99||z[5]!==99)return 'frame subarray';
 const audio=new Uint8Array(51),dv=new DataView(audio.buffer);dv.setInt32(0,0x10000,true);dv.setInt16(26,2,true);dv.setInt32(28,22050,true);dv.setInt32(36,6,true);dv.setInt32(44,48,true);audio.set([0x81,0x7f,0xc0],48);
 const samples=new Float32Array(5).fill(99);__decodeAudio(audio,samples.subarray(1,4),false);
 if(String(samples)!=='99,0.015625,0.0146484375,-1,99')return 'audio samples';
 const pcm=new Uint8Array(14).fill(99);__stereoPCM(new Float32Array([-1,.5,1]),pcm.subarray(1,13),1,-1);
 const p=new DataView(pcm.buffer,1,12);if(p.getInt16(0,true)!==-32767||p.getInt16(4,true)!==16384||p.getInt16(8,true)!==32767||p.getInt16(2,true)!==0||pcm[0]!==99||pcm[13]!==99)return 'stereo PCM';
 for(const fn of [()=>__decodeFrame(source,new Uint8Array(1),new Uint8Array(1),false),()=>__decodeAudio(audio,new Float32Array(1),false),()=>__stereoPCM(new Float32Array(3),new Uint8Array(1),1,0)]){let rejected=false;try{fn();}catch(e){rejected=true;}if(!rejected)return 'invalid buffer accepted';}
 return 'codecs match';
})()"""
		check(engine.query(codec_test) == "codecs match", "Go codecs preserve subarrays and reject short buffers")
	check(not engine.execute("throw Error('expected recovery test')").empty(), "script failures are reported")
	check(engine.execute("answer=43") == "" and engine.query("String(answer)") == "43", "a past error does not poison later successful calls")
	check(engine.execute("globalThis.liveMemory = new Uint8Array(270*1024*1024)") == "", "live memory fixture fits heap")
	for _i in range(12):
		check(engine.execute("(() => { const oldRoom={pixels:new Uint8Array(16*1024*1024)};oldRoom.self=oldRoom; })()") == "", "cyclic room caches are collected before the hard heap limit")
	check(engine.execute("delete globalThis.liveMemory") == "", "memory fixture released")
	var store = load("res://scripts/save_store.gd").new()
	var id = "retanic-test-" + str(OS.get_ticks_usec())
	store.root = "user://test-saves/" + id
	store.archives = "user://test-archives/" + id
	Directory.new().make_dir_recursive(store.root)
	Directory.new().make_dir_recursive(store.archives)
	check(store.write("voyage", bytes), "first save")
	check(store.write("VOYAGE", PoolByteArray([1,2,3])), "case-insensitive replacement")
	check(store.list_saves().size() == 1, "one save after replacement")
	var d = Directory.new()
	d.open(store.archives)
	d.list_dir_begin(true,true)
	var archived = d.get_next()
	check(not archived.empty(), "previous save archived")
	var f = File.new()
	f.open(store.archives.plus_file(archived),File.READ)
	check(f.get_buffer(f.get_len()) == bytes, "archived bytes unchanged")
	f.close()
	check(store.safe_name("../../escape") == "escape.ti", "save names cannot escape directory")
	print("PORTABLE TESTS ", "FAIL" if failed else "PASS")
	engine = null
	quit(1 if failed else 0)
