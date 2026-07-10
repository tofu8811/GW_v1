import http from 'k6/http';
import { check, group, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://host.docker.internal:8080';
const UPSTREAM_PRODUCT_URL = __ENV.UPSTREAM_PRODUCT_URL || 'http://host.docker.internal:3001';
const TOKEN = __ENV.TOKEN || 'Bearer YOUR_TOKEN';
const INVALID_TOKEN = __ENV.INVALID_TOKEN || 'Bearer invalid-demo-token';

const AUTH_401_PATH = __ENV.AUTH_401_PATH || '/api/orders';
const AUTH_403_PATH = __ENV.AUTH_403_PATH || '/api/orders';
const NOT_FOUND_PATH = __ENV.NOT_FOUND_PATH || '/api/not-found-demo';
const RATE_LIMIT_PATH = __ENV.RATE_LIMIT_PATH || '/api/products';
const RATE_LIMIT_BURST = Number(__ENV.RATE_LIMIT_BURST || 120);

const EXPECT_AUTH_ENFORCED = (__ENV.EXPECT_AUTH_ENFORCED || 'false').toLowerCase() === 'true';
const EXPECT_RATE_LIMIT = (__ENV.EXPECT_RATE_LIMIT || 'false').toLowerCase() === 'true';
const COMPARE_UPSTREAM = (__ENV.COMPARE_UPSTREAM || 'true').toLowerCase() === 'true';

export const options = {
  stages: [
    { duration: '10s', target: 5 },
    { duration: '10s', target: 5 },
    { duration: '15s', target: 5 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.20'],
    http_req_duration: ['p(95)<3000'],
  },
};

function jsonHeaders(token) {
  return {
    'Content-Type': 'application/json',
    Authorization: token || TOKEN,
  };
}

function statusIs(response, expected) {
  if (Array.isArray(expected)) {
    return expected.indexOf(response.status) !== -1;
  }

  return response.status === expected;
}

function checkStatus(response, label, expected) {
  const checks = {};
  checks[label] = function (r) {
    return statusIs(r, expected);
  };

  return check(response, checks);
}

export default function () {
  const headers = jsonHeaders();

  group('gateway public proxy success', function () {
    const responses = http.batch([
      ['GET', BASE_URL + '/api/products', null, { headers: headers, tags: { case: 'products-list' } }],
      ['GET', BASE_URL + '/api/product/2', null, { headers: headers, tags: { case: 'product-id-2' } }],
      ['GET', BASE_URL + '/api/product/3', null, { headers: headers, tags: { case: 'product-id-3' } }],
    ]);

    checkStatus(responses[0], '[gateway] GET /api/products -> 200', 200);
    checkStatus(responses[1], '[gateway] GET /api/product/2 path param -> 200', 200);
    checkStatus(responses[2], '[gateway] GET /api/product/3 path param -> 200', 200);
  });

  group('gateway 404 route miss', function () {
    const response = http.get(BASE_URL + NOT_FOUND_PATH, {
      headers: headers,
      tags: { case: 'gateway-404' },
    });

    checkStatus(response, '[gateway] GET ' + NOT_FOUND_PATH + ' -> 404', 404);
  });

  group('gateway 401/403 auth demo', function () {
    const noToken = http.get(BASE_URL + AUTH_401_PATH, {
      headers: { 'Content-Type': 'application/json' },
      tags: { case: 'auth-401' },
    });
    const badToken = http.get(BASE_URL + AUTH_403_PATH, {
      headers: jsonHeaders(INVALID_TOKEN),
      tags: { case: 'auth-403' },
    });

    const authExpected401 = EXPECT_AUTH_ENFORCED ? 401 : [200, 401, 403, 502];
    const authExpected403 = EXPECT_AUTH_ENFORCED ? [403, 401] : [200, 401, 403, 502];

    checkStatus(noToken, '[gateway] GET ' + AUTH_401_PATH + ' without token -> 401 when auth is enforced', authExpected401);
    checkStatus(badToken, '[gateway] GET ' + AUTH_403_PATH + ' invalid token -> 403/401 when auth is enforced', authExpected403);
  });

  group('gateway 429 rate limit demo', function () {
    let saw429 = false;

    for (let i = 0; i < RATE_LIMIT_BURST; i += 1) {
      const response = http.get(BASE_URL + RATE_LIMIT_PATH, {
        headers: headers,
        tags: { case: 'rate-limit-burst' },
      });

      if (response.status === 429) {
        saw429 = true;
        break;
      }
    }

    const checks = {};
    checks['[gateway] burst ' + RATE_LIMIT_BURST + ' requests to ' + RATE_LIMIT_PATH + ' -> 429 when rate limit is enforced'] =
      function (r) {
        return EXPECT_RATE_LIMIT ? r.saw429 === true : true;
      };

    check({ saw429: saw429 }, checks);
  });

  if (COMPARE_UPSTREAM) {
    group('compare gateway rewrite with product-service upstream', function () {
      const gatewayProducts = http.get(BASE_URL + '/api/products', {
        headers: headers,
        tags: { case: 'gateway-products' },
      });
      const upstreamProducts = http.get(UPSTREAM_PRODUCT_URL + '/api/products', {
        headers: headers,
        tags: { case: 'upstream-products' },
      });
      const gatewayProduct = http.get(BASE_URL + '/api/product/2', {
        headers: headers,
        tags: { case: 'gateway-product-2' },
      });
      const upstreamProduct = http.get(UPSTREAM_PRODUCT_URL + '/api/product/2', {
        headers: headers,
        tags: { case: 'upstream-product-2' },
      });

      check(gatewayProducts, {
        '[compare] gateway /api/products status equals upstream /api/products': function () {
          return gatewayProducts.status === upstreamProducts.status;
        },
      });
      check(gatewayProduct, {
        '[compare] gateway /api/product/2 status equals upstream /api/product/2': function () {
          return gatewayProduct.status === upstreamProduct.status;
        },
      });
    });
  }

  sleep(1);
}
