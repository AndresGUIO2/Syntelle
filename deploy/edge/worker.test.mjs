import { test } from 'node:test';
import assert from 'node:assert/strict';
import worker from './worker.mjs';
const env={API_ORIGIN:'https://syntelle.example.azurecontainerapps.io',EDGE_TOKEN:'fixture',ASSETS:{fetch:()=>new Response('asset')}};
test('static assets and forbidden private routes',async()=>{
 assert.equal(await (await worker.fetch(new Request('https://app.syntelle.online/'),env)).text(),'asset');
 for(const path of ['stream-worker/ws','speech/synthesize','conversation/ws']) assert.equal((await worker.fetch(new Request('https://app.syntelle.online/api/v1/'+path),env)).status,404);
});
test('proxy preserves cookies, redirects, errors and strips forged ingress',async()=>{
 const original=globalThis.fetch;
 try {
 globalThis.fetch=async r=>{
 assert.equal(r.headers.get('X-Syntelle-Edge-Token'),'fixture');
 assert.equal(r.headers.get('X-Forwarded-For'),null);
 assert.equal(r.headers.get('Host'),null);
 assert.equal(r.headers.get('X-Real-IP'),null);
 assert.equal(r.headers.get('X-Syntelle-Client-IP'),'203.0.113.8');
 assert.equal(r.headers.get('Cookie'),'syntelle_session=test');
 assert.equal(r.redirect,'manual');
 return new Response('upstream',{status:303,headers:{Location:'https://app.syntelle.online/', 'Set-Cookie':'syntelle_session=next; HttpOnly; Secure; SameSite=Lax'}});
 };
 const res=await worker.fetch(new Request('https://app.syntelle.online/api/v1/auth/google/callback',{headers:{Host:'app.syntelle.online',Cookie:'syntelle_session=test','CF-Connecting-IP':'203.0.113.8','X-Forwarded-For':'forged','X-Real-IP':'forged','X-Syntelle-Edge-Token':'forged'}}),env);
 assert.equal(res.status,303);assert.match(res.headers.get('Set-Cookie'),/HttpOnly/);assert.equal(res.headers.get('Cache-Control'),'no-store');
 } finally {globalThis.fetch=original}
});

test('API error status and payload remain intact without caching',async()=>{
 const original=globalThis.fetch;
 try {
 globalThis.fetch=async()=>new Response('{"error":{"message":"capacity busy"}}',{status:503,headers:{'Content-Type':'application/json','Retry-After':'5'}});
 const response=await worker.fetch(new Request('https://app.syntelle.online/api/v1/streams',{method:'POST'}),env);
 assert.equal(response.status,503);
 assert.equal(response.headers.get('Retry-After'),'5');
 assert.equal(response.headers.get('Cache-Control'),'no-store');
 assert.equal((await response.json()).error.message,'capacity busy');
 } finally {globalThis.fetch=original}
});
