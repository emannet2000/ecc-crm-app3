const {test, before, after} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const {spawn} = require('node:child_process');
const puppeteer = require('puppeteer');
let browser;
before(async () => {
  const local = path.join(os.homedir(), '.cache/ms-playwright/chromium-1234/chrome-linux64/chrome');
  const executablePath = process.env.CHROME_PATH || (fs.existsSync(local) ? local : await puppeteer.executablePath());
  browser = await puppeteer.launch({executablePath, headless:true, pipe:true, args:['--no-sandbox','--disable-dev-shm-usage']});
});
after(async () => { await browser?.close(); });

async function fixture(view = 'contacts') {
  const page = await browser.newPage(), calls=[], errors=[];
  const contact = {id:'c_client',name:'Client One',email:'client@example.com',phone:'+12025550123',orgId:'org_a',ownerId:'u_manager',visibility:'organization',_revision:'old-version'};
  const student = {id:'s_client',name:'Student One',schoolId:'school',applicationStage:'Applying',intake:'January 2027',_revision:'student-version'};
  const flow={contacts:[contact,{...contact,id:'c_other',name:'Other Client'}],students:[student],deals:[],cases:[{id:'case_client',clientId:contact.id,studentId:student.id,caseNumber:'ECC-001',currentStage:'Documents Pending'}],documents:[{id:'doc_client',caseId:'case_client',caseNumber:'ECC-001',docName:'Passport',latestVersion:1,status:'Received'}],invoices:[],leads:[],agents:[],partners:[],schools:[],tasks:[],templates:[],ledger:[]};
  const center={role:'manager',user:{id:'u_manager'},organization:[{name:'Demo workspace',timezone:'UTC'}],records:{contacts:flow.contacts.map(c=>({id:c.id,label:c.name})),students:[{id:student.id,label:student.name}]},features:{smtp:false,ai:false},entries:[],mail:[],notifications:[]};
  page.on('pageerror',e=>errors.push(e.message));
  await page.setRequestInterception(true);
  page.on('request', async req=> {
    try {
      const url=new URL(req.url()); calls.push({query:url.search,path:url.pathname,method:req.method(),headers:req.headers(),body:req.postData()});
      let data,contentType='application/json',status=200;
      if(url.pathname.startsWith('/public/')) {data=fs.readFileSync(path.join(__dirname,'..',url.pathname));contentType=url.pathname.endsWith('.css')?'text/css':'application/javascript';}
      else if(url.pathname==='/portal') {data=fs.readFileSync(path.join(__dirname,'../public/portal.html'));contentType='text/html';}
      else if(url.pathname==='/') {contentType='text/html';data=`<!doctype html><html><head><meta charset="utf-8"></head><body><div id="app">${view==='picker'?'<crm-record-picker entity="contacts" value="c_client" label="Client"></crm-record-picker>':view==='ai'?'<crm-ai-chat></crm-ai-chat>':view==='workspace'?'<crm-workspace></crm-workspace>':view==='insights'?'<crm-insights></crm-insights>':view==='daily'?'<crm-my-work></crm-my-work>':view==='journey'?'<crm-student-journey record-id="s_client"></crm-student-journey>':view==='quality'?'<crm-form-quality entity="students" studentcode="DUP"></crm-form-quality>':view==='connections'?'<crm-integration-health></crm-integration-health>':`<crm-record-tools entity="${view}" record-id="${view==='students'?'s_client':'c_client'}"></crm-record-tools>`}</div><script src="/public/pickers.js"></script><script src="/public/record-versions.js"></script><script src="/public/ai.js"></script><script src="/public/center.js"></script><script src="/public/components.js"></script></body></html>`;}
      else if(view==='picker'&&url.pathname==='/api/contacts/c_client') data={contact};
      else if(view==='picker'&&url.pathname==='/api/contacts') {
        let rows=Array.from({length:240},(_,i)=>({id:'pick-'+i,name:'Searchable client '+i,email:'client'+i+'@example.com'}));
        const q=url.searchParams.get('q')||'';
        if(q==='Slow'){await new Promise(resolve=>setTimeout(resolve,700));rows=[{id:'stale',name:'Slow result'}]}
        else rows=rows.filter(row=>row.name.includes(q));
        const offset=Number(url.searchParams.get('offset')||0);data={contacts:rows.slice(offset,offset+20),total:rows.length};
      }
      else if(url.pathname.startsWith('/api/records/')) data={record:contact,editable:true};
      else if(url.pathname==='/api/my-work') data={today:'2026-10-10',total:60,counts:{applications:60,documents:1,followups:0,deadlines:1,tasks:0},items:[{id:'case_client',entity:'case',name:'ECC-001',reason:'Required documents need review',action:'Review passport',priority:'high',deadline:'2026-10-12',route:'/cases/case_client'}]};
      else if(url.pathname==='/api/students/s_client/journey') data={student,applications:flow.cases.map(c=>({...c,assignedOfficer:'Adviser',nextAction:'Request passport'})),documents:flow.documents.map(d=>({...d,required:true})),invoices:[{id:'invoice-1',invoiceNumber:'INV-1',currency:'PHP',balance:50,amountReceived:20}],payments:[{invoiceId:'invoice-1',reference:'Receipt-1',amount:20,currency:'PHP'}],messages:[{caseId:'case_client',subject:'Portal response',body:'<img src=x onerror=alert(1)>',channel:'portal',direction:'inbound',createdAt:'2026-10-10'}],messageTotal:1,required:1,verified:0};
      else if(url.pathname==='/api/form-quality') data={findings:JSON.parse(req.postData()).studentCode==='DUP'?[{title:'Possible duplicate student',detail:'Review Student One.',route:'/students/s_client'}]:[]};
      else if(url.pathname==='/api/integrations/status') data={canTest:true,integrations:[{id:'email',name:'Email',state:'needs_configuration',message:'Configure SMTP.'},{id:'payments',name:'Payments',state:'configured',message:'Run a connection check.'}]};
      else if(url.pathname==='/api/admin/integrations/payments/test') data={id:'payments',name:'Payments',state:'failing',message:'Verify credentials and provider access.',checkedAt:'2026-10-10T08:00:00Z'};
      else if(url.pathname==='/api/control-center') data=center;
      else if(url.pathname==='/api/workflows') data=flow;
      else if(url.pathname==='/api/insights') data={cohort:'All accessible records',sources:[{source:'Website',leads:501,converted:100,rate:19.96}],enrollment:[],agents:[],stages:[],nextActions:[{record:'ECC-001',action:'Request passport',deadline:'2026-10-01',url:'/cases/case_client'}]};
      else if(url.pathname==='/api/assistant/capabilities') data={enabled:true,actions:true};
      else if(url.pathname==='/api/assistant/threads') data={threads:[]};
      else if(url.pathname==='/api/assistant/chat') data={reply:'<img src=x onerror=alert(1)>',threadId:'thread-1',version:1,sources:[{title:'Client One',url:'/contacts/c_client'}],actions:[{id:'action-1',kind:'update_record',label:'Client One',status:'pending',fields:{name:'Updated client'},before:{name:'Client One'}}]};
      else if(url.pathname==='/api/assistant/jobs') data={jobs:[]};
      else if(url.pathname==='/api/portal/invitations') data={invitations:[],url:'http://crm.test/portal#invite=private',message:'Share this invitation'};
      else if(url.pathname==='/api/portal/me') data={email:contact.email,student:{name:'Client One',applicationStage:'Applying'},cases:[{id:'case_client',caseNumber:'ECC-001',stage:'Documents Pending',nextAction:'Upload passport'}],documents:[{id:'doc_client',name:'Passport',required:true,status:'Requested',caseNumber:'ECC-001',latestVersion:0}],invoices:[],messages:[]};
      else if(url.pathname==='/api/data-quality') data={total:1,totalAll:1,page:1,pages:1,findings:[{severity:'error',title:'Deadline is overdue',detail:'ECC-001 passed its deadline.',entity:'cases',recordId:'case_client',route:'/cases/case_client'}]};
      else if(url.pathname==='/api/follow-up-rules') data={rules:[{id:'rule-1',data:JSON.stringify({name:'Stale case',entity:'cases',stage:'',inactiveDays:5,taskTitle:'Follow {{record}}',description:'Check in',enabled:false}),count:1}]};
      else if(url.pathname.endsWith('/rule-1/preview')) data={count:1,records:[{name:'ECC-001',lastTouch:'2026-09-20'}],previewToken:'preview-token',previewExpiresAt:'2026-10-07T09:05:00Z'};
      else if(url.pathname==='/api/follow-up-rules/rule-1'&&req.method()==='PATCH') {status=409;data={error:'Records changed since the preview. Preview the rule again before enabling it.'};}
      else if(url.pathname.includes('/conversations')) data={messages:[{channel:'email',direction:'inbound',subject:'Reply',body:'<img src=x onerror=alert(1)>',status:'received'}]};
      else if(url.pathname==='/api/contacts/c_client'&&req.method()==='GET') data={contact};
      else data={status:'ok'};
      await req.respond({status,contentType,body:typeof data==='object'&&!Buffer.isBuffer(data)?JSON.stringify(data):data});
    }catch(e){errors.push(e.message);await req.abort().catch(()=>{});}
  });
  await page.goto('http://crm.test/'+(view==='portal'?'portal#invite=single-use-invitation':''),{waitUntil:'networkidle0'});
  return {page,calls,errors,flow,center,contact};
}

test('contact tools stay on the record, default to the correct client and show safe conversation text', async () => {
  const {page,errors}=await fixture();
  try {
    await page.waitForFunction(()=>document.querySelector('crm-record-tools')?.shadowRoot.querySelector('form'));
    assert.equal(await page.$eval('body',el=>el.querySelectorAll('iframe').length),0);
    const selected=await page.evaluate(()=>document.querySelector('crm-record-tools').shadowRoot.querySelector('[name=contactId]').value);
    assert.equal(selected,'c_client');
    await page.evaluate(()=>[...document.querySelector('crm-record-tools').shadowRoot.querySelectorAll('nav button')].find(b=>b.textContent==='Conversation').click());
    await page.waitForFunction(()=>document.querySelector('crm-record-tools').shadowRoot.textContent.includes('<img src=x'));
    assert.equal(await page.evaluate(()=>document.querySelector('crm-record-tools').shadowRoot.querySelectorAll('img').length),0);
    assert.deepEqual(errors,[]);
  }finally{await page.close()}
});

test('student workflow hydrates existing values and sends its record version with edits', async()=>{
 const {page,calls,errors}=await fixture('students');
 try{
  await page.waitForFunction(()=>document.querySelector('crm-record-tools').shadowRoot.querySelector('[name=intake]')?.value==='January 2027');
  await page.evaluate(()=>{const root=document.querySelector('crm-record-tools').shadowRoot;root.querySelector('[name=intake]').value='September 2027';root.querySelector('[name=intake]').closest('form').requestSubmit()});
  await page.waitForFunction(()=>document.querySelector('crm-record-tools').shadowRoot.querySelector('#notice').textContent.includes('Saved'));
  const mutation=calls.find(r=>r.path==='/api/workflows/student');assert.ok(mutation);assert.equal(mutation.headers['if-match'],'student-version');assert.equal(JSON.parse(mutation.body).intake,'September 2027');assert.deepEqual(errors,[]);
 }finally{await page.close()}
});

test('native workspace renders operational tabs without an iframe',async()=>{
 const {page,errors}=await fixture('workspace');try{await page.waitForFunction(()=>document.querySelector('crm-workspace').shadowRoot.querySelectorAll('nav button').length>5);assert.equal(await page.$eval('body',el=>el.querySelectorAll('iframe').length),0);assert.ok(await page.evaluate(()=>document.querySelector('crm-workspace').shadowRoot.textContent.includes('Trash')));assert.deepEqual(errors,[])}finally{await page.close()}
});

test('report date filters request server aggregates and retain full record counts',async()=>{
 const {page,calls,errors}=await fixture('insights');try{await page.waitForFunction(()=>document.querySelector('crm-insights').shadowRoot.textContent.includes('501'));await page.evaluate(()=>{const root=document.querySelector('crm-insights').shadowRoot;root.querySelector('[name=from]').value='2026-01-01';root.querySelector('form').requestSubmit()});await page.waitForFunction(()=>document.querySelector('crm-insights').shadowRoot.textContent.includes('501'));assert.ok(calls.filter(r=>r.path==='/api/insights').length>=2);assert.deepEqual(errors,[])}finally{await page.close()}
});

test('portal removes invitation credentials from the URL and supports document submission',async()=>{
 const {page,calls,errors}=await fixture('portal');try{await page.waitForSelector('input[type=file]');assert.equal(new URL(page.url()).hash,'');assert.equal(JSON.parse(calls.find(r=>r.path==='/api/portal/session').body).token,'single-use-invitation');assert.ok(await page.$eval('body',el=>el.textContent.includes('Journey progress')&&el.textContent.includes('0 of 1 required documents verified')&&el.textContent.includes('Next step: Upload passport')));const file=path.join(os.tmpdir(),'ecc-portal-upload.txt');fs.writeFileSync(file,'Client document');const input=await page.$('input[type=file]');await input.uploadFile(file);await page.evaluate(()=>document.querySelector('input[type=file]').closest('form').requestSubmit());await page.waitForFunction(()=>document.getElementById('notice').textContent.includes('uploaded'));assert.ok(calls.some(r=>r.path==='/api/portal/documents/doc_client/files'&&r.method==='POST'));assert.deepEqual(errors,[]);fs.unlinkSync(file)}finally{await page.close()}
});

test('data-quality scan filters findings and navigates to the affected case',async()=>{
 const {page,calls,errors}=await fixture('workspace');try{await page.waitForFunction(()=>document.querySelector('crm-workspace').shadowRoot.querySelectorAll('nav button').length);await page.evaluate(()=>[...document.querySelector('crm-workspace').shadowRoot.querySelectorAll('nav button')].find(b=>b.textContent==='Data & views').click());await page.waitForFunction(()=>document.querySelector('crm-workspace').shadowRoot.textContent.includes('Run data quality scan'));await page.evaluate(()=>[...document.querySelector('crm-workspace').shadowRoot.querySelectorAll('button')].find(b=>b.textContent==='Run data quality scan').click());await page.waitForFunction(()=>document.querySelector('crm-workspace').shadowRoot.textContent.includes('Deadline is overdue'));const link=await page.$eval('crm-workspace',el=>[...el.shadowRoot.querySelectorAll('a')].find(a=>a.textContent==='Review cases')?.getAttribute('href'));assert.equal(link,'/cases/case_client');assert.ok(calls.some(c=>c.path==='/api/data-quality'&&c.path.includes('data-quality')));assert.deepEqual(errors,[])}finally{await page.close()}
});

test('follow-up rule preview surfaces a stale-preview rejection',async()=>{
 const {page,calls,errors}=await fixture('workspace');try{page.on('dialog',d=>d.accept());await page.waitForFunction(()=>document.querySelector('crm-workspace').shadowRoot.querySelectorAll('nav button').length);await page.evaluate(()=>[...document.querySelector('crm-workspace').shadowRoot.querySelectorAll('nav button')].find(b=>b.textContent==='Workflows').click());await page.waitForFunction(()=>document.querySelector('crm-workspace').shadowRoot.textContent.includes('Preview & enable'));await page.evaluate(()=>[...document.querySelector('crm-workspace').shadowRoot.querySelectorAll('button')].find(b=>b.textContent==='Preview & enable').click());await page.waitForFunction(()=>document.querySelector('crm-workspace').shadowRoot.textContent.includes('Records changed since the preview'));assert.ok(calls.some(c=>c.path==='/api/follow-up-rules/rule-1/preview'));assert.deepEqual(errors,[])}finally{await page.close()}
});

test('a background refresh cannot replace the version attached to an open edit dialog',async()=>{
 const {page,calls,contact,errors}=await fixture('workspace');try{assert.deepEqual(errors,[]);assert.equal(await page.evaluate(()=>XMLHttpRequest.prototype.open.toString().includes('crmURL')),true);await page.evaluate(async()=>{const get=()=>new Promise(resolve=>{const xhr=new XMLHttpRequest();xhr.open('GET','/api/contacts/c_client');xhr.onload=resolve;xhr.send()});await get();document.body.append(Object.assign(document.createElement('div'),{className:'modal-panel'}));await new Promise(resolve=>setTimeout(resolve,0))});contact._revision='new-version';await page.evaluate(async()=>{await new Promise(resolve=>{const xhr=new XMLHttpRequest();xhr.open('GET','/api/contacts/c_client');xhr.onload=resolve;xhr.send()});await new Promise(resolve=>{const xhr=new XMLHttpRequest();xhr.open('PUT','/api/contacts/c_client');xhr.onload=resolve;xhr.send('{}')})});assert.equal(calls.find(r=>r.path==='/api/contacts/c_client'&&r.method==='PUT').headers['if-match'],'old-version',JSON.stringify(calls.filter(r=>r.path==='/api/contacts/c_client')))}finally{await page.close()}
});

test('live app login, native record workflows, reporting and mobile layout', {skip:process.env.ECC_BROWSER_LIVE!=='true'},async()=>{
 const directory=fs.mkdtempSync(path.join(os.tmpdir(),'ecc-crm-browser-')),port='17001';
 const server=spawn(path.join(__dirname,'../build/ecc-crm'),[],{cwd:path.join(__dirname,'..'),env:{...process.env,PORT:port,DATABASE_PATH:path.join(directory,'crm.sqlite3'),JWT_SECRET:'browser-test-secret-only',APP_URL:'http://localhost:'+port,ALLOW_SIGNUP:'false',GRAPH_ORG_ID:'',WHATSAPP_ORG_ID:''},stdio:'pipe'});
 const page=await browser.newPage();let output='';server.stderr.on('data',data=>output+=data);server.stdout.on('data',data=>output+=data);
 try{
  for(let i=0;i<100;i++){try{const response=await fetch('http://localhost:'+port+'/api/health');if(response.ok)break}catch{}await new Promise(resolve=>setTimeout(resolve,100))}
  await page.goto('http://localhost:'+port,{waitUntil:'domcontentloaded'});
  await page.waitForSelector('input[type=email]');await page.type('input[type=email]','demo@northwind.dev');await page.type('input[type=password]','Demo1234');await page.click('button[type=submit]');
  await page.waitForSelector('.app-shell');await page.goto('http://localhost:'+port+'/contacts/c_1',{waitUntil:'domcontentloaded'});
  await page.waitForFunction(()=>document.querySelector('crm-record-tools').shadowRoot.querySelector('form'));
  assert.equal(await page.$eval('body',el=>el.querySelectorAll('iframe').length),0);
  for (const width of [320,390,768]) {
    await page.setViewport({width,height:844,isMobile:true,hasTouch:true});
    await page.goto('http://localhost:'+port+'/contacts',{waitUntil:'domcontentloaded'});
    await page.waitForSelector('table[data-mobile-cards]');
    assert.ok(await page.$eval('body',el=>el.scrollWidth<=window.innerWidth+2), 'contacts overflow at '+width);
    assert.equal(await page.$eval('.mobile-nav',el=>getComputedStyle(el).display),'grid');
    if(width<=600) assert.equal(await page.$eval('table.data-table',el=>getComputedStyle(el).minWidth),'0px');
    await page.click('.topbar__menu-btn');
    await page.waitForSelector('.sidebar.is-open');
    await page.waitForFunction(()=>document.querySelector('.app-main').inert);
    assert.equal(await page.$eval('.app-main',el=>el.inert),true);
    await page.waitForFunction(()=>document.activeElement?.classList.contains('sidebar__close'));await page.click('.sidebar__close');
    await page.waitForFunction(()=>document.querySelector('.sidebar').inert);
    await page.evaluate(()=>[...document.querySelectorAll('.mobile-nav button')].find(b=>b.textContent.trim()==='Tasks').click());
    await page.waitForFunction(()=>location.pathname==='/tasks');
    assert.equal(await page.$eval('.app-main',el=>el.inert),false);
  }
  await page.setViewport({width:390,height:844,isMobile:true,hasTouch:true});
  await page.goto('http://localhost:'+port+'/contacts',{waitUntil:'domcontentloaded'});
  await page.waitForFunction(()=>[...document.querySelectorAll('button')].some(b=>b.textContent.trim()==='Add contact'));await page.evaluate(()=>[...document.querySelectorAll('button')].find(b=>b.textContent.trim()==='Add contact').click());
  await page.waitForSelector('.modal');
  assert.ok(await page.$eval('.modal',el=>el.getBoundingClientRect().width<=window.innerWidth));
  assert.equal(await page.$eval('.modal input',el=>getComputedStyle(el).fontSize),'16px');
  await page.click('.modal__close');
  await page.goto('http://localhost:'+port+'/administration',{waitUntil:'domcontentloaded'});
  await page.waitForFunction(()=>document.querySelector('crm-administration')?.shadowRoot.querySelector('table'));
  await page.evaluate(()=>[...document.querySelector('crm-administration').shadowRoot.querySelectorAll('button')].find(b=>b.textContent==='Add user').click());
  await page.evaluate(()=>{const root=document.querySelector('crm-administration').shadowRoot;root.querySelector('[name=name]').value='Browser Viewer';root.querySelector('[name=email]').value='browser-viewer@example.com';root.querySelector('[name=role]').value='viewer';root.querySelector('[name=password]').value='BrowserViewerPassword!';root.querySelector('form').requestSubmit()});
  await page.waitForFunction(()=>document.querySelector('crm-administration').shadowRoot.textContent.includes('browser-viewer@example.com'));
  await page.evaluate(()=>[...document.querySelector('crm-administration').shadowRoot.querySelectorAll('nav button')].find(b=>b.textContent==='Access levels').click());
  assert.ok(await page.evaluate(()=>document.querySelector('crm-administration').shadowRoot.textContent.includes('Read only')));
  await page.setViewport({width:390,height:844});assert.ok(await page.$eval('body',el=>el.scrollWidth<=window.innerWidth+2));
  await page.goto('http://localhost:'+port+'/reports',{waitUntil:'domcontentloaded'});await page.waitForFunction(()=>document.querySelector('crm-insights').shadowRoot.textContent.includes('Lead conversion'));
  await page.setViewport({width:390,height:844});assert.ok(await page.$eval('body',el=>el.scrollWidth<=window.innerWidth+2));
 }catch(error){fs.mkdirSync(path.join(__dirname,'artifacts'),{recursive:true});await page.screenshot({path:path.join(__dirname,'artifacts/live-failure.png'),fullPage:true});throw new Error(error.message+'\nUI: '+await page.evaluate(()=>JSON.stringify({url:location.pathname,width:innerWidth,mobile:matchMedia('(max-width: 900px)').matches,mainInert:document.querySelector('.app-main')?.inert,mainAttr:document.querySelector('.app-main')?.hasAttribute('inert'),sideInert:document.querySelector('.sidebar')?.inert,active:document.activeElement?.outerHTML?.slice(0,200)}))+'\n'+output)}finally{await page.close();server.kill('SIGTERM');await new Promise(resolve=>{if(server.exitCode!==null)return resolve();server.once('exit',resolve)});fs.rmSync(directory,{recursive:true,force:true})}
});

test('daily work renders full counts without AI and pages the assignment queue',async()=>{
 const {page,calls,errors}=await fixture('daily');try{
  await page.waitForFunction(()=>document.querySelector('crm-my-work').shadowRoot.textContent.includes('60 work items'));
  assert.ok(await page.evaluate(()=>document.querySelector('crm-my-work').shadowRoot.querySelector('a').getAttribute('href')==='/cases/case_client'));
  await page.evaluate(()=>[...document.querySelector('crm-my-work').shadowRoot.querySelectorAll('button')].find(b=>b.textContent==='Next').click());
  await page.waitForFunction(()=>document.querySelector('crm-my-work').shadowRoot.textContent.includes('26–50 of 60')).catch(async error=>{throw new Error(error.message+' '+await page.evaluate(()=>document.querySelector('crm-my-work').shadowRoot.textContent))});
  assert.equal(calls.filter(c=>c.path.includes('/assistant')).length,0);assert.deepEqual(errors,[]);
 }finally{await page.close()}
});
test('student journey keeps owner, next action, documents and currency-specific payments together',async()=>{
 const {page,errors}=await fixture('journey');try{
  await page.waitForFunction(()=>document.querySelector('crm-student-journey').shadowRoot.textContent.includes('Owner: Adviser'));
  const result=await page.evaluate(()=>{const root=document.querySelector('crm-student-journey').shadowRoot;return {text:root.textContent,images:root.querySelectorAll('img').length,sections:root.querySelectorAll('details').length}});
  assert.ok(result.text.includes('Request passport')&&result.text.includes('PHP')&&result.text.includes('Receipt-1')&&result.text.includes('Portal response'));assert.equal(result.images,0);assert.equal(result.sections,4);assert.deepEqual(errors,[]);
 }finally{await page.close()}
});
test('inline duplicate warnings clear after input changes and keep review links safe',async()=>{
 const {page,errors}=await fixture('quality');try{
  await page.waitForFunction(()=>document.querySelector('crm-form-quality').shadowRoot.textContent.includes('Possible duplicate student'));
  assert.equal(await page.evaluate(()=>document.querySelector('crm-form-quality').shadowRoot.querySelector('a').getAttribute('rel')),'noopener');
  await page.evaluate(()=>document.querySelector('crm-form-quality').setAttribute('studentcode','UNIQUE'));
  await page.waitForFunction(()=>document.querySelector('crm-form-quality').shadowRoot.querySelector('.quality-hints').textContent==='');assert.deepEqual(errors,[]);
 }finally{await page.close()}
});
test('integration checks distinguish setup from provider failure and expose retry guidance',async()=>{
 const {page,calls,errors}=await fixture('connections');try{
  await page.waitForFunction(()=>document.querySelector('crm-integration-health').shadowRoot.querySelectorAll('.card').length===2);
  assert.equal(await page.evaluate(()=>document.querySelector('crm-integration-health').shadowRoot.querySelector('.card button').disabled),true);
  await page.evaluate(()=>document.querySelector('crm-integration-health').shadowRoot.querySelectorAll('.card button')[1].click());
  await page.waitForFunction(()=>document.querySelector('crm-integration-health').shadowRoot.textContent.includes('Verify credentials'));
  assert.ok(calls.some(c=>c.path==='/api/admin/integrations/payments/test'&&c.method==='POST'));assert.deepEqual(errors,[]);
 }finally{await page.close()}
});


test('AI proposals remain pending until staff explicitly confirm them',async()=>{
 const {page,calls,errors}=await fixture('ai');try{
  await page.evaluate(()=>document.querySelector('crm-ai-chat').shadowRoot.querySelector('button').click());
  await page.waitForFunction(()=>document.querySelector('crm-ai-chat').shadowRoot.querySelector('form button')?.disabled===false);
  await page.evaluate(()=>{const root=document.querySelector('crm-ai-chat').shadowRoot;root.querySelector('textarea').value='Suggest an update';root.querySelector('form').requestSubmit()});
  await page.waitForFunction(()=>document.querySelector('crm-ai-chat').shadowRoot.textContent.includes('Proposed — not executed.'));
  assert.equal(await page.evaluate(()=>document.querySelector('crm-ai-chat').shadowRoot.querySelectorAll('img').length),0);
  assert.ok(!calls.some(c=>c.path.endsWith('/confirm')));
  await page.evaluate(()=>[...document.querySelector('crm-ai-chat').shadowRoot.querySelectorAll('button')].find(b=>b.textContent==='Confirm action').click());
  await page.waitForFunction(()=>document.querySelector('crm-ai-chat').shadowRoot.textContent.includes('Confirmed successfully.'));
  assert.equal(calls.filter(c=>c.path.endsWith('/confirm')).length,1);assert.deepEqual(errors,[]);
 }finally{await page.close()}
});

 test('record picker searches beyond loaded pages, retains selection and ignores stale responses',async()=>{
 const {page,errors}=await fixture('picker');
 try {
  await page.waitForFunction(()=>document.querySelector('crm-record-picker').shadowRoot.textContent.includes('Client One'));
  const search=await page.$('crm-record-picker >>> input[type=search]');
  await search.focus();
  await page.waitForFunction(()=>document.querySelector('crm-record-picker').shadowRoot.textContent.includes('of 240'));
  await page.evaluate(()=>[...document.querySelector('crm-record-picker').shadowRoot.querySelectorAll('.paging button')].find(b=>b.textContent==='Next').click());
  await page.waitForFunction(()=>document.querySelector('crm-record-picker').shadowRoot.querySelector('.results button')?.textContent.includes('pick-20'),{timeout:5000});
  await page.evaluate(()=>{const search=document.querySelector('crm-record-picker').shadowRoot.querySelector('input');search.value='Slow';search.dispatchEvent(new Event('input'))});
  await new Promise(resolve=>setTimeout(resolve,350));
  await page.evaluate(()=>{const search=document.querySelector('crm-record-picker').shadowRoot.querySelector('input');search.value='Searchable client 239';search.dispatchEvent(new Event('input'))});
  await page.waitForFunction(()=>document.querySelector('crm-record-picker').shadowRoot.querySelector('.results button')?.textContent.includes('pick-239'));
  await new Promise(resolve=>setTimeout(resolve,750));
  assert.equal(await page.$eval('crm-record-picker',host=>host.value),'c_client');
  assert.equal(await page.$eval('crm-record-picker',host=>host.shadowRoot.textContent.includes('Slow result')),false);
  await search.focus();await page.keyboard.press('ArrowDown');
  assert.ok(await page.$eval('crm-record-picker',host=>host.shadowRoot.activeElement.tagName==='BUTTON'&&getComputedStyle(host.shadowRoot.activeElement).outlineStyle!=='none'));
  await page.keyboard.press('Enter');
  assert.equal(await page.$eval('crm-record-picker',host=>host.value),'pick-239');
  assert.deepEqual(errors,[]);
 }catch(error){throw new Error(error.message+' '+await page.$eval('crm-record-picker',host=>JSON.stringify({offset:host.offset,text:host.shadowRoot.textContent}))+' '+JSON.stringify(calls))}finally{await page.close()}
 });
