const publicPath = /^\/api\/v1\/(?:auth(?:\/|$)|configurations(?:\/|$)|keys(?:\/|$)|account(?:\/|$)|streams(?:\/|$)|conversations(?:\/|$))/;
export default {
 async fetch(request, env) {
  const incoming = new URL(request.url);
 if(incoming.hostname !== 'app.syntelle.online') return new Response('Unavailable',{status:404});
  if (!incoming.pathname.startsWith('/api/')) return env.ASSETS.fetch(request);
  if (!publicPath.test(incoming.pathname)) return new Response('Unavailable', {status:404});
  const target = new URL(env.API_ORIGIN);
  if (target.protocol !== 'https:' || !target.hostname.endsWith('.azurecontainerapps.io')) return new Response('Invalid origin', {status:503});
  target.pathname=incoming.pathname; target.search=incoming.search;
  const headers=new Headers(request.headers);
  for(const key of [...headers.keys()]) if (/^(host$|forwarded$|x-forwarded-|x-real-ip$|true-client-ip$|cf-connecting-ip$|x-syntelle-edge-|x-syntelle-client-)/i.test(key)) headers.delete(key);
  headers.set('X-Syntelle-Edge-Token',env.EDGE_TOKEN);
  headers.set('X-Syntelle-Client-IP',request.headers.get('CF-Connecting-IP') || '');
  const response=await fetch(new Request(target, {method:request.method,headers,body:['GET','HEAD'].includes(request.method)?undefined:request.body,redirect:'manual'}));
  // Passing the original 101 response preserves the WebSocket handle.
  if(response.status===101) return response;
  const outgoing=new Response(response.body,response);
  outgoing.headers.set('Cache-Control','no-store');
  return outgoing;
 }
};
