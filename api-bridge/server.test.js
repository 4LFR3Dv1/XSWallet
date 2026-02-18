const test = require('node:test');
const assert = require('node:assert/strict');

const { app, _test } = require('./server');

let server;
let baseUrl;

test.beforeEach(async () => {
    server = app.listen(0);
    await new Promise((resolve) => server.once('listening', resolve));
    const { port } = server.address();
    baseUrl = `http://127.0.0.1:${port}`;
});

test.afterEach(async () => {
    await new Promise((resolve, reject) => {
        server.close((err) => (err ? reject(err) : resolve()));
    });
});

test('protected route requires Authorization header', async () => {
    let called = false;
    _test.setClientsForTest({
        walletClient: {
            GetAllBalances: () => {
                called = true;
            },
        },
    });

    const res = await fetch(`${baseUrl}/api/v1/wallet/balances`);
    assert.equal(res.status, 401);
    assert.equal(called, false);

    const body = await res.json();
    assert.equal(body.code, 'UNAUTHENTICATED');
});

test('GET /api/v1/swaps/:id accepts direct SwapSnapshot response shape', async () => {
    _test.setClientsForTest({
        swapClient: {
            GetSwap: (_req, _metadata, _opts, cb) => {
                cb(null, {
                    id: 'swap-1',
                    state: 'SWAP_STATE_OPEN',
                    amount_sat: '1000',
                    from_chain: 'CHAIN_BTC',
                    to_chain: 'CHAIN_LN',
                });
            },
        },
    });

    const res = await fetch(`${baseUrl}/api/v1/swaps/swap-1`, {
        headers: {
            Authorization: 'Bearer test-session',
        },
    });

    assert.equal(res.status, 200);
    const body = await res.json();
    assert.equal(body.id, 'swap-1');
    assert.equal(body.state, 'PENDING_FUNDING');
});
