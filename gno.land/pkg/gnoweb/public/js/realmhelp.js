var s=class a{DOM;funcList;static SELECTORS={container:"#help",func:"[data-func]",addressInput:"[data-role='help-input-addr']",cmdModeSelect:"[data-role='help-select-mode']"};constructor(){this.DOM={el:document.querySelector(a.SELECTORS.container),funcs:[],addressInput:null,cmdModeSelect:null},this.funcList=[],this.DOM.el?this.init():console.warn("Help: Main container not found.")}init(){let{el:e}=this.DOM;e&&(this.DOM.funcs=Array.from(e.querySelectorAll(a.SELECTORS.func)),this.DOM.addressInput=e.querySelector(a.SELECTORS.addressInput),this.DOM.cmdModeSelect=e.querySelector(a.SELECTORS.cmdModeSelect),console.log(this.DOM),this.funcList=this.DOM.funcs.map(t=>new r(t)),this.bindEvents())}bindEvents(){let{addressInput:e,cmdModeSelect:t}=this.DOM;e?.addEventListener("input",()=>{this.funcList.forEach(n=>n.updateAddr(e.value))}),t?.addEventListener("change",n=>{let d=n.target;this.funcList.forEach(l=>l.updateMode(d.value))})}},r=class a{DOM;funcName;static SELECTORS={address:"[data-role='help-code-address']",args:"[data-role='help-code-args']",mode:"[data-code-mode]",paramInput:"[data-role='help-param-input']"};constructor(e){this.DOM={el:e,addrs:Array.from(e.querySelectorAll(a.SELECTORS.address)),args:Array.from(e.querySelectorAll(a.SELECTORS.args)),modes:Array.from(e.querySelectorAll(a.SELECTORS.mode))},this.funcName=e.dataset.func||null,this.bindEvents()}bindEvents(){this.DOM.el.addEventListener("input",e=>{let t=e.target;t.dataset.role==="help-param-input"&&this.updateArg(t.dataset.param||"",t.value)})}updateArg(e,t){this.DOM.args.filter(n=>n.dataset.arg===e).forEach(n=>{n.textContent=t.trim()||""})}updateAddr(e){this.DOM.addrs.forEach(t=>{t.textContent=e.trim()||"ADDRESS"})}updateMode(e){this.DOM.modes.forEach(t=>{let n=t.dataset.codeMode===e;t.className=n?"inline":"hidden",t.dataset.copyContent=n?`help-cmd-${this.funcName}`:""})}},i=()=>new s;export{i as default};
