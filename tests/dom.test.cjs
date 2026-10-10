const {test} = require('node:test');
const assert=require('node:assert/strict');
const {parseHTML}=require('linkedom');
const vm=require('node:vm');
const fs=require('node:fs');
const path=require('node:path');

async function mount(entity='contacts',id='c_client') {
 const {window}=parseHTML('<html><body><div id="app"></div></body></html>');
 const document=window.document,errors=[],calls=[],timers=[];
 const contact={id:'c_client',name:'Client One',email:'client@example.com',phone:'+12025550123',_revision:'contact-version'};
 const student={id:'s_client',name:'Student One',applicationStage:'Applying',intake:'January 2027',_revision:'student-version'};
 const flow={contacts:[contact,{...contact,id:'c_other',name:'Other Client'}],students:[student],deals:[],cases:[],documents:[],invoices:[],schools:[],leads:[],agents:[],partners:[],tasks:[],templates:[],ledger:[]};
 const center={role:'manager',organization:[{name:'Test workspace'}],features:{ai:false,smtp:false},records:{contacts:flow.contacts,students:flow.students},mail:[],entries:[],notifications:[]};
 // Linkedom omits several browser form APIs. Supply their standard behavior only
 // in this test harness; browser tests exercise the same code without shims.
 Object.defineProperty(window.HTMLElement.prototype,'elements',{configurable:true,get(){return {namedItem:name=>this.querySelector('[name="'+name+'"]')}}});
 Object.defineProperty(window.HTMLSelectElement.prototype,'value',{configurable:true,get(){const selected=this.querySelector('option[selected]')||this.querySelector('option');return selected?.getAttribute('value')||''},set(value){for(const o of this.querySelectorAll('option')){if(o.getAttribute('value')===String(value))o.setAttribute('selected','');else o.removeAttribute('selected')}}});
 window.HTMLElement.prototype.scrollIntoView=()=>{};
 const sandbox={window,document,location:{href:'http://crm.test/',origin:'http://crm.test',pathname:'/',search:''},localStorage:{getItem:()=>null},URLSearchParams,URL,FormData:class {constructor(form){this.items=[...form.querySelectorAll("[name]")].map(e=>[e.name,e.value])} [Symbol.iterator](){return this.items[Symbol.iterator]()}},Intl,console,CustomEvent:window.CustomEvent,Event:window.Event,MutationObserver:window.MutationObserver,HTMLElement:window.HTMLElement,customElements:window.customElements,navigator:{},setTimeout,clearTimeout,setInterval:(fn,ms)=>{const timer=setInterval(fn,ms);timers.push(timer);return timer},clearInterval};
 sandbox.fetch=async(url,opts={})=>{calls.push({url,opts});let data={status:'ok'};if(url.startsWith('/api/records/'))data={record:contact,editable:true};else if(url==='/api/workflows')data=flow;else if(url==='/api/control-center')data=center;else if(url.startsWith('/api/assistant/jobs'))data={jobs:[]};else if(url==='/api/portal/invitations')data={invitations:[]};else if(url.includes('/conversations'))data={messages:[{channel:'email',direction:'inbound',subject:'Reply',body:'<img src=x onerror=alert(1)>',status:'received'}]};else if(url.startsWith('/api/insights'))data={sources:[{source:'Website',leads:501,converted:100,rate:19.96}],enrollment:[],agents:[],stages:[],nextActions:[],cohort:'All accessible records'};return {status:200,ok:true,json:async()=>data}};
 const context=vm.createContext(sandbox);
 for(const file of ['center.js','components.js'])vm.runInContext(fs.readFileSync(path.join(__dirname,'../public',file),'utf8'),context,{filename:file});
 const element=document.createElement(entity==='workspace'?'crm-workspace':entity==='insights'?'crm-insights':'crm-record-tools');
 if(!['workspace','insights'].includes(entity)){element.setAttribute('entity',entity);element.setAttribute('record-id',id)}
 document.getElementById('app').append(element);
 await new Promise(resolve=>setTimeout(resolve,30));
 return {window,document,element,calls,flow,errors,cleanup(){element.remove();for(const timer of timers)clearInterval(timer)}};
}

test('record email tools select the current client and render incoming content as text',async()=>{
 const app=await mount();try{const root=app.element.shadowRoot;assert.equal(root.querySelector('[name=contactId]').value,'c_client');assert.equal(root.querySelector('[name=recipient]').value,'client@example.com');assert.equal(root.querySelector('[name=contactId]').querySelectorAll('option').length,2);const button=[...root.querySelectorAll('nav button')].find(b=>b.textContent==='Conversation');button.onclick();await new Promise(resolve=>setTimeout(resolve,10));assert.ok(root.querySelector(".tools").textContent.includes('<img src=x'));assert.equal(root.querySelectorAll('img').length,0)}finally{app.cleanup()}
});
test('student tools hydrate the existing application and intake',async()=>{
 const app=await mount('students','s_client');try{const root=app.element.shadowRoot;assert.equal(root.querySelector('[name=intake]').value,'January 2027');assert.equal(root.querySelector('[name=applicationStage]').value,'Applying');assert.ok(root.querySelector(".tools").textContent.includes('Create a student case'));assert.ok(!root.querySelector(".tools").textContent.includes('Lead qualification'))}finally{app.cleanup()}
});
test('native workspace offers recoverable deletion and unmatched-message workflows',async()=>{
 const app=await mount('workspace');try{assert.equal(app.document.querySelectorAll('iframe').length,0);const labels=[...app.element.shadowRoot.querySelectorAll('nav button')].map(b=>b.textContent);assert.ok(labels.includes('Trash'));assert.ok(labels.includes('Unmatched inbox'));assert.ok(labels.includes('Client portal'))}finally{app.cleanup()}
});
test('performance reports display full server aggregates',async()=>{
 const app=await mount('insights');try{assert.ok(app.element.shadowRoot.querySelector(".tools").textContent.includes('501'));assert.ok(app.calls.some(c=>c.url.startsWith('/api/insights')));assert.ok(app.element.shadowRoot.querySelector(".tools").textContent.includes('Lead conversion by source'))}finally{app.cleanup()}
});

test('record version headers stay tied to the form snapshot during background refresh',async()=>{
 const {window}=parseHTML('<html><body></body></html>');let version='version-one';
 class FakeXHR{
  constructor(){this.headers={};this.listeners={};this.status=200}
  open(method,url){this.method=method;this.url=url}
  setRequestHeader(key,value){this.headers[key]=value}
  addEventListener(key,callback){this.listeners[key]=callback}
  send(){this.responseText=JSON.stringify({contact:{id:'client',_revision:version}});this.readyState=4;this.listeners.readystatechange?.();this.listeners.load?.()}
 }
 const context=vm.createContext({window,document:window.document,location:{href:'http://crm.test/',origin:'http://crm.test'},URL,CustomEvent:window.CustomEvent,XMLHttpRequest:FakeXHR,MutationObserver:window.MutationObserver,Map,JSON});
 vm.runInContext(fs.readFileSync(path.join(__dirname,'../public/record-versions.js'),'utf8'),context);
 const get=()=>{const request=new FakeXHR();request.open('GET','/api/contacts/client');request.send()};
 get();const dialog=window.document.createElement('div');dialog.className='modal-panel';window.document.body.append(dialog);await new Promise(resolve=>setTimeout(resolve,0));
 version='version-two';get();const old=new FakeXHR();old.open('PUT','/api/contacts/client');old.send('{}');assert.equal(old.headers['If-Match'],'version-one');
 dialog.remove();await new Promise(resolve=>setTimeout(resolve,0));const fresh=new FakeXHR();fresh.open('DELETE','/api/contacts/client');fresh.send();assert.equal(fresh.headers['If-Match'],'version-two');
});
