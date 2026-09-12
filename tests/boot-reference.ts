import {readFileSync,writeFileSync} from 'node:fs';
import {readBootPlan} from '../vendor/dreamrefactory/engine/src/runtime/bootplan';
const records=process.argv.slice(3).map(path=>({path,plan:readBootPlan(new Uint8Array(readFileSync(path)))}));
writeFileSync(process.argv[2],JSON.stringify(records));console.log('Compared boot plans: '+records.length);
