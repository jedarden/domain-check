// Domain Check JS
(function(){'use strict';
document.documentElement.classList.add('js');
document.querySelectorAll('.tld-options input[type="checkbox"]').forEach(function(c){c.disabled=false});
var K='domain-check-history',M=20;
function $(s,p){return(p||document).querySelector(s)}
function $$(s,p){return(p||document).querySelectorAll(s)}
function esc(s){var d=document.createElement('div');d.textContent=s;return d.innerHTML}
function db(fn,ms){var t;return function(){var a=arguments,c=this;clearTimeout(t);t=setTimeout(function(){fn.apply(c,a)},ms)}}
function ld(){try{return JSON.parse(localStorage.getItem(K))||[]}catch(e){return[]}}
function sv(o){var h=ld().filter(function(e){return e.d!==o.d});h.unshift(o);h.length>M&&(h.length=M);try{localStorage.setItem(K,JSON.stringify(h))}catch(e){}}
function gc(){var s=$('.result-section');if(s)s.remove();s=document.createElement('section');s.className='result-section';$('main').appendChild(s);return s}
function card(r){var a=r.available,v=a?'available':'taken';sv({d:r.domain,a:a});
var h='<div class="result-card '+v+'"><h2 class="domain-name">'+esc(r.domain)+'</h2><p class="status '+v+'">'+(a?'Available':'Taken')+'</p><p class="meta">Checked via '+esc(r.source||'?')+', '+(r.duration_ms||'?')+'ms'+(r.cached?' (cached)':'')+'</p>';
if(a)h+='<button class="copy-btn" data-domain="'+esc(r.domain)+'">Copy domain</button>';
h+='</div>';if(!a&&r.registration){var g=r.registration;h+='<div class="registration-details"><h3>Registration Details</h3><dl>';
if(g.registrar)h+='<dt>Registrar</dt><dd>'+esc(g.registrar)+'</dd>';if(g.created)h+='<dt>Registered</dt><dd>'+esc(g.created)+'</dd>';if(g.expires)h+='<dt>Expires</dt><dd>'+esc(g.expires)+'</dd>';
if(g.nameservers)h+='<dt>Nameservers</dt><dd><ul class="nameservers">'+g.nameservers.map(function(n){return'<li>'+esc(n)+'</li>'}).join('')+'</ul></dd>';h+='</dl></div>'}return h}
function r1(r){var s=gc();s.innerHTML=card(r)+'<div class="api-link"><a href="/api/v1/check?d='+encodeURIComponent(r.domain)+'">View JSON</a></div>';bC()}
function rM(d){if(!d.results||!d.results.length)return;r1(d.results[0].result);
if(d.results.length>1){var s=gc(),ul='<ul class="alt-tld-list">';
d.results.slice(1).forEach(function(r){var c=r.error?'error':(r.result&&r.result.available?'available':'taken'),t=r.error?'?':(r.result&&r.result.available?'available':'taken');
if(r.result&&r.result.available)sv({d:r.domain,a:true});ul+='<li class="'+c+'"><a href="/check?d='+encodeURIComponent(r.domain)+'">.'+esc(r.tld)+'</a><span class="alt-status">'+t+'</span></li>'});
s.insertAdjacentHTML('beforeend','<div class="also-check"><h3>Also checked</h3>'+ul+'</ul></div>')}}
function rE(m,d){var s=gc();s.className='result-section error';s.innerHTML='<div class="result-card error"><p class="error-message">'+esc(m)+'</p>'+(d?'<p class="error-detail">'+esc(d)+'</p>':'')+'</div>'}
function chk(d){fetch('/api/v1/check?d='+encodeURIComponent(d)).then(function(r){return r.json()}).then(function(d){d.error?rE(d.message||d.error):r1(d)}).catch(function(){rE('Network error')})}
function mChk(n,t){fetch('/api/v1/check?d='+encodeURIComponent(n)+'&tlds='+t.join(',')).then(function(r){return r.json()}).then(function(d){d.error?rE(d.message||d.error):rM(d)}).catch(function(){rE('Network error')})}
function bC(){$$('.copy-btn').forEach(function(b){b.onclick=function(e){e.preventDefault();var d=this.getAttribute('data-domain'),o=this.textContent,cp=function(){b.textContent='Copied!';b.classList.add('copied');setTimeout(function(){b.textContent=o;b.classList.remove('copied')},1500)};
if(navigator.clipboard)navigator.clipboard.writeText(d).then(cp);else{var t=document.createElement('textarea');t.value=d;t.style.cssText='position:fixed;opacity:0';document.body.appendChild(t);t.select();document.execCommand('copy');document.body.removeChild(t);cp()}}})}
function parse(v){v=v.trim().toLowerCase().replace(/\.$/,'');if(!v)return null;var i=v.lastIndexOf('.');return i<0?{name:v,tld:null}:{name:v.substring(0,i),tld:v.substring(i+1)}}
function gT(){var t=[];$$('.tld-options input[type="checkbox"]:checked').forEach(function(c){t.push(c.value)});return t}
var inp=$('#domain-input'),frm=$('.search-form');
if(inp&&frm){
frm.addEventListener('submit',function(e){e.preventDefault();var v=inp.value.trim();if(!v)return;var t=gT();if(t.length){var p=parse(v);mChk(p&&p.tld?p.name:v.split('.')[0],t)}else chk(v)});
inp.addEventListener('input',db(function(){var p=parse(inp.value);if(!p||p.name.length<3||!p.tld)return;chk(p.name+'.'+p.tld)},300));
inp.addEventListener('keydown',function(e){if(e.key==='Tab'&&!e.shiftKey&&inp.value.trim()){var cs=$$('.tld-options input[type="checkbox"]');if(!cs.length)return;e.preventDefault();var u=[];cs.forEach(function(c){if(!c.checked)u.push(c)});if(u.length)u[0].checked=true;else cs.forEach(function(c){c.checked=false})}});
}bC();})();
