const publicPath = /^\/api\/v1\/(?:auth(?:\/|$)|configurations(?:\/|$)|keys(?:\/|$)|account(?:\/|$)|streams(?:\/|$)|conversations(?:\/|$)|characters(?:\/|$)|workspaces(?:\/|$)|admin(?:\/|$)|widgets(?:\/|$)|billing\/paddle\/webhook$)/;
const consolePage = /^\/(?:console(?:\/|$)|characters(?:\/|$)|pricing\/?$|embed\/[a-zA-Z0-9_-]+\/?$)/;

function ancestors(values) {
 if(!Array.isArray(values)) return "'none'";
 return [...new Set(values.filter(value=>{
  try { const url=new URL(value);return url.origin===value&&url.protocol==='https:'&&!url.username&&!url.password; } catch{return false;}
 }))].join(' ') || "'none'";
}
async function upstream(request, env) {
 const incoming=new URL(request.url), target=new URL(env.API_ORIGIN);
 if(target.protocol!=='https:'||!target.hostname.endsWith('.azurecontainerapps.io')) return new Response('Invalid origin',{status:503});
 target.pathname=incoming.pathname;target.search=incoming.search;
 const headers=new Headers(request.headers);
 for(const key of [...headers.keys()]) if (/^(host$|forwarded$|x-forwarded-|x-real-ip$|true-client-ip$|cf-connecting-ip$|x-syntelle-edge-|x-syntelle-client-)/i.test(key)) headers.delete(key);
 headers.set('X-Syntelle-Edge-Token',env.EDGE_TOKEN);
 headers.set('X-Syntelle-Client-IP',request.headers.get('CF-Connecting-IP') || '');
 return fetch(new Request(target,{method:request.method,headers,body:['GET','HEAD'].includes(request.method)?undefined:request.body,redirect:'manual'}));
}
export default {
 async fetch(request, env) {
  const incoming = new URL(request.url);
 if(incoming.hostname !== 'app.syntelle.online') return new Response('Unavailable',{status:404});
  if (!incoming.pathname.startsWith('/api/')) {
   let policy="'none'";
   const embedded=/^\/embed\/([a-zA-Z0-9_-]+)\/?$/.exec(incoming.pathname);
   if(embedded){
    const metaURL=new URL(`/api/v1/widgets/${embedded[1]}`,incoming);
    const metadata=await upstream(new Request(metaURL,{headers:{'CF-Connecting-IP':request.headers.get('CF-Connecting-IP')||''}}),env);
    if(!metadata.ok) return new Response('This character is unavailable.',{status:metadata.status,headers:{'Content-Security-Policy':"frame-ancestors 'none'",'Cache-Control':'no-store'}});
    policy=ancestors((await metadata.json()).allowed_origins);
   }
   let asset=await env.ASSETS.fetch(request);
   if(asset.status===404&&consolePage.test(incoming.pathname)) asset=await env.ASSETS.fetch(new Request(new URL('/index.html',incoming),request));
   const result=new Response(asset.body,asset);
   result.headers.set('Content-Security-Policy',`frame-ancestors ${policy}; object-src 'none'; base-uri 'self'`);
   if(embedded){ result.headers.delete('X-Frame-Options');result.headers.set('Cache-Control','no-store'); }
   else result.headers.set('X-Frame-Options','DENY');
   return result;
  }
  if (!publicPath.test(incoming.pathname)) return new Response('Unavailable', {status:404});
  const response=await upstream(request,env);
  // Passing the original 101 response preserves the WebSocket handle.
  if(response.status===101) return response;
  const outgoing=new Response(response.body,response);
  outgoing.headers.set('Cache-Control','no-store');
  return outgoing;
 }
};
