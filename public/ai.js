'use strict';
// Keep provider content as text. Every proposed mutation needs its own review.
(function () {
  const node=(tag,text,attrs={})=>{const el=document.createElement(tag);if(text!=null)el.textContent=String(text);for(const[k,v]of Object.entries(attrs))el.setAttribute(k,v);return el};
  async function request(path,options={}){
    let response=await fetch(path,{credentials:'same-origin',...options});
    if(response.status===401){const refreshed=await fetch('/api/session/refresh',{method:'POST',credentials:'same-origin'});if(refreshed.ok)response=await fetch(path,{credentials:'same-origin',...options});}
    const out=await response.json();if(!response.ok)throw new Error(out.error||'Request failed. Try again.');return out;
  }
  const send=(path,body)=>request(path,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body||{})});
  class Adviser extends HTMLElement {
    constructor(){super();this.attachShadow({mode:'open'});this.generation=0;this.threadId='';this.version=0;}
    connectedCallback(){
      this.shadowRoot.replaceChildren();for(const href of ['/public/center.css','/public/components.css'])this.shadowRoot.append(node('link',null,{rel:'stylesheet',href}));
      const launch=node('button','Ask ECC AI',{type:'button',class:'ai-launch','aria-expanded':'false'}),panel=node('section',null,{class:'ai-panel','aria-label':'Ask ECC AI',hidden:''}),header=node('header'),close=node('button','Close',{type:'button',class:'secondary'}),fresh=node('button','New chat',{type:'button',class:'secondary'}),history=node('select',null,{'aria-label':'Saved conversations'}),log=node('div',null,{class:'ai-log','aria-live':'polite'}),notice=node('p','',{role:'status'}),form=node('form'),label=node('label','Your question'),question=node('textarea',null,{required:'',maxlength:'12000'}),submit=node('button','Ask',{type:'submit'});
      header.append(node('h2','Ask ECC AI'),close);label.append(question);form.append(label,submit);
      panel.append(header,node('p','AI replies may contain errors. Accessible record context and your question are sent to the configured provider; conversations are saved in this workspace. Review every proposed action before confirming it.',{class:'ai-disclosure'}),fresh,history,log,notice,form);this.shadowRoot.append(launch,panel);
      const hide=()=>{panel.hidden=true;launch.hidden=false;launch.setAttribute('aria-expanded','false');launch.focus()};close.onclick=hide;
      panel.onkeydown=event=>{if(event.key==='Escape'){event.preventDefault();hide()}};
      fresh.onclick=()=>{this.generation++;this.threadId='';this.version=0;history.value='';log.replaceChildren();notice.textContent='';question.focus()};
      const listHistory=async()=>{try{const out=await request('/api/assistant/threads');if(!this.isConnected)return;history.replaceChildren(node('option','New conversation',{value:''}));for(const thread of out.threads||[])history.append(node('option',thread.title,{value:thread.id}));history.value=this.threadId}catch(error){notice.textContent=error.message}};
      const renderAction=action=>{
        const card=node('section',null,{class:'card'});card.append(node('strong',action.kind.replaceAll('_',' ')+' · '+action.label));
        for(const[key,value]of Object.entries(action.fields||{})){const previous=action.before?.[key];card.append(node('p',key+': '+(previous===undefined?'':String(previous)+' → ')+String(value)))}
        const status=node('p',action.status==='pending'?'Proposed — not executed.':action.status);card.append(status);
        if(action.status==='pending'&&this.canAct)for(const[label,endpoint]of [['Confirm action','confirm'],['Cancel proposal','reject']]){const button=node('button',label,{type:'button',class:'secondary'});button.onclick=async()=>{const controls=card.querySelectorAll('button');controls.forEach(b=>b.disabled=true);try{await send('/api/assistant/actions/'+encodeURIComponent(action.id)+'/'+endpoint);if(!this.isConnected)return;status.textContent=endpoint==='confirm'?'Confirmed successfully.':'Proposal cancelled.';if(endpoint==='confirm')window.dispatchEvent(new CustomEvent('crm:record-changed',{detail:action.recordId||''}))}catch(error){status.textContent=error.message;controls.forEach(b=>b.disabled=false)}};card.append(button)}
        log.append(card);
      };
      const renderMessage=(role,text,sources=[],actions=[])=>{
        log.append(node('p',text,{class:'ai-message'+(role==='user'?' ai-message--user':'')}));
        const links=node('div',null,{class:'ai-sources'});for(const source of sources){if(source.url?.startsWith('/')&&!source.url.startsWith('//'))links.append(node('a',source.title,{href:source.url}));else links.append(node('span',source.title))}log.append(links);for(const action of actions)renderAction(action);
      };
      history.onchange=async()=>{if(!history.value){fresh.click();return}const generation=++this.generation;try{const out=await request('/api/assistant/threads/'+encodeURIComponent(history.value));if(!this.isConnected||generation!==this.generation)return;this.threadId=out.thread.id;this.version=out.thread.version;log.replaceChildren();for(const message of out.messages||[])renderMessage(message.role,message.content,message.sources,message.actions)}catch(error){notice.textContent=error.message}};
      launch.onclick=async()=>{panel.hidden=false;launch.hidden=true;launch.setAttribute('aria-expanded','true');question.focus();try{const capabilities=await request('/api/assistant/capabilities');if(!this.isConnected)return;this.canAct=capabilities.actions;submit.disabled=!capabilities.enabled;notice.textContent=capabilities.enabled?'':'AI is not enabled. Ask an administrator to configure and enable it in Workspace tools → Integrations.';await listHistory()}catch(error){notice.textContent=error.message}};
      form.onsubmit=async event=>{event.preventDefault();const generation=++this.generation,text=question.value;submit.disabled=true;fresh.disabled=true;history.disabled=true;notice.textContent='Preparing an answer…';
        const segments=location.pathname.split('/').filter(Boolean);const payload={message:text,threadId:this.threadId,version:this.version};if(!this.threadId&&segments.length===2&&['contacts','students','cases','leads','deals','schools','agents','partners','invoices'].includes(segments[0])){payload.entity=segments[0];payload.id=segments[1]}
        try{const out=await send('/api/assistant/chat',payload);if(!this.isConnected||generation!==this.generation)return;this.threadId=out.threadId;this.version=out.version;renderMessage('user',text);renderMessage('assistant',out.reply,out.sources,out.actions);question.value='';notice.textContent='';await listHistory()}catch(error){if(generation===this.generation)notice.textContent=error.message}finally{if(generation===this.generation){submit.disabled=false;fresh.disabled=false;history.disabled=false}}
      };
    }
    disconnectedCallback(){this.generation++;this.threadId='';this.version=0;this.shadowRoot.replaceChildren();}
  }
  customElements.define('crm-ai-chat',Adviser);
})();
