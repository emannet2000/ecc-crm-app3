'use strict';
// Real-provider acceptance. Nothing is sent unless ECC_ACCEPTANCE_SEND=true.
const fs = require('node:fs');
const {randomUUID} = require('node:crypto');
const env = process.env;
const base = new URL(env.ECC_ACCEPTANCE_URL || 'http://127.0.0.1:8080');
if (!['http:', 'https:'].includes(base.protocol)) throw new Error('Use an HTTP or HTTPS app URL');
const send = env.ECC_ACCEPTANCE_SEND === 'true';
const startedAt = new Date().toISOString();
const marker = env.ECC_ACCEPTANCE_MARKER || 'Acceptance ' + randomUUID();
const seconds = Number(env.ECC_ACCEPTANCE_SECONDS || 180);
if (!Number.isFinite(seconds) || seconds < 1 || seconds > 3600) throw new Error('Acceptance wait must be 1–3600 seconds');
const cookies = new Map();
const results = [];
async function api(method, path, body) {
 const url = new URL(path, base);
 const response = await fetch(url, {method, redirect:'error', signal:AbortSignal.timeout(90000), headers:{'Content-Type':'application/json', Origin:base.origin, Cookie:[...cookies].map(([k,v])=>k+'='+v).join('; ')}, body:body===undefined?undefined:JSON.stringify(body)});
 for (const cookie of response.headers.getSetCookie()) {const first=cookie.split(';')[0], at=first.indexOf('=');cookies.set(first.slice(0,at),first.slice(at+1));}
 const out = await response.json();
 if (!response.ok) throw new Error('HTTP '+response.status+' for '+path.split('?')[0]);
 return out;
}
async function waitFor(check) {
 const until=Date.now()+seconds*1000;
 do {const value=await check();if(value)return value;await new Promise(resolve=>setTimeout(resolve,2000));} while(Date.now()<until);
 return null;
}
async function check(name, action) {
 try {results.push({integration:name,...await action()});} catch(error) {results.push({integration:name,status:'failed',detail:error.message});}
}
async function main() {
 if (!env.ECC_ACCEPTANCE_EMAIL || !env.ECC_ACCEPTANCE_PASSWORD) throw new Error('Set acceptance login email and password through environment variables');
 await api('POST','/api/login',{email:env.ECC_ACCEPTANCE_EMAIL,password:env.ECC_ACCEPTANCE_PASSWORD});
 const health=(await api('GET','/api/integrations/status')).integrations;
 const configured=id=>health.some(item=>item.id===id&&!['needs_configuration','disabled'].includes(item.state));
 const contact=env.ECC_ACCEPTANCE_CONTACT_ID;
 const messages=async()=>contact?(await api('GET','/api/contacts/'+encodeURIComponent(contact)+'/conversations?limit=100')).messages:[];
 await check('email',async()=>{
  if(!configured('email'))return {status:'not_configured'};
  if(!send||!contact||!env.ECC_ACCEPTANCE_RECIPIENT)return {status:'pending',detail:'Sending requires a dedicated contact, inbox recipient and explicit send flag'};
  const record=(await api('GET','/api/contacts/'+encodeURIComponent(contact))).contact;
  if(record?.email?.toLowerCase()!==env.ECC_ACCEPTANCE_RECIPIENT.toLowerCase())throw new Error('Dedicated recipient must match the test contact email');
  await api('POST','/api/mail',{recipient:env.ECC_ACCEPTANCE_RECIPIENT,subject:marker,body:'Please reply with the exact subject marker after checking this message arrived: '+marker,contactId:contact});
  const sent=await waitFor(async()=>{const out=await api('GET','/api/control-center');return(out.mail||[]).find(row=>row.subject===marker&&row.status==='sent'&&Date.parse(row.createdAt)>=Date.parse(startedAt))});
  if(!sent)return {status:'pending',detail:'SMTP acceptance was not observed before timeout'};
  const reply=await waitFor(async()=>(await messages()).find(row=>row.channel==='email'&&row.direction==='inbound'&&Date.parse(row.createdAt)>=Date.parse(startedAt)&&(String(row.subject)+String(row.body)).includes(marker)));
  return {status:reply?'passed':'provider_accepted',detail:reply?'Reply ingested and associated with designated contact':'SMTP accepted; inbox delivery and reply ingestion remain unverified'};
 });
 await check('whatsapp',async()=>{
  if(!configured('whatsapp'))return {status:'not_configured'};
  if(!send||!contact||env.ECC_ACCEPTANCE_WHATSAPP!=='true')return {status:'pending',detail:'Dedicated WhatsApp contact and explicit WhatsApp send flag required'};
  const queued=await api('POST','/api/contacts/'+encodeURIComponent(contact)+'/whatsapp',{body:'Please reply with '+marker});
  const received=await waitFor(async()=>{const rows=await messages();return rows.some(row=>row.id===queued.id&&['delivered','read'].includes(row.status))&&rows.some(row=>row.channel==='whatsapp'&&row.direction==='inbound'&&Date.parse(row.createdAt)>=Date.parse(startedAt)&&String(row.body).includes(marker))});
  return {status:received?'passed':'pending',detail:received?'Delivered callback and inbound reply observed':'Delivery callback or matching reply not yet observed'};
 });
 await check('payments',async()=>{
  const payment=health.find(item=>item.id==='payments');
  if(!configured('payments'))return {status:'not_configured'};
  if(!send||!env.ECC_ACCEPTANCE_INVOICE_ID)return {status:'pending',detail:'A designated unpaid test invoice and send flag are required'};
  if(payment.testMode!==true)return {status:'pending',detail:'Checkout refused: provider test mode is not confirmed'};
  const id=encodeURIComponent(env.ECC_ACCEPTANCE_INVOICE_ID);
  const before=await api('GET','/api/invoices/'+id);
  if(!(Number(before.invoice?.balance)>0))throw new Error('Use an unpaid test invoice');
  const prior=new Set((await api('GET','/api/payments?invoiceId='+id+'&limit=200')).payments.map(row=>row.id));
  const checkout=await api('POST','/api/invoices/'+id+'/checkout',{});
  const url=new URL(checkout.url);
  if(url.protocol!=='https:'||url.hostname!=='checkout.stripe.com')throw new Error('Unexpected checkout host');
  console.error('Complete the designated test checkout: '+url.href);
  const paid=await waitFor(async()=>{const out=await api('GET','/api/invoices/'+id);if(!out.invoice||Number(out.invoice.balance)!==0)return false;const ledger=await api('GET','/api/payments?invoiceId='+id+'&limit=200');return ledger.payments.some(row=>!prior.has(row.id)&&row.method==='Stripe'&&row.createdBy==='Stripe webhook')});
  return {status:paid?'callback_observed':'pending',detail:paid?'Invoice settled; replay and failed/cancelled checkout require the companion checklist':'Waiting for test checkout and signed callback'};
 });
 await check('ai',async()=>{
  if(!configured('ai'))return {status:'not_configured'};
  if(!send||!contact)return {status:'pending',detail:'Explicit provider-call flag and a designated test contact required'};
  const out=await api('POST','/api/assistant/chat',{message:'Summarize this test contact using its record. Do not propose or execute changes.',entity:'contacts',id:contact});
  if(!out.reply||!out.threadId)throw new Error('Response or saved thread is missing');
  const thread=await api('GET','/api/assistant/threads/'+encodeURIComponent(out.threadId));
  if(!thread.messages?.length)throw new Error('Conversation was not saved');
  for(const source of out.sources||[])if(source.entity&&source.id)await api('GET','/api/'+encodeURIComponent(source.entity)+'/'+encodeURIComponent(source.id));
  return {status:'response_observed',detail:'Real response and saved history verified; source/role/action acceptance requires the companion checklist'};
 });
 const report={date:new Date().toISOString(),environment:base.origin,marker,sendingEnabled:send,results};
 const output=JSON.stringify(report,null,2)+'\n';
 if(env.ECC_ACCEPTANCE_REPORT)fs.writeFileSync(env.ECC_ACCEPTANCE_REPORT,output,{mode:0o600});
 console.log(output);
 if(results.some(row=>row.status==='failed'))process.exitCode=1;
}
main().catch(()=>{console.error('Acceptance setup failed. Check URL, login and server logs; no credentials are printed.');process.exitCode=1});
