var q=Object.defineProperty,U=Object.defineProperties;var N=Object.getOwnPropertyDescriptors;var v=Object.getOwnPropertySymbols;var O=Object.prototype.hasOwnProperty,S=Object.prototype.propertyIsEnumerable;var y=(u,a,s)=>a in u?q(u,a,{enumerable:!0,configurable:!0,writable:!0,value:s}):u[a]=s,b=(u,a)=>{for(var s in a||(a={}))O.call(a,s)&&y(u,s,a[s]);if(v)for(var s of v(a))S.call(a,s)&&y(u,s,a[s]);return u},V=(u,a)=>U(u,N(a));import{a as g,b as j,Q as I}from"./QDialog.07d9a48e.js";import{Q as n}from"./QInput.75b94522.js";import{f as J,r as G,c as B,cH as i,k as p,l as c,m,q as r,t as E,G as H,x as f,s as k,Q as C,y as R,cI as $,cJ as z}from"./index.e18600f8.js";import{Q as K}from"./QForm.7e1af889.js";const L={class:"text-h6"},M={class:"tw-flex tw-flex-row-reverse tw-gap-4"},_=J({props:{modelValue:{type:Boolean,default:!1},user:{type:Object,default:null}},emits:["update:modelValue","modify"],setup(u,{emit:a}){const s=u,w=s.user!=null?V(b({},s.user),{password:"",passwordAgain:"",newPassword:""}):{id:void 0,account:"",password:"",newPassword:"",passwordAgain:"",name:""},e=G(Object.assign({},w)),D=B(()=>JSON.stringify(w)===JSON.stringify(e.value)),x=i.getters.isAdmin,A=()=>{e.value=Object.assign({},w)},d=B({get(){return s.modelValue},set(t){t||A(),a("update:modelValue",t)}}),F=async()=>{try{if(e.value.newPassword!==e.value.passwordAgain){i.commit("error",{error:new Error("\u4E24\u6B21\u5BC6\u7801\u4E0D\u4E00\u81F4")});return}(await $({userID:e.value.id?e.value.id:0,password:e.value.password,newPassword:e.value.newPassword})).meta.code===0&&(i.commit("success",{message:"\u4FEE\u6539\u6210\u529F"}),d.value=!1,a("modify",{}))}catch(t){console.log(t)}},Q=async()=>{try{if(e.value.password!==e.value.passwordAgain){i.commit("error",{error:new Error("\u4E24\u6B21\u5BC6\u7801\u4E0D\u4E00\u81F4")});return}(await z({account:e.value.account,password:e.value.password,name:e.value.name})).meta.code===0&&(i.commit("success",{message:"\u65B0\u589E\u6210\u529F"}),d.value=!1,a("modify",{}))}catch(t){console.log(t)}},P=async()=>s.user?await F():await Q();return(t,l)=>(p(),c(I,{modelValue:f(d),"onUpdate:modelValue":l[6]||(l[6]=o=>R(d)?d.value=o:null),"no-backdrop-dismiss":!f(D)},{default:m(()=>[r(j,{style:{width:"300px"}},{default:m(()=>[r(g,null,{default:m(()=>[E("div",L,H(u.user?"\u7F16\u8F91":"\u65B0\u589E"),1)]),_:1}),r(g,{class:"q-pt-none"},{default:m(()=>[r(K,{class:"tw-w-full",onSubmit:P},{default:m(()=>[r(n,{key:"name",modelValue:e.value.name,"onUpdate:modelValue":l[0]||(l[0]=o=>e.value.name=o),disable:e.value.id!==void 0,type:"textarea",label:"\u540D\u79F0",autogrow:"",square:"",filled:"",class:"tw-mb-4"},null,8,["modelValue","disable"]),r(n,{key:"account",modelValue:e.value.account,"onUpdate:modelValue":l[1]||(l[1]=o=>e.value.account=o),disable:e.value.id!==void 0,type:"text",label:"\u8D26\u53F7(\u90AE\u7BB1)",square:"",filled:"",class:"tw-mb-4"},null,8,["modelValue","disable"]),e.value.id===void 0||!f(x)?(p(),c(n,{key:"password_again",modelValue:e.value.password,"onUpdate:modelValue":l[2]||(l[2]=o=>e.value.password=o),type:"password",label:e.value.id===void 0?"\u5BC6\u7801":"\u65E7\u5BC6\u7801",square:"",filled:"",class:"tw-mb-4"},null,8,["modelValue","label"])):k("",!0),e.value.id!==void 0?(p(),c(n,{key:"password",modelValue:e.value.newPassword,"onUpdate:modelValue":l[3]||(l[3]=o=>e.value.newPassword=o),type:"password",label:"\u65B0\u5BC6\u7801",square:"",filled:"",class:"tw-mb-4"},null,8,["modelValue"])):k("",!0),r(n,{key:"password_again",modelValue:e.value.passwordAgain,"onUpdate:modelValue":l[4]||(l[4]=o=>e.value.passwordAgain=o),type:"password",label:"\u786E\u8BA4\u65B0\u5BC6\u7801",square:"",filled:"",class:"tw-mb-4"},null,8,["modelValue"]),E("div",M,[r(C,{color:"primary","text-color":"black",type:"submit",label:"\u63D0\u4EA4",class:"lg:tw-w-24 tw-w-full"}),r(C,{flat:"",type:"reset",label:"\u53D6\u6D88",class:"lg:tw-w-24 tw-w-full",onClick:l[5]||(l[5]=o=>d.value=!1)})])]),_:1})]),_:1})]),_:1})]),_:1},8,["modelValue","no-backdrop-dismiss"]))}});export{_};
