import http from 'k6/http';
import { check, sleep } from 'k6';

// const BASE_URL = 'http://127.0.0.1:8080';
const TOKEN = 'Bearer YOUR_TOKEN';

export const options = {
  stages: [
    { duration: '5s', target: 10 },  // tăng lên 20 users
    { duration: '5s', target: 10 },
    { duration: '5s', target: 0 },  // giảm về 0
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
    ['GET', `http://host.docker.internal:3001/api/products`, null, { headers }],
    ['GET', `http://host.docker.internal:3003/api/products`, null, { headers }],
    ['GET', `http://host.docker.internal:3004/api/products`, null, { headers }],
    ['GET', `http://host.docker.internal:3005/api/products`, null, { headers }],
  ]);

  // Check từng response
  check(responses[0], { '[service:3001] GET products 200': (r) => r.status === 200 });
  check(responses[1], { '[service:3003] GET products 200': (r) => r.status === 200 });
  check(responses[2], { '[service:3004] GET products 200': (r) => r.status === 200 });
  check(responses[3], { '[service:3005] GET products 200': (r) => r.status === 200 });
  sleep(1);
}