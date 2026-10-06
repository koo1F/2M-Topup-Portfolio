// HTTP integration check: real frontend proxy, Go API, SQL, Redis and worker.
// Start the e2e provider fixture first; no real Stripe account is contacted.
import assert from 'node:assert/strict';
import { writeFileSync } from 'node:fs';

const frontend = process.env.E2E_FRONTEND_URL ?? 'http://127.0.0.1:13000';
const api = process.env.E2E_API_URL ?? 'http://127.0.0.1:18080';
const provider = 'http://127.0.0.1:19090';
const results = [];
async function call(base, path, { method = 'GET', data, cookie, headers = {} } = {}) {
 const response = await fetch(base + path, { method, redirect: 'manual', headers: { ...(data ? {'Content-Type':'application/json'} : {}), ...(cookie ? {Cookie: cookie} : {}), ...headers }, body: data ? JSON.stringify(data) : undefined });
 const text = await response.text();
 let body; try { body = JSON.parse(text); } catch { body = text; }
 return { status: response.status, body, headers: response.headers };
}
async function eventually(action, expected) {
 for (let i = 0; i < 50; i++) { const value = await action(); if (value === expected) return; await new Promise(resolve => setTimeout(resolve, 100)); }
 assert.equal(await action(), expected);
}
const pass = name => { results.push({check:name,status:'passed'}); console.log(`PASS ${name}`); };
const suffix = Date.now(); const password = 'PortfolioDemo123!';
const landing = await call(frontend, '/'); assert.equal(landing.status, 200); assert.match(landing.body, /Money, elevated/); pass('public landing renders through Next.js');
assert.equal((await call(frontend, '/dashboard')).status, 307); pass('anonymous dashboard redirects');
const email = `http-${suffix}@example.com`;
assert.equal((await call(frontend, '/api/auth/register', {method:'POST',data:{email,password}})).status, 200);
const login = await call(frontend, '/api/auth/login', {method:'POST',data:{email,password}});
assert.equal(login.status,200); const setCookie = login.headers.get('set-cookie'); assert.match(setCookie,/HttpOnly/i);
const cookie = setCookie.split(';')[0]; const token = decodeURIComponent(cookie.slice('token='.length)); pass('register and login through frontend; HTTP-only cookie');
const wallet = () => call(frontend, '/api/wallet',{cookie}); assert.equal((await wallet()).body.balance,0);
const key = `http-flow-${suffix}`; const headers = {'Idempotency-Key':key}; const data={amount:100,method:'card'};
const created = await call(frontend,'/api/payment/create',{method:'POST',cookie,headers,data}); assert.equal(created.status,200); assert.equal(created.body.status,'PENDING');
const pay = created.body; assert.match(pay.redirect_url,/^http:\/\/127\.0\.0\.1:19090\/checkout\//);
const retry = await call(frontend,'/api/payment/create',{method:'POST',cookie,headers,data}); assert.equal(retry.body.payment_id,pay.payment_id); pass('same user retry returns original payment');
assert.equal((await call(frontend,'/api/payment/create',{method:'POST',cookie,headers,data:{amount:200,method:'card'}})).status,409);
assert.equal((await call(frontend,'/api/payment/create',{method:'POST',cookie,headers,data:{amount:100,method:'promptpay'}})).status,409); pass('changed amount or method rejected with 409');
const completed=await fetch(pay.redirect_url,{method:'POST',redirect:'manual'});assert.equal(completed.status,303);
await eventually(async()=> (await call(frontend,`/api/payment/${pay.payment_id}/status`,{cookie})).body.status,'SUCCESS');
await eventually(async()=> (await wallet()).body.balance,100);pass('signed provider event settles through Redis and worker');
const session=`cs_test_${pay.payment_id}`;
assert.equal((await call(provider,`/replay/${session}`,{method:'POST'})).status,200);
assert.equal((await call(provider,`/replay/${session}?new_event=1`,{method:'POST'})).status,200);
await new Promise(resolve=>setTimeout(resolve,500));
const tx = await call(frontend,'/api/wallet/transactions',{cookie});assert.equal(tx.body.pagination.total,1);assert.equal((await wallet()).body.balance,100);pass('same and different event IDs cannot double-credit; one transaction');
const otherEmail=`http-other-${suffix}@example.com`;
assert.equal((await call(api,'/api/auth/register',{method:'POST',data:{email:otherEmail,password}})).status,201);
const otherLogin=await call(api,'/api/auth/login',{method:'POST',data:{email:otherEmail,password}});
const otherHeaders={Authorization:`Bearer ${otherLogin.body.token}`,'Idempotency-Key':key};
assert.equal((await call(api,`/api/payment/${pay.payment_id}/status`,{headers:otherHeaders})).status,404);pass('non-owner denied even with warmed status cache');
const otherPay=await call(api,'/api/payment/create',{method:'POST',headers:otherHeaders,data});assert.equal(otherPay.status,201);assert.notEqual(otherPay.body.payment_id,pay.payment_id);pass('different users can reuse the same key independently');
assert.equal((await call(api,'/api/webhooks/stripe',{method:'POST',data:{},headers:{'X-Gateway-Signature':'test'}})).status,401);pass('mock webhook disabled in production service');
assert.equal((await call(api,'/api/payment/create',{method:'POST',headers:{Authorization:`Bearer ${token}`,'Idempotency-Key':`invalid-${suffix}`},data:{amount:10.001,method:'card'}})).status,400);pass('invalid monetary precision rejected before creation');
if (process.env.E2E_REPORT_PATH) writeFileSync(process.env.E2E_REPORT_PATH,JSON.stringify({provider:'local signed Stripe SDK fixture',database:process.env.E2E_DATABASE_DESCRIPTION??'PostgreSQL',checks:results},null,2)+'\n');
console.log(`All ${results.length} HTTP integration checks passed.`);
