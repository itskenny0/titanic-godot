#!/usr/bin/env python3
"""Generate a personal 2x pack from hd-pack exports. Never called by release CI."""
import argparse
from concurrent.futures import ProcessPoolExecutor
from fnmatch import fnmatchcase
import hashlib
import io
import struct
import json
from pathlib import Path
import sys
import urllib.request

sys.path.insert(0, str(Path(__file__).resolve().parent / 'hd'))
from models import PRESETS, load_model

def initialize(weights, denoise, threads, preset, needs_model=True):
    if not needs_model:
        return
    import torch
    global model, network_scale
    torch.set_num_threads(threads)
    torch.set_num_interop_threads(1)
    model = load_model(preset, weights, denoise)
    network_scale = PRESETS[preset]['scale']

def upscale(task):
    from PIL import Image
    key, source, output, size, nearest = task
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
        if nearest:
            image = rgba.resize((size[0]*2,size[1]*2), Image.Resampling.NEAREST)
        else:
            import numpy as np
            import torch
            rgb = np.array(rgba.convert('RGB'), dtype=np.float32)/255
            tensor = torch.from_numpy(rgb.transpose(2,0,1)).unsqueeze(0).contiguous(memory_format=torch.channels_last)
            w,h = size
            if network_scale == 2 and (w%2 or h%2):
                tensor = torch.nn.functional.pad(tensor, (0,w%2,0,h%2), mode='replicate')
            with torch.inference_mode():
                result = model(tensor)[:,:,:h*network_scale,:w*network_scale].squeeze(0).clamp_(0,1).numpy().transpose(1,2,0)
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

def nearest_keys(images, selectors):
    """Select pixel hashes or case-insensitive file:name shell patterns."""
    selected = set()
    for selector in selectors:
        pattern = selector.replace('\\', '/').casefold()
        matches = {key for key, entry in images.items()
                   if key == pattern or any(fnmatchcase(
                       (source['file'].replace('\\', '/')+':'+source['name']).casefold(), pattern)
                       for source in entry['sources'])}
        if not matches:
            raise ValueError('No artwork matches --nearest '+selector)
        selected.update(matches)
    return selected

def prepare_output(out, settings):
    """Changing nearest selections regenerates only the affected images."""
    out.mkdir(parents=True, exist_ok=True)
    (out/'images').mkdir(exist_ok=True)
    stamp = out/'generation.json'
    if stamp.exists():
        previous = json.loads(stamp.read_text())
        old_nearest = set(previous.pop('nearest', []))
        current = dict(settings)
        new_nearest = set(current.pop('nearest', []))
        if previous != current:
            raise ValueError('Output uses different generation settings; select a new output folder')
        changed = old_nearest ^ new_nearest
        if changed:
            # Remove the old manifest before changing its files. Commit settings
            # last so an interrupted invalidation is safely repeated on resume.
            (out/'manifest.json').unlink(missing_ok=True)
            for key in changed:
                if len(key) != 64 or any(c not in '0123456789abcdef' for c in key):
                    raise ValueError('Invalid nearest image key in generation settings')
                for suffix in ('.png', '.webp'):
                    (out/'images'/(key+suffix)).unlink(missing_ok=True)
            print(f'Regenerating {len(changed)} images with changed scaling', flush=True)
    temporary = stamp.with_suffix('.tmp')
    temporary.write_text(json.dumps(settings, indent=2)+'\n')
    temporary.replace(stamp)

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--input',default='.build/hd-personal',help='folder from go run ./cmd/hd-pack')
    p.add_argument('--output',default='.build/hd-personal/pack',help='pack folder, resumable with identical settings')
    p.add_argument('--models',default='.tools/hd-models')
    p.add_argument('--model',choices=PRESETS,default='riven',help='riven removes dithering; compact reproduces the first HD pack')
    p.add_argument('--workers',type=int,default=4)
    p.add_argument('--threads',type=int,default=2)
    p.add_argument('--denoise',type=float,help='compact model only, default 0.3')
    p.add_argument('--limit',type=int,default=0,help='optional preview image count')
    p.add_argument('--nearest',action='append',default=[],metavar='SELECTOR',help='use exact nearest-neighbor 2x for a pixel hash or file:name glob; repeat to select more assets')
    args=p.parse_args()
    if args.model == 'compact' and args.denoise is None:args.denoise=0.3
    if args.model != 'compact' and args.denoise is not None:p.error('--denoise only applies to --model compact')
    if args.workers<1 or args.threads<1 or args.denoise is not None and not 0<=args.denoise<=1 or args.limit<0:p.error('invalid worker, thread, denoise or limit setting')
    spec=PRESETS[args.model]
    root=Path(args.input).resolve();out=Path(args.output).resolve();weights=Path(args.models).resolve()
    catalog=json.loads((root/'catalog.json').read_text())
    if catalog['version']!=1:raise ValueError('Unsupported export catalog')
    nearest=nearest_keys(catalog['images'],args.nearest)
    # Menus and the first room are useful for early visual checks.
    def priority(key):
        sources=catalog['images'][key]['sources']
        return (0 if any(s['kind']=='ui' or 'bedsit' in s['file'].lower() for s in sources) else 1, key)
    keys=sorted(catalog['images'],key=priority)
    if args.limit:keys=keys[:args.limit]
    needs_model=any(key not in nearest for key in keys)
    weights.mkdir(parents=True,exist_ok=True)
    for name,(url,digest) in (spec['files'].items() if needs_model else []):
        dest=weights/name
        if not dest.exists():
            print('Downloading',name,flush=True)
            with urllib.request.urlopen(url,timeout=60) as response:
                data=response.read(100*1024*1024+1)
            if hashlib.sha256(data).hexdigest()!=digest:raise ValueError('Model checksum mismatch: '+name)
            dest.write_bytes(data)
        if hashlib.sha256(dest.read_bytes()).hexdigest()!=digest:raise ValueError('Model checksum mismatch: '+name)
    settings={'model':spec['name'],'weights':{name:digest for name,(_,digest) in spec['files'].items()},'denoise':args.denoise,'scale':2,'alpha':'original-nearest','gamma':0.65}
    if nearest:settings['nearest']=sorted(nearest)
    prepare_output(out,settings)
    tasks=[]
    for key in keys:
        if len(key)!=64 or any(c not in '0123456789abcdef' for c in key):raise ValueError('Invalid image key')
        entry=catalog['images'][key];w,h=entry['width'],entry['height']
        if not 1<=w<=512 or not 1<=h<=384:raise ValueError('Invalid source dimensions')
        tasks.append((key,str(root/'originals'/f'{key}.png'),str(out),(w,h),key in nearest))
    label=settings['model']+(f' denoise={args.denoise}' if args.denoise is not None else '')
    manifest={'version':1,'scale':2,'model':label,'images':{}}
    if nearest:manifest['nearest']=sorted(nearest.intersection(keys))
    if any(s['kind']=='character' for key in keys for s in catalog['images'][key]['sources']):
        manifest['characters']=True
    print(f'Processing {len(keys)} images: {len(nearest.intersection(keys))} nearest-neighbor, {len(set(keys)-nearest)} {spec["name"]}',flush=True)
    with ProcessPoolExecutor(max_workers=args.workers,initializer=initialize,initargs=(str(weights),args.denoise,args.threads,args.model,needs_model)) as pool:
        for i,(key,entry) in enumerate(pool.map(upscale,tasks,chunksize=1),1):
            manifest['images'][key]=entry
            if i%25==0 or i==len(tasks):print(f'Upscaled {i}/{len(tasks)} images',flush=True)
    temporary=out/'manifest.tmp'
    temporary.write_text(json.dumps(manifest,separators=(',',':'))+'\n');temporary.replace(out/'manifest.json')
    print(f'Personal HD pack: {out}',flush=True)

if __name__=='__main__':main()
