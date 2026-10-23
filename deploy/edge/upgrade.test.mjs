import {test} from 'node:test';
import assert from 'node:assert/strict';
import worker from './worker.mjs';
test('WebSocket upgrade response is passed through untouched',async()=>{
 const original=globalThis.fetch;
 const socket={status:101,webSocket:{}};
 try{
 globalThis.fetch=async()=>socket;
 assert.equal(await worker.fetch(new Request('https://app.syntelle.online/api/v1/streams/fixture/ws',{headers:{Upgrade:'websocket'}}),{API_ORIGIN:'https://api.fixture.azurecontainerapps.io',EDGE_TOKEN:'fixture'}),socket);
 } finally{globalThis.fetch=original}
});
