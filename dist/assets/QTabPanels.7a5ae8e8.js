var Ke=Object.defineProperty,je=Object.defineProperties;var Oe=Object.getOwnPropertyDescriptors;var Pe=Object.getOwnPropertySymbols;var ze=Object.prototype.hasOwnProperty,Ue=Object.prototype.propertyIsEnumerable;var qe=(e,a,i)=>a in e?Ke(e,a,{enumerable:!0,configurable:!0,writable:!0,value:i}):e[a]=i,W=(e,a)=>{for(var i in a||(a={}))ze.call(a,i)&&qe(e,i,a[i]);if(Pe)for(var i of Pe(a))Ue.call(a,i)&&qe(e,i,a[i]);return e},se=(e,a)=>je(e,Oe(a));import{O as ce,r as A,c as m,S as Ae,o as He,U as Xe,f as Ye,V as Ge,h as w,W as Le,X,Y as Ze,Z as Je,y as J,_ as et,g as ee,$ as tt,a as te,a0 as nt,a1 as at,w as U,a2 as ot,a3 as rt,a4 as it,b as ne,a5 as lt,a6 as H,a7 as ut,a8 as st,a9 as Z,aa as ve,p as xe,ab as de,n as ct,T as vt,ac as dt,ad as ft,ae as ht}from"./index.09575a90.js";import{c as bt}from"./use-key-composition.9168ed62.js";import{Q as mt,g as ke,s as Se}from"./touch.efddf660.js";import{u as fe,c as _e}from"./QDialog.707aab58.js";import{r as gt}from"./rtl.b51694b1.js";import{u as pt,a as yt}from"./focus-manager.a7562c02.js";import{c as Tt}from"./QList.9cbe6608.js";let Ct=0;const wt=["click","keydown"],Pt={icon:String,label:[Number,String],alert:[Boolean,String],alertIcon:String,name:{type:[Number,String],default:()=>`t_${Ct++}`},noCaps:Boolean,tabindex:[String,Number],disable:Boolean,contentClass:String,ripple:{type:[Boolean,Object],default:!0}};function qt(e,a,i,v){const r=Xe(Le,ce);if(r===ce)return console.error("QTab/QRouteTab component needs to be child of QTabs"),ce;const{proxy:n}=ee(),l=A(null),p=A(null),I=A(null),M=m(()=>e.disable===!0||e.ripple===!1?!1:Object.assign({keyCodes:[13,32],early:!0},e.ripple===!0?{}:e.ripple)),g=m(()=>r.currentModel.value===e.name),D=m(()=>"q-tab relative-position self-stretch flex flex-center text-center"+(g.value===!0?" q-tab--active"+(r.tabProps.value.activeClass?" "+r.tabProps.value.activeClass:"")+(r.tabProps.value.activeColor?` text-${r.tabProps.value.activeColor}`:"")+(r.tabProps.value.activeBgColor?` bg-${r.tabProps.value.activeBgColor}`:""):" q-tab--inactive")+(e.icon&&e.label&&r.tabProps.value.inlineLabel===!1?" q-tab--full":"")+(e.noCaps===!0||r.tabProps.value.noCaps===!0?" q-tab--no-caps":"")+(e.disable===!0?" disabled":" q-focusable q-hoverable cursor-pointer")+(v!==void 0?v.linkClass.value:"")),b=m(()=>"q-tab__content self-stretch flex-center relative-position q-anchor--skip non-selectable "+(r.tabProps.value.inlineLabel===!0?"row no-wrap q-tab__content--inline":"column")+(e.contentClass!==void 0?` ${e.contentClass}`:"")),y=m(()=>e.disable===!0||r.hasFocus.value===!0||g.value===!1&&r.hasActiveTab.value===!0?-1:e.tabindex||0);function q(u,x){if(x!==!0&&l.value!==null&&l.value.focus(),e.disable===!0){v!==void 0&&v.hasRouterLink.value===!0&&X(u);return}if(v===void 0){r.updateModel({name:e.name}),i("click",u);return}if(v.hasRouterLink.value===!0){const _=(P={})=>{let k;const $=P.to===void 0||tt(P.to,e.to)===!0?r.avoidRouteWatcher=bt():null;return v.navigateToRouterLink(u,se(W({},P),{returnRouterError:!0})).catch(E=>{k=E}).then(E=>{if($===r.avoidRouteWatcher&&(r.avoidRouteWatcher=!1,k===void 0&&(E===void 0||E.message.startsWith("Avoided redundant navigation")===!0)&&r.updateModel({name:e.name})),P.returnRouterError===!0)return k!==void 0?Promise.reject(k):E})};i("click",u,_),u.defaultPrevented!==!0&&_();return}i("click",u)}function L(u){Ze(u,[13,32])?q(u,!0):Je(u)!==!0&&u.keyCode>=35&&u.keyCode<=40&&u.altKey!==!0&&u.metaKey!==!0&&r.onKbdNavigate(u.keyCode,n.$el)===!0&&X(u),i("keydown",u)}function B(){const u=r.tabProps.value.narrowIndicator,x=[],_=w("div",{ref:I,class:["q-tab__indicator",r.tabProps.value.indicatorClass]});e.icon!==void 0&&x.push(w(J,{class:"q-tab__icon",name:e.icon})),e.label!==void 0&&x.push(w("div",{class:"q-tab__label"},e.label)),e.alert!==!1&&x.push(e.alertIcon!==void 0?w(J,{class:"q-tab__alert-icon",color:e.alert!==!0?e.alert:void 0,name:e.alertIcon}):w("div",{class:"q-tab__alert"+(e.alert!==!0?` text-${e.alert}`:"")})),u===!0&&x.push(_);const P=[w("div",{class:"q-focus-helper",tabindex:-1,ref:l}),w("div",{class:b.value},et(a.default,x))];return u===!1&&P.push(_),P}const V={name:m(()=>e.name),rootRef:p,tabIndicatorRef:I,routeData:v};Ae(()=>{r.unregisterTab(V)}),He(()=>{r.registerTab(V)});function j(u,x){const _=W({ref:p,class:D.value,tabindex:y.value,role:"tab","aria-selected":g.value===!0?"true":"false","aria-disabled":e.disable===!0?"true":void 0,onClick:q,onKeydown:L},x);return Ye(w(u,_,B()),[[Ge,M.value]])}return{renderTab:j,$tabs:r}}var Nt=te({name:"QRouteTab",props:W(W({},nt),Pt),emits:wt,setup(e,{slots:a,emit:i}){const v=at({useDisableForRouterLinkProps:!1}),{renderTab:r,$tabs:n}=qt(e,a,i,W({exact:m(()=>e.exact)},v));return U(()=>`${e.name} | ${e.exact} | ${(v.resolvedLink.value||{}).href}`,()=>{n.verifyRouteModel()}),()=>r(v.linkTag.value,v.linkAttrs.value)}});function xt(e,a,i){const v=i===!0?["left","right"]:["top","bottom"];return`absolute-${a===!0?v[0]:v[1]}${e?` text-${e}`:""}`}const kt=["left","center","right","justify"];var Kt=te({name:"QTabs",props:{modelValue:[Number,String],align:{type:String,default:"center",validator:e=>kt.includes(e)},breakpoint:{type:[String,Number],default:600},vertical:Boolean,shrink:Boolean,stretch:Boolean,activeClass:String,activeColor:String,activeBgColor:String,indicatorColor:String,leftIcon:String,rightIcon:String,outsideArrows:Boolean,mobileArrows:Boolean,switchIndicator:Boolean,narrowIndicator:Boolean,inlineLabel:Boolean,noCaps:Boolean,dense:Boolean,contentClass:String,"onUpdate:modelValue":[Function,Array]},setup(e,{slots:a,emit:i}){const{proxy:v}=ee(),{$q:r}=v,{registerTick:n}=fe(),{registerTick:l}=fe(),{registerTick:p}=fe(),{registerTimeout:I,removeTimeout:M}=_e(),{registerTimeout:g,removeTimeout:D}=_e(),b=A(null),y=A(null),q=A(e.modelValue),L=A(!1),B=A(!0),V=A(!1),j=A(!1),u=[],x=A(0),_=A(!1);let P=null,k=null,$;const E=m(()=>({activeClass:e.activeClass,activeColor:e.activeColor,activeBgColor:e.activeBgColor,indicatorClass:xt(e.indicatorColor,e.switchIndicator,e.vertical),narrowIndicator:e.narrowIndicator,inlineLabel:e.inlineLabel,noCaps:e.noCaps})),ae=m(()=>{const t=x.value,o=q.value;for(let s=0;s<t;s++)if(u[s].name.value===o)return!0;return!1}),oe=m(()=>`q-tabs__content--align-${L.value===!0?"left":j.value===!0?"justify":e.align}`),re=m(()=>`q-tabs row no-wrap items-center q-tabs--${L.value===!0?"":"not-"}scrollable q-tabs--${e.vertical===!0?"vertical":"horizontal"} q-tabs__arrows--${e.outsideArrows===!0?"outside":"inside"} q-tabs--mobile-with${e.mobileArrows===!0?"":"out"}-arrows`+(e.dense===!0?" q-tabs--dense":"")+(e.shrink===!0?" col-shrink":"")+(e.stretch===!0?" self-stretch":"")),c=m(()=>"q-tabs__content scroll--mobile row no-wrap items-center self-stretch hide-scrollbar relative-position "+oe.value+(e.contentClass!==void 0?` ${e.contentClass}`:"")),h=m(()=>e.vertical===!0?{container:"height",content:"offsetHeight",scroll:"scrollHeight"}:{container:"width",content:"offsetWidth",scroll:"scrollWidth"}),R=m(()=>e.vertical!==!0&&r.lang.rtl===!0),F=m(()=>gt===!1&&R.value===!0);U(R,z),U(()=>e.modelValue,t=>{ie({name:t,setCurrent:!0,skipEmit:!0})}),U(()=>e.outsideArrows,Y);function ie({name:t,setCurrent:o,skipEmit:s}){q.value!==t&&(s!==!0&&e["onUpdate:modelValue"]!==void 0&&i("update:modelValue",t),(o===!0||e["onUpdate:modelValue"]===void 0)&&($e(q.value,t),q.value=t))}function Y(){n(()=>{he({width:b.value.offsetWidth,height:b.value.offsetHeight})})}function he(t){if(h.value===void 0||y.value===null)return;const o=t[h.value.container],s=Math.min(y.value[h.value.scroll],Array.prototype.reduce.call(y.value.children,(C,f)=>C+(f[h.value.content]||0),0)),T=o>0&&s>o;L.value=T,T===!0&&l(z),j.value=o<parseInt(e.breakpoint,10)}function $e(t,o){const s=t!=null&&t!==""?u.find(C=>C.name.value===t):null,T=o!=null&&o!==""?u.find(C=>C.name.value===o):null;if(s&&T){const C=s.tabIndicatorRef.value,f=T.tabIndicatorRef.value;P!==null&&(clearTimeout(P),P=null),C.style.transition="none",C.style.transform="none",f.style.transition="none",f.style.transform="none";const d=C.getBoundingClientRect(),S=f.getBoundingClientRect();f.style.transform=e.vertical===!0?`translate3d(0,${d.top-S.top}px,0) scale3d(1,${S.height?d.height/S.height:1},1)`:`translate3d(${d.left-S.left}px,0,0) scale3d(${S.width?d.width/S.width:1},1,1)`,p(()=>{P=setTimeout(()=>{P=null,f.style.transition="transform .25s cubic-bezier(.4, 0, .2, 1)",f.style.transform="none"},70)})}T&&L.value===!0&&O(T.rootRef.value)}function O(t){const{left:o,width:s,top:T,height:C}=y.value.getBoundingClientRect(),f=t.getBoundingClientRect();let d=e.vertical===!0?f.top-T:f.left-o;if(d<0){y.value[e.vertical===!0?"scrollTop":"scrollLeft"]+=Math.floor(d),z();return}d+=e.vertical===!0?f.height-C:f.width-s,d>0&&(y.value[e.vertical===!0?"scrollTop":"scrollLeft"]+=Math.ceil(d),z())}function z(){const t=y.value;if(t===null)return;const o=t.getBoundingClientRect(),s=e.vertical===!0?t.scrollTop:Math.abs(t.scrollLeft);R.value===!0?(B.value=Math.ceil(s+o.width)<t.scrollWidth-1,V.value=s>0):(B.value=s>0,V.value=e.vertical===!0?Math.ceil(s+o.height)<t.scrollHeight:Math.ceil(s+o.width)<t.scrollWidth)}function be(t){k!==null&&clearInterval(k),k=setInterval(()=>{Me(t)===!0&&N()},5)}function me(){be(F.value===!0?Number.MAX_SAFE_INTEGER:0)}function ge(){be(F.value===!0?0:Number.MAX_SAFE_INTEGER)}function N(){k!==null&&(clearInterval(k),k=null)}function Ie(t,o){const s=Array.prototype.filter.call(y.value.children,S=>S===o||S.matches&&S.matches(".q-tab.q-focusable")===!0),T=s.length;if(T===0)return;if(t===36)return O(s[0]),s[0].focus(),!0;if(t===35)return O(s[T-1]),s[T-1].focus(),!0;const C=t===(e.vertical===!0?38:37),f=t===(e.vertical===!0?40:39),d=C===!0?-1:f===!0?1:void 0;if(d!==void 0){const S=R.value===!0?-1:1,Q=s.indexOf(o)+d*S;return Q>=0&&Q<T&&(O(s[Q]),s[Q].focus({preventScroll:!0})),!0}}const Be=m(()=>F.value===!0?{get:t=>Math.abs(t.scrollLeft),set:(t,o)=>{t.scrollLeft=-o}}:e.vertical===!0?{get:t=>t.scrollTop,set:(t,o)=>{t.scrollTop=o}}:{get:t=>t.scrollLeft,set:(t,o)=>{t.scrollLeft=o}});function Me(t){const o=y.value,{get:s,set:T}=Be.value;let C=!1,f=s(o);const d=t<f?-1:1;return f+=d*5,f<0?(C=!0,f=0):(d===-1&&f<=t||d===1&&f>=t)&&(C=!0,f=t),T(o,f),z(),C}function pe(t,o){for(const s in t)if(t[s]!==o[s])return!1;return!0}function De(){let t=null,o={matchedLen:0,queryDiff:9999,hrefLen:0};const s=u.filter(d=>d.routeData!==void 0&&d.routeData.hasRouterLink.value===!0),{hash:T,query:C}=v.$route,f=Object.keys(C).length;for(const d of s){const S=d.routeData.exact.value===!0;if(d.routeData[S===!0?"linkIsExactActive":"linkIsActive"].value!==!0)continue;const{hash:Q,query:le,matched:Fe,href:Ne}=d.routeData.resolvedLink.value,ue=Object.keys(le).length;if(S===!0){if(Q!==T||ue!==f||pe(C,le)===!1)continue;t=d.name.value;break}if(Q!==""&&Q!==T||ue!==0&&pe(le,C)===!1)continue;const K={matchedLen:Fe.length,queryDiff:f-ue,hrefLen:Ne.length-Q.length};if(K.matchedLen>o.matchedLen){t=d.name.value,o=K;continue}else if(K.matchedLen!==o.matchedLen)continue;if(K.queryDiff<o.queryDiff)t=d.name.value,o=K;else if(K.queryDiff!==o.queryDiff)continue;K.hrefLen>o.hrefLen&&(t=d.name.value,o=K)}t===null&&u.some(d=>d.routeData===void 0&&d.name.value===q.value)===!0||ie({name:t,setCurrent:!0})}function Ee(t){if(M(),_.value!==!0&&b.value!==null&&t.target&&typeof t.target.closest=="function"){const o=t.target.closest(".q-tab");o&&b.value.contains(o)===!0&&(_.value=!0,L.value===!0&&O(o))}}function Ve(){I(()=>{_.value=!1},30)}function G(){Te.avoidRouteWatcher===!1?g(De):D()}function ye(){if($===void 0){const t=U(()=>v.$route.fullPath,G);$=()=>{t(),$=void 0}}}function Qe(t){u.push(t),x.value++,Y(),t.routeData===void 0||v.$route===void 0?g(()=>{if(L.value===!0){const o=q.value,s=o!=null&&o!==""?u.find(T=>T.name.value===o):null;s&&O(s.rootRef.value)}}):(ye(),t.routeData.hasRouterLink.value===!0&&G())}function We(t){u.splice(u.indexOf(t),1),x.value--,Y(),$!==void 0&&t.routeData!==void 0&&(u.every(o=>o.routeData===void 0)===!0&&$(),G())}const Te={currentModel:q,tabProps:E,hasFocus:_,hasActiveTab:ae,registerTab:Qe,unregisterTab:We,verifyRouteModel:G,updateModel:ie,onKbdNavigate:Ie,avoidRouteWatcher:!1};ot(Le,Te);function Ce(){P!==null&&clearTimeout(P),N(),$!==void 0&&$()}let we;return Ae(Ce),rt(()=>{we=$!==void 0,Ce()}),it(()=>{we===!0&&ye(),Y()}),()=>w("div",{ref:b,class:re.value,role:"tablist",onFocusin:Ee,onFocusout:Ve},[w(mt,{onResize:he}),w("div",{ref:y,class:c.value,onScroll:z},ne(a.default)),w(J,{class:"q-tabs__arrow q-tabs__arrow--left absolute q-tab__icon"+(B.value===!0?"":" q-tabs__arrow--faded"),name:e.leftIcon||r.iconSet.tabs[e.vertical===!0?"up":"left"],onMousedownPassive:me,onTouchstartPassive:me,onMouseupPassive:N,onMouseleavePassive:N,onTouchendPassive:N}),w(J,{class:"q-tabs__arrow q-tabs__arrow--right absolute q-tab__icon"+(V.value===!0?"":" q-tabs__arrow--faded"),name:e.rightIcon||r.iconSet.tabs[e.vertical===!0?"down":"right"],onMousedownPassive:ge,onTouchstartPassive:ge,onMouseupPassive:N,onMouseleavePassive:N,onTouchendPassive:N})])}});function St(e){const a=[.06,6,50];return typeof e=="string"&&e.length&&e.split(":").forEach((i,v)=>{const r=parseFloat(i);r&&(a[v]=r)}),a}var _t=lt({name:"touch-swipe",beforeMount(e,{value:a,arg:i,modifiers:v}){if(v.mouse!==!0&&H.has.touch!==!0)return;const r=v.mouseCapture===!0?"Capture":"",n={handler:a,sensitivity:St(i),direction:ke(v),noop:ut,mouseStart(l){Se(l,n)&&st(l)&&(Z(n,"temp",[[document,"mousemove","move",`notPassive${r}`],[document,"mouseup","end","notPassiveCapture"]]),n.start(l,!0))},touchStart(l){if(Se(l,n)){const p=l.target;Z(n,"temp",[[p,"touchmove","move","notPassiveCapture"],[p,"touchcancel","end","notPassiveCapture"],[p,"touchend","end","notPassiveCapture"]]),n.start(l)}},start(l,p){H.is.firefox===!0&&ve(e,!0);const I=xe(l);n.event={x:I.left,y:I.top,time:Date.now(),mouse:p===!0,dir:!1}},move(l){if(n.event===void 0)return;if(n.event.dir!==!1){X(l);return}const p=Date.now()-n.event.time;if(p===0)return;const I=xe(l),M=I.left-n.event.x,g=Math.abs(M),D=I.top-n.event.y,b=Math.abs(D);if(n.event.mouse!==!0){if(g<n.sensitivity[1]&&b<n.sensitivity[1]){n.end(l);return}}else if(window.getSelection().toString()!==""){n.end(l);return}else if(g<n.sensitivity[2]&&b<n.sensitivity[2])return;const y=g/p,q=b/p;n.direction.vertical===!0&&g<b&&g<100&&q>n.sensitivity[0]&&(n.event.dir=D<0?"up":"down"),n.direction.horizontal===!0&&g>b&&b<100&&y>n.sensitivity[0]&&(n.event.dir=M<0?"left":"right"),n.direction.up===!0&&g<b&&D<0&&g<100&&q>n.sensitivity[0]&&(n.event.dir="up"),n.direction.down===!0&&g<b&&D>0&&g<100&&q>n.sensitivity[0]&&(n.event.dir="down"),n.direction.left===!0&&g>b&&M<0&&b<100&&y>n.sensitivity[0]&&(n.event.dir="left"),n.direction.right===!0&&g>b&&M>0&&b<100&&y>n.sensitivity[0]&&(n.event.dir="right"),n.event.dir!==!1?(X(l),n.event.mouse===!0&&(document.body.classList.add("no-pointer-events--children"),document.body.classList.add("non-selectable"),Tt(),n.styleCleanup=L=>{n.styleCleanup=void 0,document.body.classList.remove("non-selectable");const B=()=>{document.body.classList.remove("no-pointer-events--children")};L===!0?setTimeout(B,50):B()}),n.handler({evt:l,touch:n.event.mouse!==!0,mouse:n.event.mouse,direction:n.event.dir,duration:p,distance:{x:g,y:b}})):n.end(l)},end(l){n.event!==void 0&&(de(n,"temp"),H.is.firefox===!0&&ve(e,!1),n.styleCleanup!==void 0&&n.styleCleanup(!0),l!==void 0&&n.event.dir!==!1&&X(l),n.event=void 0)}};if(e.__qtouchswipe=n,v.mouse===!0){const l=v.mouseCapture===!0||v.mousecapture===!0?"Capture":"";Z(n,"main",[[e,"mousedown","mouseStart",`passive${l}`]])}H.has.touch===!0&&Z(n,"main",[[e,"touchstart","touchStart",`passive${v.capture===!0?"Capture":""}`],[e,"touchmove","noop","notPassiveCapture"]])},updated(e,a){const i=e.__qtouchswipe;i!==void 0&&(a.oldValue!==a.value&&(typeof a.value!="function"&&i.end(),i.handler=a.value),i.direction=ke(a.modifiers))},beforeUnmount(e){const a=e.__qtouchswipe;a!==void 0&&(de(a,"main"),de(a,"temp"),H.is.firefox===!0&&ve(e,!1),a.styleCleanup!==void 0&&a.styleCleanup(),delete e.__qtouchswipe)}});function Rt(){const e=new Map;return{getCache:function(a,i){return e[a]===void 0?e[a]=i:e[a]},getCacheWithFn:function(a,i){return e[a]===void 0?e[a]=i():e[a]}}}const At={name:{required:!0},disable:Boolean},Re={setup(e,{slots:a}){return()=>w("div",{class:"q-panel scroll",role:"tabpanel"},ne(a.default))}},Lt={modelValue:{required:!0},animated:Boolean,infinite:Boolean,swipeable:Boolean,vertical:Boolean,transitionPrev:String,transitionNext:String,transitionDuration:{type:[String,Number],default:300},keepAlive:Boolean,keepAliveInclude:[String,Array,RegExp],keepAliveExclude:[String,Array,RegExp],keepAliveMax:Number},$t=["update:modelValue","beforeTransition","transition"];function It(){const{props:e,emit:a,proxy:i}=ee(),{getCacheWithFn:v}=Rt();let r,n;const l=A(null),p=A(null);function I(c){const h=e.vertical===!0?"up":"left";k((i.$q.lang.rtl===!0?-1:1)*(c.direction===h?1:-1))}const M=m(()=>[[_t,I,void 0,{horizontal:e.vertical!==!0,vertical:e.vertical,mouse:!0}]]),g=m(()=>e.transitionPrev||`slide-${e.vertical===!0?"down":"right"}`),D=m(()=>e.transitionNext||`slide-${e.vertical===!0?"up":"left"}`),b=m(()=>`--q-transition-duration: ${e.transitionDuration}ms`),y=m(()=>typeof e.modelValue=="string"||typeof e.modelValue=="number"?e.modelValue:String(e.modelValue)),q=m(()=>({include:e.keepAliveInclude,exclude:e.keepAliveExclude,max:e.keepAliveMax})),L=m(()=>e.keepAliveInclude!==void 0||e.keepAliveExclude!==void 0);U(()=>e.modelValue,(c,h)=>{const R=u(c)===!0?x(c):-1;n!==!0&&P(R===-1?0:R<x(h)?-1:1),l.value!==R&&(l.value=R,a("beforeTransition",c,h),ct(()=>{a("transition",c,h)}))});function B(){k(1)}function V(){k(-1)}function j(c){a("update:modelValue",c)}function u(c){return c!=null&&c!==""}function x(c){return r.findIndex(h=>h.props.name===c&&h.props.disable!==""&&h.props.disable!==!0)}function _(){return r.filter(c=>c.props.disable!==""&&c.props.disable!==!0)}function P(c){const h=c!==0&&e.animated===!0&&l.value!==-1?"q-transition--"+(c===-1?g.value:D.value):null;p.value!==h&&(p.value=h)}function k(c,h=l.value){let R=h+c;for(;R>-1&&R<r.length;){const F=r[R];if(F!==void 0&&F.props.disable!==""&&F.props.disable!==!0){P(c),n=!0,a("update:modelValue",F.props.name),setTimeout(()=>{n=!1});return}R+=c}e.infinite===!0&&r.length!==0&&h!==-1&&h!==r.length&&k(c,c===-1?r.length:-1)}function $(){const c=x(e.modelValue);return l.value!==c&&(l.value=c),!0}function E(){const c=u(e.modelValue)===!0&&$()&&r[l.value];return e.keepAlive===!0?[w(dt,q.value,[w(L.value===!0?v(y.value,()=>se(W({},Re),{name:y.value})):Re,{key:y.value,style:b.value},()=>c)])]:[w("div",{class:"q-panel scroll",style:b.value,key:y.value,role:"tabpanel"},[c])]}function ae(){if(r.length!==0)return e.animated===!0?[w(vt,{name:p.value},E)]:E()}function oe(c){return r=ft(ne(c.default,[])).filter(h=>h.props!==null&&h.props.slot===void 0&&u(h.props.name)===!0),r.length}function re(){return r}return Object.assign(i,{next:B,previous:V,goTo:j}),{panelIndex:l,panelDirectives:M,updatePanelsList:oe,updatePanelIndex:$,getPanelContent:ae,getEnabledPanels:_,getPanels:re,isValidPanelName:u,keepAliveProps:q,needsUniqueKeepAliveWrapper:L,goToPanelByOffset:k,goToPanel:j,nextPanel:B,previousPanel:V}}var jt=te({name:"QTabPanel",props:At,setup(e,{slots:a}){return()=>w("div",{class:"q-tab-panel",role:"tabpanel"},ne(a.default))}}),Ot=te({name:"QTabPanels",props:W(W({},Lt),pt),emits:$t,setup(e,{slots:a}){const i=ee(),v=yt(e,i.proxy.$q),{updatePanelsList:r,getPanelContent:n,panelDirectives:l}=It(),p=m(()=>"q-tab-panels q-panel-parent"+(v.value===!0?" q-tab-panels--dark q-dark":""));return()=>(r(a),ht("div",{class:p.value},n(),"pan",e.swipeable,()=>l.value))}});export{Kt as Q,Nt as a,Ot as b,jt as c,Rt as u};