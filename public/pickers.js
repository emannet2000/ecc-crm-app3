'use strict';
(() => {
  let nextId=0;
  class RecordPicker extends HTMLElement {
    static formAssociated=true;
    static get observedAttributes(){return ['entity','value','label','disabled','required'];}
    constructor(){super();this.attachShadow({mode:'open'});this.internals=this.attachInternals?.();this.generation=0;this.offset=0;this.selected='';}
    get value(){return this.getAttribute('value')||'';}
    set value(value){this.setAttribute('value',value||'');}
    get disabled(){return this.hasAttribute('disabled');}
    set disabled(value){this.toggleAttribute('disabled',!!value);}
    connectedCallback(){this.mount();}
    disconnectedCallback(){clearTimeout(this.timer);this.generation++;}
    attributeChangedCallback(name){if(!this.isConnected)return;if(name==='entity')this.mount();else if(name==='value')this.hydrate();else if(name==='label')this.search?.setAttribute('aria-label',this.getAttribute('label')||'Find a record');else this.validity();}
    validity(){this.internals?.setFormValue(this.value);this.internals?.setValidity(this.hasAttribute('required')&&!this.value?{valueMissing:true}:{},this.value?'':'Choose a record',this.search);if(this.search)this.search.disabled=this.disabled;if(this.clear)this.clear.disabled=this.disabled;for(const button of this.results?.querySelectorAll('button')||[])button.disabled=this.disabled;}
    mount(){
      clearTimeout(this.timer);this.generation++;this.shadowRoot.replaceChildren();
      const make=(tag,text,attrs={})=>{const e=document.createElement(tag);if(text!=null)e.textContent=text;for(const[k,v]of Object.entries(attrs))e.setAttribute(k,v);return e};
      const style=make('style',`:host{display:block;min-width:0}input,button{font:inherit;box-sizing:border-box;min-height:44px;border:1px solid var(--border,#ccd3df);border-radius:8px;padding:9px;background:var(--surface,#fff);color:var(--text,#172132)}input{width:100%}button{cursor:pointer}button:focus-visible,input:focus-visible{outline:3px solid var(--accent,#3266dd);outline-offset:2px}.selected{display:flex;gap:8px;align-items:center;overflow-wrap:anywhere;margin-bottom:8px}.selected span{flex:1}.results{display:grid;gap:6px;margin-top:8px;max-height:260px;overflow:auto}.results button{text-align:left}.paging{display:flex;gap:8px;margin-top:8px}.status{font-size:12px;color:var(--text-muted,#526078)}[hidden]{display:none}`);
      const listId='record-picker-'+(++nextId),selected=make('div',null,{class:'selected'});this.caption=make('span','No record selected');this.clear=make('button','Clear',{type:'button','aria-label':'Clear '+(this.getAttribute('label')||'record')});selected.append(this.caption,this.clear);
      this.search=make('input',null,{type:'search',placeholder:'Search all accessible records…','aria-label':this.getAttribute('label')||'Find a record','aria-controls':listId,'aria-expanded':'false',autocomplete:'off'});
      this.status=make('p','',{class:'status',role:'status','aria-live':'polite'});this.results=make('div',null,{class:'results',id:listId});this.paging=make('div',null,{class:'paging'});
      this.shadowRoot.append(style,selected,this.search,this.status,this.results,this.paging);this.validity();this.hydrate();
      this.clear.onclick=()=>this.choose('','No record selected');
      this.search.oninput=()=>{clearTimeout(this.timer);this.generation++;this.offset=0;this.timer=setTimeout(()=>this.load(),250)};
      this.search.onfocus=()=>{if(!this.results.children.length)this.load()};
      this.search.onkeydown=event=>{if(event.key==='ArrowDown'){event.preventDefault();this.results.querySelector('button')?.focus()}if(event.key==='Escape'){this.results.replaceChildren();this.paging.replaceChildren();this.search.setAttribute('aria-expanded','false')}};
      this.results.onkeydown=event=>{const buttons=[...this.results.querySelectorAll('button')],i=buttons.indexOf(this.shadowRoot.activeElement);if(['ArrowDown','ArrowUp','Escape'].includes(event.key)){event.preventDefault();if(event.key==='Escape')this.search.focus();else (buttons[i+(event.key==='ArrowDown'?1:-1)]||this.search).focus()}};
    }
    async get(url){const response=await fetch(url,{credentials:'same-origin'});const out=await response.json();if(!response.ok)throw new Error(out.error||'Could not load records. Try again.');return out;}
    label(record){const main=record.name||record.caseNumber||record.title||record.id;const detail=record.email||record.studentCode||record.clientName||record.program||'';return main+(detail?' · '+detail:'')+' · '+record.id;}
    choose(id,label){this.value=id;this.caption.textContent=label;this.search.value='';this.results.replaceChildren();this.paging.replaceChildren();this.status.textContent='';this.search.setAttribute('aria-expanded','false');this.dispatchEvent(new Event('input',{bubbles:true,composed:true}));this.dispatchEvent(new Event('change',{bubbles:true,composed:true}));this.search.focus();}
    async hydrate(){this.validity();const value=this.value,entity=this.getAttribute('entity');if(!value){this.caption.textContent='No record selected';return}this.caption.textContent='Selected record: '+value;try{const out=await this.get('/api/'+entity+'/'+encodeURIComponent(value));if(this.isConnected&&this.value===value&&this.getAttribute('entity')===entity){const record=Object.values(out).find(v=>v&&typeof v==='object'&&v.id===value);if(record)this.caption.textContent=this.label(record)}}catch(error){if(this.value===value)this.status.textContent=error.message+' The selection has been kept.'}}
    async load(){const generation=++this.generation,entity=this.getAttribute('entity');if(!['contacts','students','cases'].includes(entity))return;this.status.textContent='Searching…';this.results.replaceChildren();this.paging.replaceChildren();try{const out=await this.get('/api/'+entity+'?'+new URLSearchParams({q:this.search.value,limit:'20',offset:String(this.offset)}));if(!this.isConnected||generation!==this.generation)return;const records=out[entity]||[];this.status.textContent=records.length?`${this.offset+1}–${Math.min(this.offset+20,out.total)} of ${out.total} accessible records`:'No matching accessible records.';for(const record of records){const button=document.createElement('button');button.type='button';button.disabled=this.disabled;button.textContent=this.label(record);button.onclick=()=>this.choose(record.id,this.label(record));this.results.append(button)}this.search.setAttribute('aria-expanded',String(records.length>0));for(const[label,offset,enabled]of [['Previous',this.offset-20,this.offset>0],['Next',this.offset+20,this.offset+20<out.total]])if(enabled){const button=document.createElement('button');button.type='button';button.textContent=label;button.onclick=()=>{this.offset=offset;this.load()};this.paging.append(button)}}catch(error){if(generation!==this.generation)return;this.status.textContent=error.message;const retry=document.createElement('button');retry.type='button';retry.textContent='Retry search';retry.onclick=()=>this.load();this.paging.append(retry)}}
  }
  customElements.define('crm-record-picker',RecordPicker);
})();
