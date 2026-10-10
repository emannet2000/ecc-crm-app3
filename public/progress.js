'use strict';
(() => {
 const node=(tag,text,attrs={})=>{const el=document.createElement(tag);if(text!=null)el.textContent=String(text);for(const[k,v]of Object.entries(attrs))el.setAttribute(k,v);return el};
 async function request(path,body){const options={credentials:'same-origin',...(body===undefined?{}:{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)})};let response=await fetch(path,options);if(response.status===401){const refreshed=await fetch('/api/session/refresh',{method:'POST',credentials:'same-origin'});if(refreshed.ok)response=await fetch(path,options)}const out=await response.json();if(!response.ok)throw new Error(out.error||'Could not load record. Refresh and try again.');return out}
 class RecordProgress extends HTMLElement {
  static get observedAttributes(){return ['record-id']}
  constructor(){super();this.attachShadow({mode:'open'});this.generation=0}
  connectedCallback(){this.changed=()=>this.load();window.addEventListener('crm:record-changed',this.changed);this.load()}
  attributeChangedCallback(){if(this.isConnected)this.load()}
  disconnectedCallback(){this.generation++;window.removeEventListener('crm:record-changed',this.changed)}
  async load(){const generation=++this.generation;const root=node('section',null,{class:'tools progress-panel'}),notice=node('p','Loading…',{role:'status','aria-live':'polite'}),refresh=node('button','Refresh '+this.label,{type:'button',class:'secondary'});this.shadowRoot.replaceChildren(node('link',null,{rel:'stylesheet',href:'/public/center.css'}),node('link',null,{rel:'stylesheet',href:'/public/components.css'}),root);refresh.onclick=()=>this.load();root.append(node('h2',this.label),refresh,notice);try{const out=await request(this.endpoint());if(!this.isConnected||generation!==this.generation)return;notice.textContent='';this.render(root,out,notice,generation)}catch(error){if(generation===this.generation)notice.textContent=error.message}}
 }
 class ApplicationBlockers extends RecordProgress {
  get label(){return 'Application requirements'}
  endpoint(){return '/api/cases/'+encodeURIComponent(this.getAttribute('record-id'))+'/blockers'}
  render(root,out){root.append(node('p',out.blockingCount?out.blockingCount+' document requirements prevent automatic submission readiness.':'Review the items below before progressing this application.'));if(!out.items?.length)root.append(node('p','No issues found in accessible application records.'));for(const item of out.items||[]){const card=node('article',null,{class:'card requirement-item'});card.append(node('strong',item.title),node('p',item.detail),node('p',item.blocking?'Blocks automatic submission readiness':'Needs staff attention'),node('a',item.action,{href:item.route,class:'button secondary'}));root.append(card)}}
 }
 class InvoiceFinance extends RecordProgress {
  get label(){return 'Balance and receipt approvals'}
  endpoint(){return '/api/invoices/'+encodeURIComponent(this.getAttribute('record-id'))+'/finance-status'}
  render(root,out,notice,generation){const inv=out.invoice,currency=inv.currency||'USD',amount=value=>currency+' '+Number(value||0).toFixed(2);root.append(node('p','Posted receipts: '+amount(inv.amountReceived)+' · Outstanding balance: '+amount(Math.max(0,Number(inv.balance)))+(Number(inv.balance)<0?' · Credit balance: '+amount(-Number(inv.balance)):'')),node('p','Awaiting approval: '+amount(out.pendingAmount)+' in '+out.pendingCount+' receipt(s). Pending receipts do not reduce the posted balance.'));if(out.pendingCount)root.append(node('p','Review pending receipts before requesting another payment.'));if(out.approvalNote)root.append(node('p','Legacy invoice approval note: '+out.approvalNote));if(!out.pendingCount)root.append(node('p','No receipts awaiting approval.'));for(const receipt of out.pendingReceipts||[]){const card=node('article',null,{class:'card'});card.append(node('strong',amount(receipt.amount)+' · '+(receipt.reference||'No reference')),node('p',(receipt.method||'Method not set')+' · '+receipt.paidOn+' · recorded by '+receipt.createdBy));if(out.canReview)for(const[decision,title]of [['approve','Approve receipt'],['reject','Reject receipt']]){const button=node('button',title,{type:'button',class:'secondary'});button.onclick=async()=>{card.querySelectorAll('button').forEach(b=>b.disabled=true);try{await request('/api/payments/'+encodeURIComponent(receipt.id)+'/review',{decision});if(!this.isConnected||generation!==this.generation)return;window.dispatchEvent(new CustomEvent('crm:record-changed',{detail:inv.id}))}catch(error){notice.textContent=error.message;card.querySelectorAll('button').forEach(b=>b.disabled=false)}};card.append(button)}else card.append(node('p','Awaiting manager review.'));root.append(card)}}
 }
 customElements.define('crm-application-blockers',ApplicationBlockers);
 customElements.define('crm-invoice-status',InvoiceFinance);
})();
