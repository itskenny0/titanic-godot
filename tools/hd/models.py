"""Pinned local upscaling models. None of these run on the game device."""
import re
from pathlib import Path

RELEASE = 'https://github.com/xinntao/Real-ESRGAN/releases/download/'
PRESETS = {
    'riven': {
        'name': 'FSDedither Riven', 'scale': 4,
        'files': {'4x-FSDedither-Riven.pth': (
            'https://objectstorage.us-phoenix-1.oraclecloud.com/n/ax6ygfvpvzka/b/open-modeldb-files/o/4x-FSDedither-Riven.pth',
            '712f18baca9bcc09cd30f4ba8a524d66d0960ea72e2fe33f939e190cdc27745d')},
    },
    'compact': {
        'name': 'Real-ESRGAN realesr-general-x4v3 / wdn', 'scale': 4,
        'files': {
            'realesr-general-x4v3.pth': (RELEASE+'v0.2.5.0/realesr-general-x4v3.pth',
                '8dc7edb9ac80ccdc30c3a5dca6616509367f05fbc184ad95b731f05bece96292'),
            'realesr-general-wdn-x4v3.pth': (RELEASE+'v0.2.5.0/realesr-general-wdn-x4v3.pth',
                '1641f8c4464b9f097c9fdda5589273713f67cf59f3d909e0bd688f0cee269dca'),
        },
    },

}


def riven_state(state):
    """Translate the original ESRGAN checkpoint's layer names to BasicSR names."""
    layers = {'model.0': 'conv_first', 'model.1.sub.23': 'conv_body',
              'model.3': 'conv_up1', 'model.6': 'conv_up2',
              'model.8': 'conv_hr', 'model.10': 'conv_last'}
    mapped = {}
    for key, value in state.items():
        match = re.fullmatch(r'model\.1\.sub\.(\d+)\.RDB(\d)\.conv(\d)\.0\.(weight|bias)', key)
        if match:
            block, rdb, conv, param = match.groups()
            target = f'body.{block}.rdb{rdb}.conv{conv}.{param}'
        else:
            layer, param = key.rsplit('.', 1)
            target = layers[layer]+'.'+param
        if target in mapped:
            raise ValueError('Duplicate model parameter: '+target)
        mapped[target] = value
    return mapped


def load_model(preset, weights, denoise):
    import torch
    spec = PRESETS[preset]
    states = [torch.load(Path(weights)/name, map_location='cpu', weights_only=True)
              for name in spec['files']]
    if preset == 'compact':
        from srvgg import SRVGGNetCompact
        model = SRVGGNetCompact(num_in_ch=3, num_out_ch=3, num_feat=64,
                                num_conv=32, upscale=4, act_type='prelu')
        strong, weak = [s['params'] for s in states]
        state = {key: strong[key]*denoise + weak[key]*(1-denoise) for key in strong}
    else:
        from rrdb import RRDBNet
        model = RRDBNet(3, 3, scale=spec['scale'])
        state = states[0]
        state = riven_state(state)
    model.load_state_dict(state, strict=True)
    return model.eval().to(memory_format=torch.channels_last)
