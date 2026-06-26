import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = 'http://127.0.0.1:8080';
const TOKEN = 'Bearer YOUR_TOKEN';

export const options = {
  stages: [
    { duration: '10s', target: 20 },
    { duration: '10s', target: 0 },
  ],
};

export default function () {
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': TOKEN,
  };

  const createOrderPayload = JSON.stringify({
    user_id: 1,
    total: 2500000,
    items: [
      { product_id: 1, quantity: 1, price: 1500000 },
      { product_id: 3, quantity: 2, price: 500000 },
    ],
  });

  const updateOrderPayload = JSON.stringify({
    status: 'paid',
  });

  const responses = http.batch([
    ['GET', `${BASE_URL}/api/products`, null, { headers }],
    ['GET', `${BASE_URL}/api/product/5`, null, { headers }],
    ['GET', `${BASE_URL}/api/product/7`, null, { headers }],

    // Order service routes
    // ['GET', `${BASE_URL}/api/orders`, null, { headers }],
    // ['GET', `${BASE_URL}/api/order/1`, null, { headers }],
    // ['POST', `${BASE_URL}/api/order/create`, createOrderPayload, { headers }],
  ]);

  check(responses[0], { '[gateway] GET products 200': (r) => r.status === 200 });
  check(responses[1], { '[gateway] GET product/5 200': (r) => r.status === 200 });
  check(responses[2], { '[gateway] GET product/7 200': (r) => r.status === 200 });

  // check(responses[3], { '[gateway] GET orders 200': (r) => r.status === 200 });
  // check(responses[4], { '[gateway] GET order/1 200': (r) => r.status === 200 });
  // check(responses[5], { '[gateway] POST order/create 201/200': (r) => r.status === 201 || r.status === 200 });

  sleep(1);
}