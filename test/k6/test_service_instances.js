import http from 'k6/http';
import { check, sleep } from 'k6';

// const BASE_URL = 'http://127.0.0.1:8080';
const TOKEN = 'Bearer YOUR_TOKEN';

export const options = {
  stages: [
    { duration: '10s', target: 20 },  // tăng lên 20 users
    { duration: '10s', target: 10 },
    { duration: '10s', target: 0 },  // giảm về 0
  ],
};

// Mỗi user sẽ chạy function này lặp đi lặp lại
export default function () {
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': TOKEN,
  };

  // Gửi đồng thời nhiều endpoint trong 1 vòng lặp
  const responses = http.batch([
    ['GET', `http://localhost:3001/api/products`, null, { headers }],
    ['GET', `http://localhost:3003/api/products`, null, { headers }],
    ['GET', `http://localhost:3004/api/products`, null, { headers }],
    ['GET', `http://localhost:3005/api/products`, null, { headers }],
  ]);

  // Check từng response
  check(responses[0], { '[service:3001] GET products 200': (r) => r.status === 200 });
  check(responses[1], { '[service:3003] GET products 200': (r) => r.status === 200 });
  check(responses[2], { '[service:3004] GET products 200': (r) => r.status === 200 });
  check(responses[3], { '[service:3005] GET products 200': (r) => r.status === 200 });
  sleep(1);
}