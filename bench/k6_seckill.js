import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  vus: Number(__ENV.VUS || 200),
  duration: __ENV.DURATION || "30s",
  thresholds: {
    http_req_failed: ["rate<0.05"],
    http_req_duration: ["p(95)<200"],
  },
};

const base = __ENV.BASE_URL || "http://127.0.0.1:8080";
const activityId = Number(__ENV.ACTIVITY_ID || 1001);
const userBase = Number(__ENV.USER_BASE || 100000);

export default function () {
  const uid = "k6_" + (userBase + __VU * 1000000 + __ITER);

  const body = JSON.stringify({
    user_id: uid,
    activity_id: activityId,
  });

  const res = http.post(base + "/api/seckill", body, {
    headers: { "Content-Type": "application/json" },
  });

  check(res, {
    "status is 200": (r) => r.status === 200,
    "code is 0/1001/1002": (r) => {
      if (!r.body) return false;
      return r.body.includes('"code":0') || r.body.includes('"code":1001') || r.body.includes('"code":1002');
    },
  });

  sleep(0.05);
}
