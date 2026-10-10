const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const os=require('node:os');
const net=require('node:net');
const {spawn}=require('node:child_process');
const puppeteer=require('puppeteer');
const pause=ms=>new Promise(resolve=>setTimeout(resolve,ms));
async function api(page,method,url,body){
 const result=await page.evaluate(async(method,url,body)=>{const response=await fetch(url,{method,credentials:'same-origin',headers:{'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body)});return{status:response.status,data:await response.json()}},method,url,body);
 return result;
}
async function clickText(page,text,container='document'){
 await page.waitForFunction((text,container)=>{const root=container==='document'?document:(document.querySelector(container)?.shadowRoot||document.querySelector(container));return root&&[...root.querySelectorAll('button')].some(b=>b.textContent.trim()===text&&!b.disabled)}, {}, text,container);
 await page.evaluate((text,container)=>{const root=container==='document'?document:(document.querySelector(container).shadowRoot||document.querySelector(container));const button=[...root.querySelectorAll('button')].find(b=>b.textContent.trim()===text&&!b.disabled);if(!button)throw new Error('Missing available button: '+text);button.click()},text,container);
}
async function input(page,selector,value){
 await page.$eval(selector,(element,value)=>{element.value=value;element.dispatchEvent(new Event(element.tagName==='SELECT'?'change':'input',{bubbles:true}));if(element.tagName==='SELECT')element.dispatchEvent(new Event('input',{bubbles:true}));},value);
}
async function chooseRecord(page,selector,query,id){
 await page.waitForFunction(selector=>document.querySelector(selector)?.shadowRoot.querySelector('input[type=search]'),{},selector);
 await page.$eval(selector,(host,query)=>{const input=host.shadowRoot.querySelector('input');input.value=query;input.dispatchEvent(new Event('input',{bubbles:true,composed:true}))},query);
 await page.waitForFunction((selector,id)=>[...document.querySelector(selector).shadowRoot.querySelectorAll('.results button')].some(b=>b.textContent.includes(id)),{},selector,id);
 await page.$eval(selector,(host,id)=>[...host.shadowRoot.querySelectorAll('.results button')].find(b=>b.textContent.includes(id)).click(),id);
}
async function saveModal(page,method,endpoint){
 const response=page.waitForResponse(r=>new URL(r.url()).pathname===endpoint&&r.request().method()===method);
 await page.evaluate(()=>document.querySelector('.modal form').requestSubmit());const out=await response;assert.ok(out.ok(),await out.text());const data=await out.json();await page.waitForFunction(()=>!document.querySelector('.modal'));return data;
}
async function toolSubmit(page,title,values={}){
 await page.waitForFunction(title=>[...document.querySelector('crm-record-tools').shadowRoot.querySelectorAll('.card')].some(c=>c.querySelector('h2')?.textContent===title),{},title);
 await page.evaluate((title,values)=>{const root=document.querySelector('crm-record-tools').shadowRoot,card=[...root.querySelectorAll('.card')].find(c=>c.querySelector('h2')?.textContent===title),form=card.querySelector('form');for(const[key,value]of Object.entries(values)){const element=form.elements.namedItem(key);element.value=value;element.dispatchEvent(new Event('change',{bubbles:true}))}form.requestSubmit()},title,values);
}
async function login(page,base,email,password){
 await page.goto(base,{waitUntil:'domcontentloaded'});await page.waitForSelector('input[type=email]');await page.type('input[type=email]',email);await page.type('input[type=password]',password);await page.click('button[type=submit]');await page.waitForSelector('.app-shell');
}

test('live education journey: lead conversion, student, application, review, finance, portal, roles and mobile', {skip:process.env.ECC_BROWSER_LIVE!=='true',timeout:300000},async()=>{
 const directory=fs.mkdtempSync(path.join(os.tmpdir(),'ecc-journey-')),port=await new Promise(resolve=>{const listener=net.createServer();listener.listen(0,'127.0.0.1',()=>{const port=listener.address().port;listener.close(()=>resolve(port))})}),base='http://127.0.0.1:'+port;
 const server=spawn(path.join(__dirname,'../build/ecc-crm'),[],{cwd:path.join(__dirname,'..'),env:{...process.env,PORT:String(port),DATABASE_PATH:path.join(directory,'crm.sqlite3'),JWT_SECRET:'journey-test-secret-only',APP_URL:base,ALLOW_SIGNUP:'false',SMTP_HOST:'',OPENAI_API_KEY:'',STRIPE_SECRET_KEY:'',GRAPH_ORG_ID:'',WHATSAPP_ORG_ID:''},stdio:'pipe'});
 let serverLog='';server.stderr.on('data',chunk=>serverLog+=chunk);server.stdout.on('data',chunk=>serverLog+=chunk);const executablePath=process.env.CHROME_PATH||await puppeteer.executablePath();let browser,staff,client,viewer;let step='login';const errors=[],pending=new Set();
 try{
  for(let i=0;i<150;i++){try{const response=await fetch(base+'/api/health');if(response.ok)break}catch{}if(server.exitCode!==null)throw new Error(serverLog);await pause(100)}
  browser=await puppeteer.launch({executablePath,headless:true,pipe:true,args:['--no-sandbox','--disable-dev-shm-usage']});staff=await browser.newPage();await staff.setViewport({width:1280,height:900});staff.setDefaultTimeout(20000);staff.on('pageerror',error=>errors.push(error.message));staff.on('request',r=>pending.add(r.url()));staff.on('requestfinished',r=>pending.delete(r.url()));staff.on('requestfailed',r=>pending.delete(r.url()));await login(staff,base,'demo@northwind.dev','Demo1234');
  await staff.waitForFunction(()=>document.querySelector('crm-my-work')?.shadowRoot.textContent.includes('work items'));
  assert.ok(await staff.evaluate(()=>document.querySelector('crm-my-work').getBoundingClientRect().top<document.querySelector('.stats').getBoundingClientRect().top));
  assert.equal((await api(staff,'POST','/api/admin/users',{name:'Journey Adviser',email:'journey-adviser@example.com',password:'JourneyPassword123!',role:'member'})).status,201);
  assert.equal((await api(staff,'POST','/api/admin/users',{name:'Journey Viewer',email:'journey-viewer@example.com',password:'JourneyPassword123!',role:'viewer'})).status,201);
  // Create a lead through the real Elm form and check inline guidance.
  await staff.goto(base+'/leads',{waitUntil:'domcontentloaded'});await clickText(staff,'Add lead');await staff.waitForSelector('#lf-name');await input(staff,'#lf-name','Journey Student');await input(staff,'#lf-email','journey-student@example.com');
  await staff.waitForFunction(()=>document.querySelector('crm-form-quality')?.shadowRoot.textContent.includes('follow-up date'));await input(staff,'#lf-followUpDate','2099-01-01');const lead=(await saveModal(staff,'POST','/api/leads')).lead;
  // Conversion uses the existing native record workflow.
  await staff.goto(base+'/leads/'+lead.id,{waitUntil:'domcontentloaded'});await staff.waitForFunction(()=>document.querySelector('crm-record-tools')?.shadowRoot.querySelector('[name=score]'));
  const conversion=staff.waitForResponse(r=>new URL(r.url()).pathname==='/api/workflows/lead-convert'&&r.request().method()==='POST');await toolSubmit(staff,'Lead conversion');const converted=await conversion;assert.ok(converted.ok());const contactId=(await converted.json()).contactId;
  await staff.goto(base+'/students',{waitUntil:'domcontentloaded'});await clickText(staff,'Add student');await staff.waitForSelector('#stf-name');await input(staff,'#stf-name','Journey Student');await input(staff,'#stf-studentCode','JOURNEY-001');await input(staff,'#stf-countryCode','PH');await input(staff,'#stf-schoolId','s_1');await input(staff,'#stf-program','Computer Science');const student=(await saveModal(staff,'POST','/api/students')).student;
  await staff.goto(base+'/students/'+student.id,{waitUntil:'domcontentloaded'});await staff.waitForFunction(()=>document.querySelector('crm-student-journey')?.shadowRoot.textContent.includes('No linked applications'));
  const creation=staff.waitForResponse(r=>new URL(r.url()).pathname==='/api/workflows/student-case'&&r.request().method()==='POST');await toolSubmit(staff,'Create a student case');const caseResponse=await creation;assert.ok(caseResponse.ok());const application=(await caseResponse.json()).case;assert.equal(application.clientId,contactId);
  await staff.goto(base+'/cases/'+application.id,{waitUntil:'domcontentloaded'});await clickText(staff,'Edit','.detail-hero');
  await staff.waitForSelector('#cf-nextAction');await input(staff,'#cf-nextAction','Review the required passport');await input(staff,'#cf-nextDeadline','2099-01-01');await saveModal(staff,'PUT','/api/cases/'+application.id);
  await clickText(staff,'Add document');await staff.waitForSelector('#df-docName');await input(staff,'#df-docName','Journey Passport');const document=(await saveModal(staff,'POST','/api/documents')).document;
  // Upload an actual file before staff document verification.
  step='document tool options';await staff.waitForFunction(id=>document.querySelector('crm-record-tools')?.shadowRoot.querySelector('select[name=id] option[value="'+id+'"]'),{},document.id);
  const file=path.join(directory,'passport.txt');fs.writeFileSync(file,'Test passport evidence');const fileInput=await staff.$('crm-record-tools >>> input[type=file]');await fileInput.uploadFile(file);
  step='document upload';const uploaded=staff.waitForResponse(r=>new URL(r.url()).pathname==='/api/documents/'+document.id+'/files'&&r.request().method()==='POST');
  await staff.evaluate(id=>{const root=document.querySelector('crm-record-tools').shadowRoot,form=root.querySelector('input[type=file]').closest('form');form.elements.namedItem('id').value=id;form.requestSubmit()},document.id);assert.ok((await uploaded).ok());
  step='document row';await staff.waitForFunction(()=>[...document.querySelectorAll('.activity-item')].some(item=>item.textContent.includes('Journey Passport')&&item.querySelector('.stage-pill--active')?.textContent==='Received'));
  await staff.waitForFunction(()=>document.querySelector('crm-application-blockers')?.shadowRoot.textContent.includes('Journey Passport'));
  assert.ok((await api(staff,'GET','/api/cases/'+application.id+'/blockers')).data.items.some(item=>item.kind==='document'&&item.route.includes(document.id)));
  step='verify document';const verified=staff.waitForResponse(r=>new URL(r.url()).pathname==='/api/documents/'+document.id+'/status'&&r.request().method()==='PATCH');
  await staff.evaluate(()=>{const item=[...document.querySelectorAll('.activity-item')].find(e=>e.textContent.includes('Journey Passport'));[...item.querySelectorAll('button')].find(b=>b.textContent.trim()==='Verified').click()});const verification=await verified;assert.ok(verification.ok(),await verification.text());
  step='create invoice';await staff.goto(base+'/invoices',{waitUntil:'domcontentloaded'});await clickText(staff,'Add invoice');await chooseRecord(staff,'#inv-clientId','Journey Student',contactId);await chooseRecord(staff,'#inv-caseId',application.caseNumber,application.id);await input(staff,'#inv-totalFee','150');const invoice=(await saveModal(staff,'POST','/api/invoices')).invoice;
  assert.equal((await api(staff,'POST','/api/payments',{invoiceId:invoice.id,amount:50,paidOn:'2026-10-10',method:'Bank',reference:'Journey receipt'})).status,201);
  step='pending receipt';await staff.goto(base+'/invoices/'+invoice.id,{waitUntil:'domcontentloaded'});await clickText(staff,'Record payment');await input(staff,'#pf-amount','25');await input(staff,'#pf-paidOn','2026-10-10');await input(staff,'#pf-method','Bank');await input(staff,'#pf-reference','Journey approval receipt');await staff.click('#pf-requireApproval');await staff.waitForFunction(()=>document.getElementById('pf-requireApproval')?.checked);const pendingReceipt=(await saveModal(staff,'POST','/api/payments')).payment;assert.equal(pendingReceipt.status,'pending_approval');
  await staff.waitForFunction(()=>document.querySelector('crm-invoice-status')?.shadowRoot.textContent.includes('Awaiting approval: USD 25.00'));
  assert.equal((await api(staff,'GET','/api/invoices/'+invoice.id+'/finance-status')).data.invoice.balance,100);
  step='approve receipt';await clickText(staff,'Approve receipt','crm-invoice-status');await staff.waitForFunction(()=>document.querySelector('crm-invoice-status')?.shadowRoot.textContent.includes('Outstanding balance: USD 75.00'));
  assert.equal((await api(staff,'POST','/api/payments',{invoiceId:invoice.id,amount:25,paidOn:'2026-10-10',method:'Bank',reference:'Journey approval receipt',confirmDuplicate:true})).status,400);
  step='possible duplicate receipt';await clickText(staff,'Record payment');await input(staff,'#pf-amount','25');await input(staff,'#pf-paidOn','2026-10-10');await input(staff,'#pf-method','Bank');await input(staff,'#pf-reference','Journey separate receipt');let warning=staff.waitForResponse(r=>new URL(r.url()).pathname==='/api/payments'&&r.request().method()==='POST');await staff.evaluate(()=>document.querySelector('.modal form').requestSubmit());assert.equal((await warning).status(),400);await staff.waitForSelector('#pf-confirmDuplicate');assert.ok(await staff.$eval('.modal',el=>el.textContent.includes('same amount, date and method')));await staff.click('#pf-confirmDuplicate');await saveModal(staff,'POST','/api/payments');
  await staff.goto(base+'/students/'+student.id,{waitUntil:'domcontentloaded'});await staff.waitForFunction(()=>document.querySelector('crm-student-journey')?.shadowRoot.textContent.includes('1 of 1 required documents verified'));
  assert.ok(await staff.evaluate(()=>document.querySelector('crm-student-journey').shadowRoot.textContent.includes('Journey receipt')));
  await clickText(staff,'Client portal','crm-record-tools');const invitation=staff.waitForResponse(r=>new URL(r.url()).pathname==='/api/portal/invitations'&&r.request().method()==='POST');await toolSubmit(staff,'Invite a client',{email:'journey-student@example.com'});const invited=await invitation;assert.ok(invited.ok());const portalURL=(await invited.json()).url;
  const clientContext=await browser.createBrowserContext();client=await clientContext.newPage();client.on('pageerror',error=>errors.push(error.message));await client.goto(portalURL,{waitUntil:'domcontentloaded'});await client.waitForSelector('textarea');assert.equal(new URL(client.url()).hash,'');
  await client.type('textarea','I reviewed the next step and uploaded my documents.');await client.evaluate(()=>document.querySelector('textarea').closest('form').requestSubmit());await client.waitForFunction(()=>document.getElementById('notice').textContent.includes('response was sent'));
  assert.equal((await api(client,'GET','/api/students/'+student.id+'/journey')).status,401);
  await clickText(staff,'Refresh journey','crm-student-journey');await staff.waitForFunction(()=>document.querySelector('crm-student-journey').shadowRoot.textContent.includes('I reviewed the next step'));
  // The preset affects navigation only. Verify persistence, keyboard focus and
  // touch targets on phone/tablet, including expandable journey sections.
  for(const width of [320,390,768]){
   await staff.setViewport({width,height:844,isMobile:true,hasTouch:true});await staff.goto(base+'/students/'+student.id,{waitUntil:'domcontentloaded'});await staff.waitForFunction(()=>document.querySelector('crm-student-journey')?.shadowRoot.querySelector('details'));
   assert.ok(await staff.$eval('body',body=>body.scrollWidth<=innerWidth+2),'journey overflow at '+width);assert.equal(await staff.$$eval('.mobile-nav button',buttons=>buttons.filter(b=>getComputedStyle(b).display!=='none').length),5);
   const summary=await staff.$('crm-student-journey >>> .journey-section > summary');await summary.focus();await staff.keyboard.press('Tab');await staff.keyboard.down('Shift');await staff.keyboard.press('Tab');await staff.keyboard.up('Shift');assert.ok(await staff.evaluate(()=>{const root=document.querySelector('crm-student-journey').shadowRoot;const active=root.activeElement;return active?.tagName==='SUMMARY'&&getComputedStyle(active).outlineStyle!=='none'&&active.getBoundingClientRect().height>=44}));await staff.keyboard.press('Enter');assert.ok(await staff.evaluate(()=>document.querySelector('crm-student-journey').shadowRoot.querySelector('details').open));
   await staff.click('.topbar__menu-btn');await staff.waitForSelector('.sidebar.is-open');await input(staff,'crm-nav-focus select','finance');await staff.click('.sidebar__close');assert.equal(await staff.$eval('.mobile-nav [data-nav-label=Invoices]',el=>getComputedStyle(el).display),'flex');
   assert.ok(await staff.$eval('.mobile-nav [data-nav-label=Invoices]',el=>el.getBoundingClientRect().height>=44));
  }
  await staff.reload({waitUntil:'domcontentloaded'});await staff.waitForFunction(()=>document.querySelector('.sidebar')?.dataset.focus==='finance');assert.equal(await staff.$eval('.sidebar',el=>el.dataset.focus),'finance');
  await staff.setViewport({width:1280,height:900});await staff.goto(base+'/workspace?tab=integrations',{waitUntil:'domcontentloaded'});await staff.waitForFunction(()=>document.querySelector('crm-workspace')?.shadowRoot.querySelector('crm-integration-health')?.shadowRoot.textContent.includes('needs configuration'));
  const viewerContext=await browser.createBrowserContext();viewer=await viewerContext.newPage();await login(viewer,base,'journey-viewer@example.com','JourneyPassword123!');assert.equal(await viewer.$eval('.sidebar',el=>el.dataset.focus),'adviser');assert.equal((await api(viewer,'POST','/api/students',{name:'Forbidden'})).status,403);assert.equal((await api(viewer,'POST','/api/admin/integrations/payments/test')).status,403);assert.equal((await api(viewer,'GET','/api/students/'+student.id+'/journey')).status,200);
  const memberContext=await browser.createBrowserContext(),member=await memberContext.newPage();await login(member,base,'journey-adviser@example.com','JourneyPassword123!');assert.equal(await member.$eval('.sidebar',el=>el.dataset.focus),'adviser');assert.equal((await api(member,'PUT','/api/students/'+student.id,{name:'Forbidden ownership edit'})).status,403);// Exercise the handoff UI in both authenticated roles.
  step='handoff proposal';await staff.goto(base+'/cases/'+application.id,{waitUntil:'domcontentloaded'});await staff.waitForFunction(()=>document.querySelector('crm-case-handoff')?.shadowRoot.querySelector('form'));
  await staff.evaluate(()=>{const root=document.querySelector('crm-case-handoff').shadowRoot;root.querySelector('details').open=true;const form=root.querySelector('form');const recipient=[...form.elements.toId.options].find(o=>o.textContent==='Journey Adviser');form.elements.toId.value=recipient.value;form.elements.reason.value='Continue the review';form.elements.nextAction.value='Check enrollment evidence';form.elements.dueDate.value='2099-01-01';form.requestSubmit()});
  await staff.waitForFunction(()=>document.querySelector('crm-case-handoff').shadowRoot.textContent.includes('Awaiting Journey Adviser'));
  step='handoff acceptance';await member.goto(base+'/cases/'+application.id,{waitUntil:'domcontentloaded'});await member.waitForFunction(()=>document.querySelector('crm-case-handoff')?.shadowRoot.textContent.includes('Accept handoff'));await member.evaluate(()=>document.querySelector('crm-case-handoff').shadowRoot.querySelector('details').open=true);await clickText(member,'Accept handoff','crm-case-handoff');await member.waitForFunction(()=>document.querySelector('crm-case-handoff').shadowRoot.textContent.includes('Current owner: Journey Adviser'));
  await member.close();
  step='usability observation';await staff.goto(base+'/workspace?tab=usability',{waitUntil:'domcontentloaded'});await staff.waitForFunction(()=>document.querySelector('crm-workspace')?.shadowRoot.querySelector('crm-usability')?.shadowRoot.querySelector('form'));
  await staff.evaluate(()=>{const form=document.querySelector('crm-workspace').shadowRoot.querySelector('crm-usability').shadowRoot.querySelector('form');form.elements.seconds.value='75';form.elements.comments.value='Automated fixture observation; this is not staff session evidence.';form.elements.completed.checked=true;form.requestSubmit()});await staff.waitForFunction(()=>document.querySelector('crm-workspace').shadowRoot.querySelector('crm-usability').shadowRoot.textContent.includes('Automated fixture observation'));
  const acceptance=await new Promise((resolve,reject)=>{const child=spawn(process.execPath,[path.join(__dirname,'live-integrations.cjs')],{env:{...process.env,ECC_ACCEPTANCE_URL:base,ECC_ACCEPTANCE_EMAIL:'demo@northwind.dev',ECC_ACCEPTANCE_PASSWORD:'Demo1234',ECC_ACCEPTANCE_SEND:'false'}});let output='',error='';child.stdout.on('data',chunk=>output+=chunk);child.stderr.on('data',chunk=>error+=chunk);child.once('error',reject);child.once('exit',code=>code===0?resolve(JSON.parse(output)):reject(new Error(error)))});
  assert.equal(acceptance.sendingEnabled,false);assert.equal(acceptance.results.length,4);assert.ok(acceptance.results.every(row=>row.status==='not_configured'));
  
  assert.deepEqual(errors,[]);
 }catch(error){if(staff){fs.mkdirSync(path.join(__dirname,'artifacts'),{recursive:true});await staff.screenshot({path:path.join(__dirname,'artifacts/journey-failure.png'),fullPage:true}).catch(()=>{})}throw new Error(error.stack+"\nStep: "+step+"\nUI: "+await staff.evaluate(()=>JSON.stringify({url:location.pathname,modal:document.querySelector('.modal')?.innerText,tools:document.querySelector('crm-record-tools')?.shadowRoot.textContent,fields:[...document.querySelectorAll('.modal input,.modal select')].map(e=>[e.id,e.value])}))+"\nPending: "+[...pending].join(", ")+"\nBrowser errors: "+errors.join("; ")+"\n"+serverLog.slice(-2500))}
 finally{await browser?.close();server.kill('SIGTERM');await new Promise(resolve=>{if(server.exitCode!==null)return resolve();server.once('exit',resolve)});fs.rmSync(directory,{recursive:true,force:true})}
});
