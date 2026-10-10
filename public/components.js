'use strict';
(function () {
  const node = (tag, text, attrs = {}) => {
    const element = document.createElement(tag);
    if (text !== undefined && text !== null) element.textContent = String(text);
    for (const [key, value] of Object.entries(attrs)) element.setAttribute(key, value);
    return element;
  };
  const style = root => {
    root.append(node('link', null, {rel: 'stylesheet', href: '/public/center.css'}));
    root.append(node('link', null, {rel: 'stylesheet', href: '/public/components.css'}));
    root.append(node('link', null, {rel: 'stylesheet', href: '/public/mobile-tables.css'}));
  };
  async function request(path, options = {}) {
    let response = await fetch(path, {credentials: 'same-origin', ...options});
    if (response.status === 401) {
      const refresh = await fetch('/api/session/refresh', {method: 'POST', credentials: 'same-origin'});
      if (refresh.ok) response = await fetch(path, {credentials: 'same-origin', ...options});
    }
    const result = await response.json();
    if (!response.ok) throw new Error(result.error || 'Request failed. Use Refresh to retry.');
    return result;
  }
  function shell(root) {
    style(root);
    const container = node('section', null, {class: 'tools'});
    // This template is static. Record/provider content is inserted only as text.
    container.innerHTML = '<header><div><h2 id="title">Record tools</h2><p id="subtitle"></p></div><button id="refresh" class="secondary" type="button">Refresh tools</button></header><nav id="tabs" aria-label="Record tools"></nav><div id="notice" role="status" aria-live="polite"></div><main id="content" tabindex="-1"><p>Loading…</p></main>';
    root.append(container);
  }
  class Workspace extends HTMLElement {
    constructor() { super(); this.attachShadow({mode: 'open'}); }
    connectedCallback() {
      shell(this.shadowRoot);
      this.stop = window.mountWorkspaceTools(this.shadowRoot);
    }
    disconnectedCallback() { this.stop?.(); this.shadowRoot.replaceChildren(); }
  }
  class RecordTools extends HTMLElement {
    static get observedAttributes() { return ['entity', 'record-id']; }
    constructor() { super(); this.attachShadow({mode: 'open'}); this.generation = 0; }
    connectedCallback() { this.mount(); }
    attributeChangedCallback() { if (this.isConnected) this.mount(); }
    disconnectedCallback() { this.generation++; this.stop?.(); }
    async mount() {
      const generation = ++this.generation;
      this.stop?.(); this.shadowRoot.replaceChildren(); shell(this.shadowRoot);
      const entity = this.getAttribute('entity'), id = this.getAttribute('record-id');
      const main = this.shadowRoot.querySelector('#content');
      try {
        const detail = await request('/api/records/' + encodeURIComponent(entity) + '/' + encodeURIComponent(id));
        if (!this.isConnected || generation !== this.generation) return;
        const tabs = {contacts: 'email', cases: 'documents', students: 'workflows', deals: 'workflows', leads: 'workflows', schools: 'workflows', agents: 'workflows', partners: 'workflows', invoices: 'workflows'};
        this.stop = window.mountWorkspaceTools(this.shadowRoot, {entity, id, editable: detail.editable, tab: tabs[entity]});
      } catch (error) {
        if (generation === this.generation) main.replaceChildren(node('p', error.message));
      }
    }
  }
  class RecordPhoto extends HTMLElement {
    static get observedAttributes() { return ['entity', 'record-id', 'initials', 'person-name', 'editable']; }
    attributeChangedCallback() {
      if (!this.isConnected || !this.dataset.ready || this.refreshQueued) return;
      this.refreshQueued = true;
      queueMicrotask(() => {
        if (!this.isConnected) return;
        this.refreshQueued = false; this.dataset.ready = ''; this.replaceChildren(); this.connectedCallback();
      });
    }
    connectedCallback() {
      if (this.dataset.ready) return;
      this.dataset.ready = 'true';
      const entity = this.getAttribute('entity'), id = this.getAttribute('record-id');
      this.setAttribute('role', 'img');
      this.setAttribute('aria-label', this.getAttribute('person-name') ? 'Photo of ' + this.getAttribute('person-name') : 'Record photo');
      const fallback = node('span', this.getAttribute('initials') || '?', {class: 'record-photo__fallback', 'aria-hidden': 'true'});
      const image = node('img', null, {class: 'record-photo__image', alt: '', loading: 'lazy'});
      let removeButton = null;
      image.onload = () => { image.hidden = false; fallback.hidden = true; if (removeButton) removeButton.hidden = false; };
      image.onerror = () => { image.hidden = true; fallback.hidden = false; };
      this.append(fallback, image);
      const refresh = () => { image.src = '/api/records/' + encodeURIComponent(entity) + '/' + encodeURIComponent(id) + '/photo?v=' + Date.now(); };
      refresh();
      if (this.getAttribute('editable') !== 'true') return;
      const picker = node('input', null, {type: 'file', accept: 'image/png,image/jpeg', capture: 'environment', class: 'record-photo__input', 'aria-label': 'Choose a record photo'});
      const button = node('button', 'Add photo', {type: 'button', class: 'record-photo__button'});
      const notice = node('span', '', {class: 'record-photo__notice', role: 'status', 'aria-live': 'polite'});
      removeButton = node('button', 'Remove photo', {type: 'button', class: 'record-photo__remove', hidden: ''});
      button.onclick = () => picker.click();
      removeButton.onclick = async () => {
        if (!window.confirm('Remove this profile photo?')) return;
        removeButton.disabled = true;
        try {
          const response = await fetch('/api/records/' + encodeURIComponent(entity) + '/' + encodeURIComponent(id) + '/photo', {method: 'DELETE', credentials: 'same-origin'});
          if (!response.ok) throw new Error('Could not remove photo.');
          notice.textContent = 'Photo removed.'; image.hidden = true; fallback.hidden = false; removeButton.hidden = true;
        } catch (error) { notice.textContent = error.message; }
        finally { removeButton.disabled = false; }
      };
      picker.onchange = async () => {
        const photo = picker.files?.[0]; if (!photo) return;
        if (!['image/png', 'image/jpeg'].includes(photo.type) || photo.size > 5 * 1024 * 1024) { notice.textContent = 'Choose a PNG or JPEG smaller than 5 MB.'; return; }
        const data = new FormData(); data.append('photo', photo, photo.name);
        button.disabled = true; notice.textContent = 'Uploading…';
        try {
          const response = await fetch('/api/records/' + encodeURIComponent(entity) + '/' + encodeURIComponent(id) + '/photo', {method: 'PUT', body: data, credentials: 'same-origin'});
          if (!response.ok) { const result = await response.json().catch(() => ({})); throw new Error(result.message || result.error || 'Could not upload photo.'); }
          notice.textContent = 'Photo saved.'; refresh();
        } catch (error) { notice.textContent = error.message; }
        finally { button.disabled = false; picker.value = ''; }
      };
      this.append(button, removeButton, picker, notice);
    }
  }
  function table(title, rows, columns) {
    const card = node('section', null, {class: 'card'}); card.append(node('h3', title));
    if (!rows.length) { card.append(node('p', 'No matching records.')); return card; }
    const wrap = node('div', null, {class: 'table-wrap'}), table = node('table'), head = node('thead'), tr = node('tr');
    for (const [, label] of columns) tr.append(node('th', label, {scope: 'col'}));
    head.append(tr); table.append(head); const body = node('tbody');
    for (const record of rows) {
      const row = node('tr');
      for (const [key] of columns) {
        const cell = node('td');
        if (key === 'record' && record.url) cell.append(node('a', record[key], {href: record.url}));
        else cell.textContent = String(record[key] ?? '');
        row.append(cell);
      }
      body.append(row);
    }
    table.append(body); wrap.append(table); card.append(wrap); return card;
  }
  class AIProductivity extends HTMLElement {
    constructor() { super(); this.attachShadow({mode:'open'}); this.generation=0; }
    connectedCallback() { this.render(); }
    disconnectedCallback() { this.generation++; }
    render() {
      this.shadowRoot.replaceChildren(); style(this.shadowRoot);
      const root=node('section',null,{class:'tools'}), card=node('section',null,{class:'card'}), result=node('div',null,{'aria-live':'polite'});
      card.append(node('h3','AI work brief'),node('p','Get a prioritized view of your open tasks, near-term application deadlines, missing required documents, and lead follow-ups. AI receives work-item names, type, priority, deadlines, reasons, and action text from records you can access; names may identify people. Email addresses, phone numbers, and internal record IDs are excluded. AI does not send messages or change records.'),result);
      const button=node('button','Plan my day',{type:'button'});card.append(button);root.append(card);this.shadowRoot.append(root);
      button.onclick=async()=>{
        const generation=++this.generation;button.disabled=true;result.replaceChildren(node('p','Checking your accessible work…'));
        try {
          const queued=await request('/api/assistant/productivity',{method:'POST'});
          if(!this.isConnected||generation!==this.generation)return;
          if(queued.status==='empty'){result.replaceChildren(node('p',queued.message));return;}
          const rows=(queued.items||[]).map(item=>({priority:item.priority,record:item.name,reason:item.reason,action:item.action,deadline:item.deadline,url:item.route}));
          result.replaceChildren(node('p',`Work items selected: ${rows.length}. Review the source records before acting.`),table('Priority worklist',rows,[['priority','Priority'],['record','Record'],['reason','Why now'],['action','Suggested action'],['deadline','Due']]));
          const brief=node('p','Preparing your AI work brief…',{class:'result'});result.prepend(brief);
          let job;
          for(let attempt=0;attempt<40;attempt++){if(!this.isConnected||generation!==this.generation)return;await new Promise(resolve=>setTimeout(resolve,1500));job=await request('/api/assistant/jobs/'+queued.id);if(['completed','failed'].includes(job.status))break;}
          if(job?.status==='failed')throw new Error(job.error||'AI brief could not be generated.');
          if(job?.status!=='completed')throw new Error('The brief is still processing. Try again shortly.');
          if(!this.isConnected||generation!==this.generation)return;
          brief.textContent=job.result;
        } catch(error){if(generation===this.generation)result.replaceChildren(node('p',error.message))}
        finally{if(generation===this.generation)button.disabled=false;}
      };
    }
  }
  class Insights extends HTMLElement {
    constructor() { super(); this.attachShadow({mode: 'open'}); this.generation = 0; }
    connectedCallback() { this.render(); }
    disconnectedCallback() { this.generation++; }
    render() {
      this.shadowRoot.replaceChildren(); style(this.shadowRoot);
      const root = node('section', null, {class: 'tools'}), result = node('div', null, {'aria-live': 'polite'});
      root.append(node('h2', this.getAttribute('mode') === 'dashboard' ? 'Action queue · all accessible records' : 'Performance reports'));
      this.shadowRoot.append(root);
      const form = node('form', null, {class: 'toolbar'});
      if (this.getAttribute('mode') !== 'dashboard') {
        for (const [key, label] of [['from', 'Created from'], ['to', 'Created through']]) {
          const field = node('label', label), input = node('input', null, {type: 'date', name: key}); field.append(input); form.append(field);
        }
      }
      const submit = node('button', 'Refresh', {type: 'submit', class: 'secondary'}); form.append(submit); root.append(form, result);
      const load = async () => {
        const generation = ++this.generation; submit.disabled = true; result.replaceChildren(node('p', 'Loading all accessible records…'));
        try {
          const query = new URLSearchParams(new FormData(form)); const out = await request('/api/insights?' + query);
          if (!this.isConnected || generation !== this.generation) return;
          result.replaceChildren();
          if (this.getAttribute('mode') !== 'dashboard') {
            result.append(node('p', out.cohort));
            result.append(table('Lead conversion by source', out.sources, [['source','Source'],['leads','Leads'],['converted','Converted'],['rate','Conversion %']]));
            result.append(table('Enrolments by school and intake', out.enrollment, [['school','School'],['intake','Intake'],['students','Students'],['enrolled','Enrolled']]));
            result.append(table('Agent performance', out.agents, [['agent','Agent'],['students','Students'],['enrolled','Enrolled']]));
            result.append(table('Recorded case stage duration', out.stages, [['stage','Stage'],['visits','Visits'],['averageHours','Average hours'],['hours','Total hours']]));
          }
          result.append(table('Due and overdue actions', out.nextActions, [['record','Record'],['action','Next action'],['deadline','Deadline']]));
        } catch (error) { if (generation === this.generation) result.replaceChildren(node('p', error.message)); }
        finally { if (generation === this.generation) submit.disabled = false; }
      };
      form.onsubmit = event => { event.preventDefault(); load(); }; load();
    }
  }
  function focusKey(user, org) { return 'ecc-navigation-focus:' + org + ':' + user; }
  function readFocus(key, fallback = 'adviser') {
    try { const value = localStorage.getItem(key); return ['adviser','finance','management'].includes(value) ? value : fallback; } catch { return fallback; }
  }
  class NavigationFocus extends HTMLElement {
    connectedCallback() {
      if (this.firstChild) return;
      const sidebar = this.closest('.sidebar'), key = focusKey(this.getAttribute('user-id'), this.getAttribute('org-id'));
      const label = node('label', 'Navigation focus', {class:'navigation-focus'}), select = node('select', null, {'aria-label':'Navigation focus'});
      for (const [value, text] of [['adviser','Advising'],['finance','Finance'],['management','Management']]) select.append(node('option', text, {value}));
      const apply = () => {
        if (sidebar) sidebar.dataset.focus = select.value;
        document.documentElement.dataset.navFocus = select.value;
        window.dispatchEvent(new CustomEvent('ecc-navigation-focus', {detail:select.value}));
      };
      select.value = readFocus(key, this.getAttribute('default-focus') || 'adviser');
      select.onchange = () => { try { localStorage.setItem(key, select.value); } catch {} apply(); };
      label.append(select); this.append(label); apply();
    }
  }
  class MyWork extends HTMLElement {
    constructor() { super(); this.attachShadow({mode:'open'}); this.generation = 0; }
    connectedCallback() {
      style(this.shadowRoot);
      const root = node('section', null, {class:'tools daily-work'}), result = node('div', null, {'aria-live':'polite'});
      const toolbar = node('form', null, {class:'toolbar'}), label = node('label','Show work'), scope = node('select', null, {name:'scope', 'aria-label':'Work assignment'});
      for (const [value,text] of [['mine','Mine and unassigned'],['unassigned','Unassigned'],['all','All accessible work']]) scope.append(node('option',text,{value}));
      label.append(scope); const refresh = node('button','Refresh work',{type:'submit',class:'secondary'});
      toolbar.append(label,refresh); root.append(node('h2','My work today'),node('p','Applications, follow-ups and tasks that need your attention. Unassigned work is included so it does not get missed.'),toolbar,result);this.shadowRoot.append(root);
      let offset=0;
      this.load = async () => {
        const generation=++this.generation; refresh.disabled=true;result.replaceChildren(node('p','Loading your work…'));
        try {
          const focus=document.documentElement.dataset.navFocus||'adviser';
          const out=await request('/api/my-work?'+new URLSearchParams({scope:scope.value,focus,offset:String(offset),limit:'25'}));
          if(!this.isConnected||generation!==this.generation)return;
          result.replaceChildren(node('p',`${out.today} · ${out.total} work items · full accessible dataset`));
          const counts=node('div',null,{class:'work-counts'});
          for(const [key,title] of [['applications','Applications'],['followups','Follow-ups'],['documents','Applications needing documents'],['deadlines','Due within 7 days / overdue'],['tasks','Tasks'],['handoffs','Pending handoffs'],['overdueHandoffs','Overdue handoffs'],['pendingApprovals','Receipts awaiting approval'],['invoices','Invoices requiring attention']]){
            if(['invoices','pendingApprovals'].includes(key)&&focus!=='finance')continue;
            const card=node('div',null,{class:'work-count'});card.append(node('strong',out.counts?.[key]||0),node('span',title));counts.append(card);
          }
          result.append(counts);
          if(!out.items?.length){result.append(node('p','No work items in this view. Choose All accessible work to check other assignments.'));return;}
          for(const item of out.items){
            const card=node('article',null,{class:'card work-item work-item--'+item.priority});
            card.append(node('a',item.name,{href:item.route,class:'work-item__title'}),node('p',item.reason),node('p',item.action||'Open the record and choose the next action.'));
            if(item.deadline)card.append(node('p','Due '+item.deadline));
            result.append(card);
          }
          const pager=node('div',null,{class:'actions'});pager.append(node('span',`${offset+1}–${Math.min(offset+25,out.total)} of ${out.total}`));
          for(const [text,next,enabled] of [['Previous',offset-25,offset>0],['Next',offset+25,offset+25<out.total]]){if(!enabled)continue;const button=node('button',text,{type:'button',class:'secondary'});button.onclick=()=>{offset=next;this.load()};pager.append(button)}result.append(pager);
        } catch(error){if(generation===this.generation){result.replaceChildren(node('p',error.message));}}
        finally{if(generation===this.generation)refresh.disabled=false;}
      };
      toolbar.onsubmit=event=>{event.preventDefault();offset=0;this.load()};scope.onchange=()=>{offset=0;this.load()};
      this.focusChanged=()=>{offset=0;this.load()};window.addEventListener('ecc-navigation-focus',this.focusChanged);window.addEventListener('crm:record-changed',this.focusChanged);this.load();
    }
    disconnectedCallback(){this.generation++;window.removeEventListener('ecc-navigation-focus',this.focusChanged);window.removeEventListener('crm:record-changed',this.focusChanged);this.shadowRoot.replaceChildren();}
  }
  class StudentJourney extends HTMLElement {
    static get observedAttributes(){return ['record-id'];}
    constructor(){super();this.attachShadow({mode:'open'});this.generation=0;}
    connectedCallback(){this.changed=()=>this.render();window.addEventListener('crm:record-changed',this.changed);this.render();}
    attributeChangedCallback(){if(this.isConnected)this.render();}
    disconnectedCallback(){this.generation++;window.removeEventListener('crm:record-changed',this.changed);}
    async render(){
      const generation=++this.generation;this.shadowRoot.replaceChildren();style(this.shadowRoot);
      const root=node('section',null,{class:'tools journey'}),result=node('div',null,{'aria-live':'polite'}),refresh=node('button','Refresh journey',{type:'button',class:'secondary'});
      refresh.onclick=()=>this.render();root.append(node('h2','Student journey'),refresh,result);this.shadowRoot.append(root);result.append(node('p','Loading applications, documents, messages and payments…'));
      try{
        const out=await request('/api/students/'+encodeURIComponent(this.getAttribute('record-id'))+'/journey');if(!this.isConnected||generation!==this.generation)return;
        result.replaceChildren(node('p',`${out.verified} of ${out.required} required documents verified · ${out.student?.applicationStage||'Application stage not set'} · ${out.student?.intake||'Intake not set'}`));
        const applications=out.applications||[];
        if(!applications.length)result.append(node('p','No linked applications yet. Use Application tools & create case to begin.'));
        for(const app of applications){
          const card=node('article',null,{class:'card'});card.append(node('a',app.caseNumber,{href:'/cases/'+app.id}),node('p',`Stage: ${app.currentStage} · Owner: ${app.assignedOfficer||'Unassigned'}`),node('p','Next action: '+(app.nextAction||'Choose the next staff action')),node('p','Deadline: '+(app.nextDeadline||'Not set')));
          const links=node('div',null,{class:'links'});for(const [tab,label] of [['documents','Review documents'],['conversations','Messages'],['workflows','Application tools']])links.append(node('a',label,{href:'/cases/'+app.id+'#record-tools?tab='+tab}));card.append(links);const handoff=node('crm-case-handoff',null,{'record-id':app.id});card.append(node('crm-application-blockers',null,{'record-id':app.id}),handoff);result.append(card);
        }
        const sections=[
          ['Documents',(out.documents||[]).map(d=>({record:d.docName,status:d.status,required:d.required?'Required':'Optional',url:'/cases/'+d.caseId+'#record-tools?tab=documents'})),[['record','Document'],['status','Status'],['required','Requirement']]],
          ['Invoices',(out.invoices||[]).map(i=>({record:i.invoiceNumber,currency:i.currency||'USD',balance:i.balance,received:i.amountReceived,url:'/invoices/'+i.id})),[['record','Invoice'],['currency','Currency'],['balance','Balance'],['received','Received']]],
          ['Recorded payments',(out.payments||[]).map(p=>({record:p.reference||p.id,amount:p.amount,currency:p.currency||'USD',status:p.status||'Recorded',date:p.paidOn,url:'/invoices/'+p.invoiceId})),[['record','Reference'],['amount','Amount'],['currency','Currency'],['status','Status'],['date','Paid on']]]
        ];
        for(const [title,rows,columns] of sections){const details=node('details',null,{class:'journey-section'});details.append(node('summary',`${title} (${rows.length})`),table(title,rows,columns));result.append(details)}
        const messages=node('details',null,{class:'journey-section'});messages.append(node('summary',`Application messages (${out.messageTotal||0})`));
        if(!out.messages?.length)messages.append(node('p','No messages explicitly linked to these applications.'));
        for(const m of out.messages||[]){const card=node('article',null,{class:'card'});card.append(node('strong',m.subject||m.channel),node('p',`${m.channel} · ${m.direction} · ${m.createdAt}`),node('p',m.body),node('a','Open conversation',{href:'/cases/'+m.caseId+'#record-tools?tab=conversations'}));messages.append(card)}
        if(out.messageTotal>25)messages.append(node('p','Showing the latest 25 messages. Open an application conversation for more.'));result.append(messages);
      }catch(error){if(generation===this.generation)result.replaceChildren(node('p',error.message));}
    }
  }
  class FormQuality extends HTMLElement {
    static get observedAttributes(){return ['entity','record-id','name','studentcode','schoolid','program','email','phone','followupdate','status','studentid','clientid','nextaction','nextdeadline','currentstage'];}
    constructor(){super();this.attachShadow({mode:'open'});this.generation=0;}
    connectedCallback(){this.shadowRoot.replaceChildren();style(this.shadowRoot);this.result=node('div',null,{class:'quality-hints',role:'status','aria-live':'polite'});this.shadowRoot.append(this.result);this.schedule();}
    attributeChangedCallback(){if(this.isConnected)this.schedule();}
    disconnectedCallback(){clearTimeout(this.timer);this.generation++;}
    schedule(){
      clearTimeout(this.timer);const generation=++this.generation;
      this.timer=setTimeout(async()=>{
        const payload={entity:this.getAttribute('entity'),id:this.getAttribute('record-id')||''};
        for(const key of ['name','studentCode','schoolId','program','email','phone','followUpDate','status','studentId','clientId','nextAction','nextDeadline','currentStage'])payload[key]=this.getAttribute(key)||'';
        try{
          const out=await request('/api/form-quality',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)});
          if(!this.isConnected||generation!==this.generation)return;this.result.replaceChildren();
          if(!out.findings?.length)return;
          this.result.append(node('strong','Before you save'));const list=node('ul');
          for(const finding of out.findings){const item=node('li',finding.title+': '+finding.detail);if(finding.route){item.append(document.createTextNode(' '),node('a','Review record',{href:finding.route,target:'_blank',rel:'noopener'}))}list.append(item)}this.result.append(list);
        }catch(error){if(generation===this.generation&&this.result)this.result.replaceChildren(node('p','Could not check data quality. '+error.message));}
      },400);
    }
  }
  class IntegrationHealth extends HTMLElement {
    constructor(){super();this.attachShadow({mode:'open'});this.generation=0;}
    connectedCallback(){this.render();}
    disconnectedCallback(){this.generation++;}
    async render(){
      const generation=++this.generation;this.shadowRoot.replaceChildren();style(this.shadowRoot);
      const root=node('section',null,{class:'tools'}),result=node('div',null,{'aria-live':'polite'}),refresh=node('button','Refresh status',{class:'secondary',type:'button'});refresh.onclick=()=>this.render();
      root.append(node('h2','Integration connections'),node('p','Configured means settings exist. Connected means the most recent connection check succeeded. Checks do not send messages or create payments.'),refresh,result);this.shadowRoot.append(root);
      try{
        const out=await request('/api/integrations/status');if(!this.isConnected||generation!==this.generation)return;
        for(const integration of out.integrations||[]){
          const card=node('article',null,{class:'card integration-health'}),status=node('strong',integration.state.replaceAll('_',' ')),message=node('p',integration.message),checked=node('p',integration.checkedAt?'Last checked: '+integration.checkedAt:'Connection has not been tested.');
          card.append(node('h3',integration.name),status,message,checked);
          if(out.canTest){const button=node('button','Test connection',{type:'button',class:'secondary'});button.disabled=['needs_configuration','disabled'].includes(integration.state);button.onclick=async()=>{button.disabled=true;message.textContent='Testing connection…';try{const check=await request('/api/admin/integrations/'+integration.id+'/test',{method:'POST'});if(!this.isConnected||generation!==this.generation)return;status.textContent=check.state.replaceAll('_',' ');message.textContent=check.message;checked.textContent=check.checkedAt?'Last checked: '+check.checkedAt:'Connection has not been tested.';}catch(error){if(generation===this.generation)message.textContent=error.message;}finally{button.disabled=false}};card.append(button)}
          result.append(card);
        }
      }catch(error){if(generation===this.generation)result.replaceChildren(node('p',error.message));}
    }
  }
  customElements.define('crm-nav-focus',NavigationFocus);
  customElements.define('crm-my-work',MyWork);
  customElements.define('crm-student-journey',StudentJourney);
  customElements.define('crm-form-quality',FormQuality);
  customElements.define('crm-integration-health',IntegrationHealth);
  class RecentChanges extends HTMLElement {
    constructor(){super();this.attachShadow({mode:'open'});this.generation=0;}
    connectedCallback(){this.load();}
    disconnectedCallback(){this.generation++;}
    async load(){const generation=++this.generation;this.shadowRoot.replaceChildren();style(this.shadowRoot);const root=node('section',null,{class:'tools'}),result=node('div',null,{'aria-live':'polite'}),refresh=node('button','Refresh changes',{type:'button',class:'secondary'});refresh.onclick=()=>this.load();root.append(refresh,result);this.shadowRoot.append(root);try{const out=await request('/api/audit?limit=8&offset=0');if(!this.isConnected||generation!==this.generation)return;result.append(table('Recent workspace changes',out.events||[],[['label','Record'],['action','Change'],['actor','By'],['occurredAt','When']]),node('p',`${out.total||0} accessible changes in history`));}catch(error){if(generation===this.generation)result.textContent=error.message;}}
  }
  customElements.define('crm-recent-changes',RecentChanges);
  customElements.define('crm-workspace', Workspace);
  customElements.define('crm-record-tools', RecordTools);
  customElements.define('crm-record-photo', RecordPhoto);
  customElements.define('crm-insights', Insights);
  customElements.define('crm-ai-productivity', AIProductivity);
  const updateTheme = () => {
    const theme=localStorage.getItem('theme') || 'light';
    for (const element of document.querySelectorAll('crm-workspace,crm-record-tools,crm-insights,crm-ai-productivity,crm-ai-chat,crm-administration,crm-student-journey,crm-my-work,crm-recent-changes,crm-form-quality,crm-integration-health')) { if(element.dataset.theme!==theme)element.dataset.theme=theme; }
  };
  new MutationObserver(updateTheme).observe(document.getElementById('app'), {childList:true, subtree:true});
  window.addEventListener('crm:theme-changed', updateTheme);
  document.addEventListener('click', event => {
    const link=event.target.closest('a[href^="#record-tools?"]');
    if(!link)return;
    const host=document.querySelector('crm-record-tools');if(!host)return;
    event.preventDefault();event.stopPropagation();
    const params=new URLSearchParams(link.getAttribute('href').split('?')[1]);
    const tab=params.get('tab');
    host.dispatchEvent(new CustomEvent('crm:select-tab',{detail:{tab,document:params.get('document')}}));
    host.scrollIntoView({block:'start',behavior:'smooth'});
  },true);
})();
