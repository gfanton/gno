var s=class n{DOM;static FEEDBACK_DELAY=1500;btnClicked=null;btnClickedIcons=[];static SELECTORS={button:"[data-copy-btn]",icon:"[data-copy-icon] > use",content:t=>`[data-copy-content="${t}"]`};constructor(){this.DOM={el:document.querySelector("main")},this.DOM.el?this.init():console.warn("Copy: Main container not found.")}init(){this.bindEvents()}bindEvents(){this.DOM.el?.addEventListener("click",this.handleClick.bind(this))}handleClick(t){let e=t.target.closest(n.SELECTORS.button);if(!e)return;this.btnClicked=e,this.btnClickedIcons=Array.from(e.querySelectorAll(n.SELECTORS.icon));let i=e.getAttribute("data-copy-btn");if(!i){console.warn("Copy: No content ID found on the button.");return}let c=this.DOM.el?.querySelector(n.SELECTORS.content(i));c?this.copyToClipboard(c):console.warn(`Copy: No content found for ID "${i}".`)}sanitizeContent(t){let o=t.innerHTML.replace(/<span class="chroma-ln">.*?<\/span>/g,""),e=document.createElement("div");return e.innerHTML=o,e.textContent?.trim()||""}toggleIcons(){this.btnClickedIcons.forEach(t=>{t.classList.toggle("hidden")})}showFeedback(){this.btnClicked&&(this.toggleIcons(),window.setTimeout(()=>{this.toggleIcons()},n.FEEDBACK_DELAY))}async copyToClipboard(t){let o=this.sanitizeContent(t);if(!navigator.clipboard){console.error("Copy: Clipboard API is not supported in this browser."),this.showFeedback();return}try{await navigator.clipboard.writeText(o),console.info("Copy: Text copied successfully."),this.showFeedback()}catch(e){console.error("Copy: Error while copying text.",e),this.showFeedback()}}},r=()=>new s;export{r as default};
