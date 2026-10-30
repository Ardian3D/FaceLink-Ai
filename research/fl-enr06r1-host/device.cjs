'use strict';
const cp = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const ROOT = path.resolve(__dirname, '../..');
const OUT = path.join(ROOT, 'research/fl-enr02a-host/evidence-local/enr06-r1/run4');
const ADB = process.env.ADB_PATH?.trim() || 'adb';
const SERIAL = process.env.ANDROID_SERIAL?.trim();
if (!SERIAL) throw Error('ANDROID_SERIAL is required; select the authorized staging device explicitly.');
function adb(args, binary = false) { return cp.execFileSync(ADB, ['-s', SERIAL, ...args], { encoding: binary ? undefined : 'utf8', timeout: 20000, maxBuffer: 160 * 1024 * 1024 }); }
function ui() {
  const result = adb(['shell', 'uiautomator', 'runtest', '/data/local/tmp/facelink-enr06r1.jar', '-c', 'DumpUI']);
  if (!result.includes('OK (1 test)') || /No accessibility root|unexpected exception/.test(result)) throw Error('UI tooling failed');
  const xml = adb(['exec-out', 'cat', '/data/local/tmp/facelink-enr06r1.xml']);
  const decode = s => s.replaceAll('&amp;', '&').replaceAll('&quot;', '"').replaceAll('&lt;', '<').replaceAll('&gt;', '>');
  return [...xml.matchAll(/<node [^>]*>/g)].map(m => {
    const attr = name => decode(new RegExp(name + '="([^"]*)"').exec(m[0])?.[1] || '');
    return { text: attr('text'), desc: attr('content-desc'), cls: attr('class'), clickable: attr('clickable') === 'true', enabled: attr('enabled') === 'true', bounds: attr('bounds').match(/-?\d+/g)?.map(Number) };
  });
}
function at(x,y) { adb(['shell','input','tap',String(x),String(y)]); }
function tapNode(n) { if(!n?.bounds) throw Error('UI node missing'); const [x,y,a,b]=n.bounds; if(a<=x||b<=y) throw Error('UI node outside viewport'); at(Math.round((x+a)/2),Math.round((y+b)/2)); }
function tap(q) { const ns=ui(); const exact=ns.filter(n=>n.text===q||n.desc===q); tapNode(exact.find(n=>n.clickable)||exact[0]||ns.find(n=>n.clickable&&(n.text.includes(q)||n.desc.includes(q)))||ns.find(n=>n.text.includes(q)||n.desc.includes(q))); }
const pause = ms => new Promise(resolve => setTimeout(resolve,ms));
async function waitUI(q,seconds=20) { const end=Date.now()+seconds*1000; while(Date.now()<end) { const ns=ui(); if(ns.some(n=>n.text.includes(q)||n.desc.includes(q))) return ns; await pause(400); } throw Error('UI deadline: '+q); }
function shot(name) { fs.writeFileSync(path.join(OUT,name+'.png'),adb(['exec-out','screencap','-p'],true),{flag:'wx'}); }
function view() { return ui().filter(n=>n.text||n.desc).map(n=>({...n,text:/^\d{8}$/.test(n.text)?'[OTP omitted]':n.text})); }
let packageName = 'ai.facelink.enr02host';
function setPackage(value) {
  if (!['ai.facelink.enr02host', 'ai.facelink.enr06admin'].includes(value)) throw Error('Not an authorized staging package');
  packageName = value;
}
function launch() { return adb(['shell','am','start','-a','android.intent.action.VIEW','-d','exp+facelink-enr02a://expo-development-client/?url=http%3A%2F%2F127.0.0.1%3A8083',packageName]); }
async function control(command) {
  const file=path.join(OUT,'acks.jsonl'); const prior=fs.existsSync(file)?fs.readFileSync(file,'utf8').split('\n').filter(Boolean).length:0;
  fs.appendFileSync(path.join(OUT,'commands.jsonl'),JSON.stringify(command)+'\n');
  const end=Date.now()+25000;
  while(Date.now()<end) {
    const lines=fs.existsSync(file)?fs.readFileSync(file,'utf8').split('\n').filter(Boolean):[];
    if(lines.length>prior) { const ack=JSON.parse(lines[prior]); if(ack.error||ack.data?.ok===false)throw Error('Host control rejected'); return ack.data; }
    await pause(100);
  }
  throw Error('Host control deadline');
}
async function enterOTP(startup) {
  await control({command:'otp.latest'}); await pause(700);
  let code=fs.readFileSync(startup.otp_file,'utf8').trim(); if(!/^\d{8}$/.test(code))throw Error('Invalid OTP shape');
  for(let i=0;i<10;i++)adb(['shell','input','keyevent','67']);
  for(const d of code) { adb(['shell','input','keyevent',String(Number(d)+7)]); await pause(110); }
  code=''; adb(['shell','input','keyevent','66']);
}
function settings() { const n=ui().find(n=>n.desc.startsWith('Profil ')&&n.clickable); if(!n)throw Error('Profile missing'); at(n.bounds[0]+25,Math.round((n.bounds[1]+n.bounds[3])/2)); }
module.exports={ROOT,OUT,adb,ui,at,tapNode,tap,pause,waitUI,shot,view,launch,setPackage,control,enterOTP,settings};
