set -euo pipefail

WEBHOOK='https://oapi.dingtalk.com/robot/send?access_token=2781bf51cfedea75017e650c381b9f44e581400fbde34d20c469dd93f912c506'
SECRET='SEC08f44ab8f5ca5e16a96593cc9389d778f4bdc4e7a6bafaa190a96c2a05c986ec'

# 毫秒时间戳（Linux一般有；如果你的系统没有%3N，告诉我我给你兼容写法）
TS="$(date +%s%3N)"

SIGN="$(TS="$TS" SECRET="$SECRET" python3 - << 'PY'
import os, hmac, hashlib, base64, urllib.parse
ts = os.environ["TS"]
secret = os.environ["SECRET"]
string_to_sign = f"{ts}\n{secret}"
digest = hmac.new(secret.encode("utf-8"),
                  string_to_sign.encode("utf-8"),
                  hashlib.sha256).digest()
print(urllib.parse.quote(base64.b64encode(digest)))
PY
)"

curl -sS -X POST "${WEBHOOK}&timestamp=${TS}&sign=${SIGN}" \
  -H 'Content-Type: application/json' \
  -d '{"msgtype":"text","text":{"content":"【A股快讯】机器人联通测试"}}'
echo
