var Fe=Object.defineProperty,Ne=Object.defineProperties;var Qe=Object.getOwnPropertyDescriptors;var ye=Object.getOwnPropertySymbols;var We=Object.prototype.hasOwnProperty,ze=Object.prototype.propertyIsEnumerable;var Te=(e,a,r)=>a in e?Fe(e,a,{enumerable:!0,configurable:!0,writable:!0,value:r}):e[a]=r,Q=(e,a)=>{for(var r in a||(a={}))We.call(a,r)&&Te(e,r,a[r]);if(ye)for(var r of ye(a))ze.call(a,r)&&Te(e,r,a[r]);return e},we=(e,a)=>Ne(e,Qe(a));import{r as I,c as m,L as Re,o as Ke,M as Oe,e as je,N as Ue,h as w,O as Ae,S as He,U as Xe,V as G,v as J,W as Ye,g as Z,a as ee,X as Ge,Y as Je,w as W,Z as ce,_ as Ze,$ as et,a0 as tt,b as te,n as O,a1 as nt,a2 as K,a3 as at,a4 as Y,a5 as se,p as xe,a6 as ue,T as ot,a7 as lt,a8 as rt,a9 as it}from"./index.e18600f8.js";import{Q as st,g as Pe,s as qe}from"./touch.1d111f56.js";import{u as ut}from"./QDialog.07d9a48e.js";import{u as _e}from"./use-prevent-scroll.d4f2a0d7.js";import{r as ct}from"./rtl.4b414a6d.js";import{u as dt,a as vt}from"./use-dark.f89e8300.js";import{c as ft}from"./QList.68ba6ad8.js";let bt=0;const ht=["click","keydown"],mt={icon:String,label:[Number,String],alert:[Boolean,String],alertIcon:String,name:{type:[Number,String],default:()=>`t_${bt++}`},noCaps:Boolean,tabindex:[String,Number],disable:Boolean,contentClass:String,ripple:{type:[Boolean,Object],default:!0}};function gt(e,a,r,u){const o=Oe(Ae,()=>{console.error("QTab/QRouteTab component needs to be child of QTabs")}),{proxy:n}=Z(),s=I(null),C=I(null),$=I(null),_=m(()=>e.disable===!0||e.ripple===!1?!1:Object.assign({keyCodes:[13,32],early:!0},e.ripple===!0?{}:e.ripple)),c=m(()=>o.currentModel.value===e.name),S=m(()=>"q-tab relative-position self-stretch flex flex-center text-center"+(c.value===!0?" q-tab--active"+(o.tabProps.value.activeClass?" "+o.tabProps.value.activeClass:"")+(o.tabProps.value.activeColor?` text-${o.tabProps.value.activeColor}`:"")+(o.tabProps.value.activeBgColor?` bg-${o.tabProps.value.activeBgColor}`:""):" q-tab--inactive")+(e.icon&&e.label&&o.tabProps.value.inlineLabel===!1?" q-tab--full":"")+(e.noCaps===!0||o.tabProps.value.noCaps===!0?" q-tab--no-caps":"")+(e.disable===!0?" disabled":" q-focusable q-hoverable cursor-pointer")+(u!==void 0&&u.linkClass.value!==""?` ${u.linkClass.value}`:"")),b=m(()=>"q-tab__content self-stretch flex-center relative-position q-anchor--skip non-selectable "+(o.tabProps.value.inlineLabel===!0?"row no-wrap q-tab__content--inline":"column")+(e.contentClass!==void 0?` ${e.contentClass}`:"")),k=m(()=>e.disable===!0||o.hasFocus.value===!0?-1:e.tabindex||0);function A(f,x){if(x!==!0&&s.value!==null&&s.value.focus(),e.disable!==!0){let R;if(u!==void 0)if(u.hasRouterLink.value===!0)R=()=>{f.__qNavigate=!0,o.avoidRouteWatcher=!0;const P=u.navigateToRouterLink(f);P===!1?o.avoidRouteWatcher=!1:P.then(q=>{o.avoidRouteWatcher=!1,q===void 0&&o.updateModel({name:e.name,fromRoute:!0})})};else{r("click",f);return}else R=()=>{o.updateModel({name:e.name,fromRoute:!1})};r("click",f,R),f.defaultPrevented!==!0&&R()}}function E(f){He(f,[13,32])?A(f,!0):Xe(f)!==!0&&f.keyCode>=35&&f.keyCode<=40&&o.onKbdNavigate(f.keyCode,n.$el)===!0&&G(f),r("keydown",f)}function B(){const f=o.tabProps.value.narrowIndicator,x=[],R=w("div",{ref:$,class:["q-tab__indicator",o.tabProps.value.indicatorClass]});e.icon!==void 0&&x.push(w(J,{class:"q-tab__icon",name:e.icon})),e.label!==void 0&&x.push(w("div",{class:"q-tab__label"},e.label)),e.alert!==!1&&x.push(e.alertIcon!==void 0?w(J,{class:"q-tab__alert-icon",color:e.alert!==!0?e.alert:void 0,name:e.alertIcon}):w("div",{class:"q-tab__alert"+(e.alert!==!0?` text-${e.alert}`:"")})),f===!0&&x.push(R);const P=[w("div",{class:"q-focus-helper",tabindex:-1,ref:s}),w("div",{class:b.value},Ye(a.default,x))];return f===!1&&P.push(R),P}const M={name:m(()=>e.name),rootRef:C,tabIndicatorRef:$,routerProps:u};Re(()=>{o.unregisterTab(M),o.recalculateScroll()}),Ke(()=>{o.registerTab(M),o.recalculateScroll()});function D(f,x){const R=Q({ref:C,class:S.value,tabindex:k.value,role:"tab","aria-selected":c.value===!0?"true":"false","aria-disabled":e.disable===!0?"true":void 0,onClick:A,onKeydown:E},x);return je(w(f,R,B()),[[Ue,_.value]])}return{renderTab:D,$tabs:o}}var Mt=ee({name:"QRouteTab",props:Q(Q({},Ge),mt),emits:ht,setup(e,{slots:a,emit:r}){const u=Je(),{renderTab:o,$tabs:n}=gt(e,a,r,Q({exact:m(()=>e.exact)},u));return W(()=>e.name+e.exact+(u.linkRoute.value||{}).href,()=>{n.verifyRouteModel()}),()=>o(u.linkTag.value,u.linkProps.value)}});function pt(e,a,r){const u=r===!0?["left","right"]:["top","bottom"];return`absolute-${a===!0?u[0]:u[1]}${e?` text-${e}`:""}`}const Ct=["left","center","right","justify"],Se=()=>{};var Et=ee({name:"QTabs",props:{modelValue:[Number,String],align:{type:String,default:"center",validator:e=>Ct.includes(e)},breakpoint:{type:[String,Number],default:600},vertical:Boolean,shrink:Boolean,stretch:Boolean,activeClass:String,activeColor:String,activeBgColor:String,indicatorColor:String,leftIcon:String,rightIcon:String,outsideArrows:Boolean,mobileArrows:Boolean,switchIndicator:Boolean,narrowIndicator:Boolean,inlineLabel:Boolean,noCaps:Boolean,dense:Boolean,contentClass:String,"onUpdate:modelValue":[Function,Array]},setup(e,{slots:a,emit:r}){const u=Z(),{proxy:{$q:o}}=u,{registerTick:n}=ut(),{registerTimeout:s,removeTimeout:C}=_e(),{registerTimeout:$}=_e(),_=I(null),c=I(null),S=I(e.modelValue),b=I(!1),k=I(!0),A=I(!1),E=I(!1),B=m(()=>o.platform.is.desktop===!0||e.mobileArrows===!0),M=[],D=I(!1);let f=!1,x,R,P,q=B.value===!0?ve:ce;const j=m(()=>({activeClass:e.activeClass,activeColor:e.activeColor,activeBgColor:e.activeBgColor,indicatorClass:pt(e.indicatorColor,e.switchIndicator,e.vertical),narrowIndicator:e.narrowIndicator,inlineLabel:e.inlineLabel,noCaps:e.noCaps})),U=m(()=>`q-tabs__content--align-${b.value===!0?"left":E.value===!0?"justify":e.align}`),ne=m(()=>`q-tabs row no-wrap items-center q-tabs--${b.value===!0?"":"not-"}scrollable q-tabs--${e.vertical===!0?"vertical":"horizontal"} q-tabs__arrows--${B.value===!0&&e.outsideArrows===!0?"outside":"inside"}`+(e.dense===!0?" q-tabs--dense":"")+(e.shrink===!0?" col-shrink":"")+(e.stretch===!0?" self-stretch":"")),ae=m(()=>"q-tabs__content row no-wrap items-center self-stretch hide-scrollbar relative-position "+U.value+(e.contentClass!==void 0?` ${e.contentClass}`:"")+(o.platform.is.mobile===!0?" scroll":"")),z=m(()=>e.vertical===!0?{container:"height",content:"offsetHeight",scroll:"scrollHeight"}:{container:"width",content:"offsetWidth",scroll:"scrollWidth"}),l=m(()=>e.vertical!==!0&&o.lang.rtl===!0),h=m(()=>ct===!1&&l.value===!0);W(l,q),W(()=>e.modelValue,t=>{L({name:t,setCurrent:!0,skipEmit:!0})}),W(()=>e.outsideArrows,()=>{O(V())}),W(B,t=>{q=t===!0?ve:ce,O(V())});function L({name:t,setCurrent:i,skipEmit:d,fromRoute:p}){S.value!==t&&(d!==!0&&r("update:modelValue",t),(i===!0||e["onUpdate:modelValue"]===void 0)&&(Le(S.value,t),S.value=t)),p!==void 0&&(f=p)}function V(){n(()=>{u.isDeactivated!==!0&&u.isUnmounted!==!0&&de({width:_.value.offsetWidth,height:_.value.offsetHeight})})}function de(t){if(z.value===void 0||c.value===null)return;const i=t[z.value.container],d=Math.min(c.value[z.value.scroll],Array.prototype.reduce.call(c.value.children,(v,y)=>v+(y[z.value.content]||0),0)),p=i>0&&d>i;b.value!==p&&(b.value=p),p===!0&&O(q);const T=i<parseInt(e.breakpoint,10);E.value!==T&&(E.value=T)}function Le(t,i){const d=t!=null&&t!==""?M.find(T=>T.name.value===t):null,p=i!=null&&i!==""?M.find(T=>T.name.value===i):null;if(d&&p){const T=d.tabIndicatorRef.value,v=p.tabIndicatorRef.value;clearTimeout(x),T.style.transition="none",T.style.transform="none",v.style.transition="none",v.style.transform="none";const y=T.getBoundingClientRect(),g=v.getBoundingClientRect();v.style.transform=e.vertical===!0?`translate3d(0,${y.top-g.top}px,0) scale3d(1,${g.height?y.height/g.height:1},1)`:`translate3d(${y.left-g.left}px,0,0) scale3d(${g.width?y.width/g.width:1},1,1)`,O(()=>{x=setTimeout(()=>{v.style.transition="transform .25s cubic-bezier(.4, 0, .2, 1)",v.style.transform="none"},70)})}p&&b.value===!0&&H(p.rootRef.value)}function H(t){const{left:i,width:d,top:p,height:T}=c.value.getBoundingClientRect(),v=t.getBoundingClientRect();let y=e.vertical===!0?v.top-p:v.left-i;if(y<0){c.value[e.vertical===!0?"scrollTop":"scrollLeft"]+=Math.floor(y),q();return}y+=e.vertical===!0?v.height-T:v.width-d,y>0&&(c.value[e.vertical===!0?"scrollTop":"scrollLeft"]+=Math.ceil(y),q())}function ve(){const t=c.value;if(t!==null){const i=t.getBoundingClientRect(),d=e.vertical===!0?t.scrollTop:Math.abs(t.scrollLeft);l.value===!0?(k.value=Math.ceil(d+i.width)<t.scrollWidth-1,A.value=d>0):(k.value=d>0,A.value=e.vertical===!0?Math.ceil(d+i.height)<t.scrollHeight:Math.ceil(d+i.width)<t.scrollWidth)}}function fe(t){N(),me(t),R=setInterval(()=>{me(t)===!0&&N()},5)}function be(){fe(h.value===!0?Number.MAX_SAFE_INTEGER:0)}function he(){fe(h.value===!0?0:Number.MAX_SAFE_INTEGER)}function N(){clearInterval(R)}function Ie(t,i){const d=Array.prototype.filter.call(c.value.children,g=>g===i||g.matches&&g.matches(".q-tab.q-focusable")===!0),p=d.length;if(p===0)return;if(t===36)return H(d[0]),!0;if(t===35)return H(d[p-1]),!0;const T=t===(e.vertical===!0?38:37),v=t===(e.vertical===!0?40:39),y=T===!0?-1:v===!0?1:void 0;if(y!==void 0){const g=l.value===!0?-1:1,F=d.indexOf(i)+y*g;return F>=0&&F<p&&(H(d[F]),d[F].focus({preventScroll:!0})),!0}}const $e=m(()=>h.value===!0?{get:t=>Math.abs(t.scrollLeft),set:(t,i)=>{t.scrollLeft=-i}}:e.vertical===!0?{get:t=>t.scrollTop,set:(t,i)=>{t.scrollTop=i}}:{get:t=>t.scrollLeft,set:(t,i)=>{t.scrollLeft=i}});function me(t){const i=c.value,{get:d,set:p}=$e.value;let T=!1,v=d(i);const y=t<v?-1:1;return v+=y*5,v<0?(T=!0,v=0):(y===-1&&v<=t||y===1&&v>=t)&&(T=!0,v=t),p(i,v),q(),T}function oe(){return M.filter(t=>t.routerProps!==void 0&&t.routerProps.hasRouterLink.value===!0)}function Be(){let t=null,i=f;const d={matchedLen:0,hrefLen:0,exact:!1,found:!1},{hash:p}=u.proxy.$route,T=S.value;let v=i===!0?Se:g=>{T===g.name.value&&(i=!0,v=Se)};const y=oe();for(const g of y){const F=g.routerProps.exact.value===!0;if(g.routerProps[F===!0?"linkIsExactActive":"linkIsActive"].value!==!0||d.exact===!0&&F!==!0){v(g);continue}const le=g.routerProps.linkRoute.value,re=le.hash;if(F===!0){if(p===re){t=g.name.value;break}else if(p!==""&&re!==""){v(g);continue}}const ie=le.matched.length,Ce=le.href.length-re.length;if(ie===d.matchedLen?Ce>d.hrefLen:ie>d.matchedLen){t=g.name.value,Object.assign(d,{matchedLen:ie,hrefLen:Ce,exact:F});continue}v(g)}(i===!0||t!==null)&&L({name:t,setCurrent:!0,fromRoute:!0})}function Me(t){if(C(),D.value!==!0&&_.value!==null&&t.target&&typeof t.target.closest=="function"){const i=t.target.closest(".q-tab");i&&_.value.contains(i)===!0&&(D.value=!0)}}function Ee(){s(()=>{D.value=!1},30)}function X(){ge.avoidRouteWatcher!==!0&&$(Be)}function De(t){M.push(t),oe().length>0&&(P===void 0&&(P=W(()=>u.proxy.$route,X)),X())}function Ve(t){M.splice(M.indexOf(t),1),P!==void 0&&(oe().length===0&&(P(),P=void 0),X())}const ge={currentModel:S,tabProps:j,hasFocus:D,registerTab:De,unregisterTab:Ve,verifyRouteModel:X,updateModel:L,recalculateScroll:V,onKbdNavigate:Ie,avoidRouteWatcher:!1};Ze(Ae,ge),Re(()=>{clearTimeout(x),P!==void 0&&P()});let pe=!1;return et(()=>{pe=!0}),tt(()=>{pe===!0&&V()}),()=>{const t=[w(st,{onResize:de}),w("div",{ref:c,class:ae.value,onScroll:q},te(a.default))];return B.value===!0&&t.push(w(J,{class:"q-tabs__arrow q-tabs__arrow--left absolute q-tab__icon"+(k.value===!0?"":" q-tabs__arrow--faded"),name:e.leftIcon||o.iconSet.tabs[e.vertical===!0?"up":"left"],onMousedown:be,onTouchstartPassive:be,onMouseup:N,onMouseleave:N,onTouchend:N}),w(J,{class:"q-tabs__arrow q-tabs__arrow--right absolute q-tab__icon"+(A.value===!0?"":" q-tabs__arrow--faded"),name:e.rightIcon||o.iconSet.tabs[e.vertical===!0?"down":"right"],onMousedown:he,onTouchstartPassive:he,onMouseup:N,onMouseleave:N,onTouchend:N})),w("div",{ref:_,class:ne.value,role:"tablist",onFocusin:Me,onFocusout:Ee},t)}}});function yt(e){const a=[.06,6,50];return typeof e=="string"&&e.length&&e.split(":").forEach((r,u)=>{const o=parseFloat(r);o&&(a[u]=o)}),a}var Tt=nt({name:"touch-swipe",beforeMount(e,{value:a,arg:r,modifiers:u}){if(u.mouse!==!0&&K.has.touch!==!0)return;const o=u.mouseCapture===!0?"Capture":"",n={handler:a,sensitivity:yt(r),direction:Pe(u),noop:ce,mouseStart(s){qe(s,n)&&at(s)&&(Y(n,"temp",[[document,"mousemove","move",`notPassive${o}`],[document,"mouseup","end","notPassiveCapture"]]),n.start(s,!0))},touchStart(s){if(qe(s,n)){const C=s.target;Y(n,"temp",[[C,"touchmove","move","notPassiveCapture"],[C,"touchcancel","end","notPassiveCapture"],[C,"touchend","end","notPassiveCapture"]]),n.start(s)}},start(s,C){K.is.firefox===!0&&se(e,!0);const $=xe(s);n.event={x:$.left,y:$.top,time:Date.now(),mouse:C===!0,dir:!1}},move(s){if(n.event===void 0)return;if(n.event.dir!==!1){G(s);return}const C=Date.now()-n.event.time;if(C===0)return;const $=xe(s),_=$.left-n.event.x,c=Math.abs(_),S=$.top-n.event.y,b=Math.abs(S);if(n.event.mouse!==!0){if(c<n.sensitivity[1]&&b<n.sensitivity[1]){n.end(s);return}}else if(c<n.sensitivity[2]&&b<n.sensitivity[2])return;const k=c/C,A=b/C;n.direction.vertical===!0&&c<b&&c<100&&A>n.sensitivity[0]&&(n.event.dir=S<0?"up":"down"),n.direction.horizontal===!0&&c>b&&b<100&&k>n.sensitivity[0]&&(n.event.dir=_<0?"left":"right"),n.direction.up===!0&&c<b&&S<0&&c<100&&A>n.sensitivity[0]&&(n.event.dir="up"),n.direction.down===!0&&c<b&&S>0&&c<100&&A>n.sensitivity[0]&&(n.event.dir="down"),n.direction.left===!0&&c>b&&_<0&&b<100&&k>n.sensitivity[0]&&(n.event.dir="left"),n.direction.right===!0&&c>b&&_>0&&b<100&&k>n.sensitivity[0]&&(n.event.dir="right"),n.event.dir!==!1?(G(s),n.event.mouse===!0&&(document.body.classList.add("no-pointer-events--children"),document.body.classList.add("non-selectable"),ft(),n.styleCleanup=E=>{n.styleCleanup=void 0,document.body.classList.remove("non-selectable");const B=()=>{document.body.classList.remove("no-pointer-events--children")};E===!0?setTimeout(B,50):B()}),n.handler({evt:s,touch:n.event.mouse!==!0,mouse:n.event.mouse,direction:n.event.dir,duration:C,distance:{x:c,y:b}})):n.end(s)},end(s){n.event!==void 0&&(ue(n,"temp"),K.is.firefox===!0&&se(e,!1),n.styleCleanup!==void 0&&n.styleCleanup(!0),s!==void 0&&n.event.dir!==!1&&G(s),n.event=void 0)}};e.__qtouchswipe=n,u.mouse===!0&&Y(n,"main",[[e,"mousedown","mouseStart",`passive${o}`]]),K.has.touch===!0&&Y(n,"main",[[e,"touchstart","touchStart",`passive${u.capture===!0?"Capture":""}`],[e,"touchmove","noop","notPassiveCapture"]])},updated(e,a){const r=e.__qtouchswipe;r!==void 0&&(a.oldValue!==a.value&&(typeof a.value!="function"&&r.end(),r.handler=a.value),r.direction=Pe(a.modifiers))},beforeUnmount(e){const a=e.__qtouchswipe;a!==void 0&&(ue(a,"main"),ue(a,"temp"),K.is.firefox===!0&&se(e,!1),a.styleCleanup!==void 0&&a.styleCleanup(),delete e.__qtouchswipe)}});function wt(){const e=new Map;return{getCache:function(a,r){return e[a]===void 0?e[a]=r:e[a]},getCacheWithFn:function(a,r){return e[a]===void 0?e[a]=r():e[a]}}}const xt={name:{required:!0},disable:Boolean},ke={setup(e,{slots:a}){return()=>w("div",{class:"q-panel scroll",role:"tabpanel"},te(a.default))}},Pt={modelValue:{required:!0},animated:Boolean,infinite:Boolean,swipeable:Boolean,vertical:Boolean,transitionPrev:String,transitionNext:String,transitionDuration:{type:[String,Number],default:300},keepAlive:Boolean,keepAliveInclude:[String,Array,RegExp],keepAliveExclude:[String,Array,RegExp],keepAliveMax:Number},qt=["update:modelValue","before-transition","transition"];function _t(){const{props:e,emit:a,proxy:r}=Z(),{getCacheWithFn:u}=wt();let o,n;const s=I(null),C=I(null);function $(l){const h=e.vertical===!0?"up":"left";q((r.$q.lang.rtl===!0?-1:1)*(l.direction===h?1:-1))}const _=m(()=>[[Tt,$,void 0,{horizontal:e.vertical!==!0,vertical:e.vertical,mouse:!0}]]),c=m(()=>e.transitionPrev||`slide-${e.vertical===!0?"down":"right"}`),S=m(()=>e.transitionNext||`slide-${e.vertical===!0?"up":"left"}`),b=m(()=>`--q-transition-duration: ${e.transitionDuration}ms`),k=m(()=>typeof e.modelValue=="string"||typeof e.modelValue=="number"?e.modelValue:String(e.modelValue)),A=m(()=>({include:e.keepAliveInclude,exclude:e.keepAliveExclude,max:e.keepAliveMax})),E=m(()=>e.keepAliveInclude!==void 0||e.keepAliveExclude!==void 0);W(()=>e.modelValue,(l,h)=>{const L=f(l)===!0?x(l):-1;n!==!0&&P(L===-1?0:L<x(h)?-1:1),s.value!==L&&(s.value=L,a("before-transition",l,h),O(()=>{a("transition",l,h)}))});function B(){q(1)}function M(){q(-1)}Object.assign(r,{next:B,previous:M,goTo:D});function D(l){a("update:modelValue",l)}function f(l){return l!=null&&l!==""}function x(l){return o.findIndex(h=>h.props.name===l&&h.props.disable!==""&&h.props.disable!==!0)}function R(){return o.filter(l=>l.props.disable!==""&&l.props.disable!==!0)}function P(l){const h=l!==0&&e.animated===!0&&s.value!==-1?"q-transition--"+(l===-1?c.value:S.value):null;C.value!==h&&(C.value=h)}function q(l,h=s.value){let L=h+l;for(;L>-1&&L<o.length;){const V=o[L];if(V!==void 0&&V.props.disable!==""&&V.props.disable!==!0){P(l),n=!0,a("update:modelValue",V.props.name),setTimeout(()=>{n=!1});return}L+=l}e.infinite===!0&&o.length>0&&h!==-1&&h!==o.length&&q(l,l===-1?o.length:-1)}function j(){const l=x(e.modelValue);return s.value!==l&&(s.value=l),!0}function U(){const l=f(e.modelValue)===!0&&j()&&o[s.value];return e.keepAlive===!0?[w(lt,A.value,[w(E.value===!0?u(k.value,()=>we(Q({},ke),{name:k.value})):ke,{key:k.value,style:b.value},()=>l)])]:[w("div",{class:"q-panel scroll",style:b.value,key:k.value,role:"tabpanel"},[l])]}function ne(){if(o.length!==0)return e.animated===!0?[w(ot,{name:C.value},U)]:U()}function ae(l){return o=rt(te(l.default,[])).filter(h=>h.props!==null&&h.props.slot===void 0&&f(h.props.name)===!0),o.length}function z(){return o}return{panelIndex:s,panelDirectives:_,updatePanelsList:ae,updatePanelIndex:j,getPanelContent:ne,getEnabledPanels:R,getPanels:z,isValidPanelName:f,keepAliveProps:A,needsUniqueKeepAliveWrapper:E,goToPanelByOffset:q,goToPanel:D,nextPanel:B,previousPanel:M}}var Dt=ee({name:"QTabPanel",props:xt,setup(e,{slots:a}){return()=>w("div",{class:"q-tab-panel"},te(a.default))}}),Vt=ee({name:"QTabPanels",props:Q(Q({},Pt),dt),emits:qt,setup(e,{slots:a}){const r=Z(),u=vt(e,r.proxy.$q),{updatePanelsList:o,getPanelContent:n,panelDirectives:s}=_t(),C=m(()=>"q-tab-panels q-panel-parent"+(u.value===!0?" q-tab-panels--dark q-dark":""));return()=>(o(a),it("div",{class:C.value},n(),"pan",e.swipeable,()=>s.value))}});export{Et as Q,Mt as a,Vt as b,Dt as c,wt as u};