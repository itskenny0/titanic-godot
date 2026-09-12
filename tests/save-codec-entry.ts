import {GameSession} from '../vendor/dreamrefactory/engine/src/runtime/session';
import {NullAudioSink} from '../vendor/dreamrefactory/engine/src/runtime/audio';
import {createNeutralSaveTemplate,createRestoredSnapshot} from '../engine/restoration/neutral-save';
import {parseRestoredSave} from '../engine/restoration/save-extension';
export function fixture(){const s=new GameSession(()=>null,new NullAudioSink());s.interp.globals.set('mission',2);s.interp.globals.set('a_long_custom_global','retained');return createRestoredSnapshot(s);}
export {createNeutralSaveTemplate,parseRestoredSave};
