#!/usr/bin/env python3
"""Generate a private 2x pack from hd-pack exports. Never called by release CI."""
import argparse
from concurrent.futures import ProcessPoolExecutor
import hashlib
import io
import struct
import json
from pathlib import Path
import sys
import urllib.request

sys.path.insert(0, str(Path(__file__).resolve().parent / 'hd'))
MODELS = {
    'realesr-general-x4v3.pth': '8dc7edb9ac80ccdc30c3a5dca6616509367f05fbc184ad95b731f05bece96292',
    'realesr-general-wdn-x4v3.pth': '1641f8c4464b9f097c9fdda5589273713f67cf59f3d909e0bd688f0cee269dca',
}
MODEL_URL = 'https://github.com/xinntao/Real-ESRGAN/releases/download/v0.2.5.0/'

def initialize(weights, denoise, threads):
    import torch
    from srvgg import SRVGGNetCompact
    global model
    torch.set_num_threads(threads)
    torch.set_num_interop_threads(1)
    model = SRVGGNetCompact(num_in_ch=3, num_out_ch=3, num_feat=64, num_conv=32, upscale=4, act_type='prelu')
    strong, weak = [torch.load(Path(weights)/name, map_location='cpu', weights_only=True)['params'] for name in MODELS]
    model.load_state_dict({key: strong[key]*denoise + weak[key]*(1-denoise) for key in strong}, strict=True)
    model.eval().to(memory_format=torch.channels_last)

def upscale(task):
    import numpy as np
    import torch
    from PIL import Image
    key, source, output, size = task
    destination = Path(output)/'images'/f'{key}.png'
    if destination.exists():
        with Image.open(destination) as img:
            img.verify()
        with Image.open(destination) as img:
            if list(img.size) != [size[0]*2, size[1]*2]:
                raise ValueError(f'Wrong size in resumed image: {destination}')
    else:
        with Image.open(source) as img:
            rgba = img.convert('RGBA')
        if rgba.size != tuple(size) or hashlib.sha256(struct.pack('<II',*size)+rgba.tobytes()).hexdigest() != key:
            raise ValueError('Source image does not match export catalog: '+key)
        rgb = np.array(rgba.convert('RGB'), dtype=np.float32)/255
        tensor = torch.from_numpy(rgb.transpose(2,0,1)).unsqueeze(0).contiguous(memory_format=torch.channels_last)
        with torch.inference_mode():
            result = model(tensor).squeeze(0).clamp_(0,1).numpy().transpose(1,2,0)
        image = Image.fromarray(np.rint(result*255).astype(np.uint8))
        image = image.resize((size[0]*2,size[1]*2), Image.Resampling.LANCZOS)
        # Keep the authored alpha mask exact. No invented holes or clickable edges.
        image.putalpha(rgba.getchannel('A').resize(image.size, Image.Resampling.NEAREST))
        temp = destination.with_suffix('.tmp')
        image.save(temp, format='PNG', compress_level=6)
        temp.replace(destination)
    data = destination.read_bytes()
    webp = destination.with_suffix('.webp')
    if not webp.exists():
        with Image.open(destination) as original:
            encoded = io.BytesIO()
            original.save(encoded, format='WEBP', lossless=True, exact=True, method=4)
            encoded.seek(0)
            with Image.open(encoded) as check:
                if check.convert('RGBA').tobytes() != original.convert('RGBA').tobytes():
                    raise ValueError('Lossless WebP changed pixels: '+key)
            temporary = webp.with_suffix('.webp.tmp')
            temporary.write_bytes(encoded.getvalue()); temporary.replace(webp)
    compressed = webp.read_bytes()
    format = 'png'
    if len(compressed) < len(data):
        data, format = compressed, 'webp'
    return key, {'width':size[0]*2, 'height':size[1]*2, 'format':format, 'sha256':hashlib.sha256(data).hexdigest()}

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--input',default='.build/hd-personal',help='folder from go run ./cmd/hd-pack')
    p.add_argument('--output',default='.build/hd-personal/pack',help='private pack folder, resumable with identical settings')
    p.add_argument('--models',default='.tools/hd-models')
    p.add_argument('--workers',type=int,default=4)
    p.add_argument('--threads',type=int,default=2)
    p.add_argument('--denoise',type=float,default=0.3)
    p.add_argument('--limit',type=int,default=0,help='optional preview image count')
    args=p.parse_args()
    if args.workers<1 or args.threads<1 or not 0<=args.denoise<=1 or args.limit<0:p.error('invalid worker, thread, denoise or limit setting')
    root=Path(args.input).resolve();out=Path(args.output).resolve();weights=Path(args.models).resolve()
    catalog=json.loads((root/'catalog.json').read_text())
    if catalog['version']!=1:raise ValueError('Unsupported export catalog')
    weights.mkdir(parents=True,exist_ok=True)
    for name,digest in MODELS.items():
        dest=weights/name
        if not dest.exists():
            print('Downloading',name,flush=True)
            with urllib.request.urlopen(MODEL_URL+name,timeout=60) as response:
                data=response.read(10*1024*1024+1)
            if hashlib.sha256(data).hexdigest()!=digest:raise ValueError('Model checksum mismatch: '+name)
            dest.write_bytes(data)
        if hashlib.sha256(dest.read_bytes()).hexdigest()!=digest:raise ValueError('Model checksum mismatch: '+name)
    settings={'model':'Real-ESRGAN realesr-general-x4v3 / wdn','weights':MODELS,'denoise':args.denoise,'scale':2,'alpha':'original-nearest','gamma':0.65}
    out.mkdir(parents=True,exist_ok=True);(out/'images').mkdir(exist_ok=True)
    stamp=out/'generation.json'
    if stamp.exists() and json.loads(stamp.read_text())!=settings:raise ValueError('Output uses different generation settings; select a new output folder')
    stamp.write_text(json.dumps(settings,indent=2)+'\n')
    # Menus and the first room are useful for early visual checks.
    def priority(key):
        sources=catalog['images'][key]['sources']
        return (0 if any(s['kind']=='ui' or 'bedsit' in s['file'].lower() for s in sources) else 1, key)
    keys=sorted(catalog['images'],key=priority)
    if args.limit:keys=keys[:args.limit]
    tasks=[]
    for key in keys:
        if len(key)!=64 or any(c not in '0123456789abcdef' for c in key):raise ValueError('Invalid image key')
        entry=catalog['images'][key];w,h=entry['width'],entry['height']
        if not 1<=w<=512 or not 1<=h<=384:raise ValueError('Invalid source dimensions')
        tasks.append((key,str(root/'originals'/f'{key}.png'),str(out),(w,h)))
    manifest={'version':1,'scale':2,'model':settings['model']+f' denoise={args.denoise}','images':{}}
    with ProcessPoolExecutor(max_workers=args.workers,initializer=initialize,initargs=(str(weights),args.denoise,args.threads)) as pool:
        for i,(key,entry) in enumerate(pool.map(upscale,tasks,chunksize=1),1):
            manifest['images'][key]=entry
            if i%25==0 or i==len(tasks):print(f'Upscaled {i}/{len(tasks)} images',flush=True)
    temporary=out/'manifest.tmp'
    temporary.write_text(json.dumps(manifest,separators=(',',':'))+'\n');temporary.replace(out/'manifest.json')
    print(f'Personal HD pack: {out}. Keep the generated artwork private.',flush=True)

if __name__=='__main__':main()
