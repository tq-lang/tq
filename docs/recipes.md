# tq Recipes

Real-world patterns for processing JSON and TOON data with tq.

TOON sorts object keys alphabetically and table columns alphabetically. String values that would be ambiguous in TOON (such as those containing colons, or those that look like numbers, booleans, or null) are quoted.

## 1. API Responses

Simulate `curl` output by echoing realistic JSON. tq converts it to compact TOON by default.

Extract fields from an API response:

```tq
echo '{"status":"ok","data":{"userId":42,"email":"alice@example.com","plan":"pro","createdAt":"2024-01-15"}}' | tq '.data | {email, plan}'
# output
email: alice@example.com
plan: pro
```

Filter a paginated response — keep only paid plans:

```tq
echo '{"page":1,"totalPages":3,"data":[{"id":1,"username":"alice","plan":"pro"},{"id":2,"username":"bob","plan":"free"},{"id":3,"username":"carol","plan":"enterprise"}]}' | tq '[.data[] | select(.plan != "free") | {id, username, plan}]'
# output
[2]{id,plan,username}:
  1,pro,alice
  3,enterprise,carol
```

Build a compact summary from a verbose EC2-style API response:

```tq
echo '{"instanceId":"i-0abc123","instanceType":"t3.medium","state":{"name":"running"},"publicIpAddress":"54.12.34.56","placement":{"availabilityZone":"us-east-1a"}}' | tq '{id: .instanceId, type: .instanceType, state: .state.name, ip: .publicIpAddress, az: .placement.availabilityZone}'
# output
az: us-east-1a
id: i-0abc123
ip: 54.12.34.56
state: running
type: t3.medium
```

Extract and compute from a payment webhook event:

```tq
echo '{"id":"evt_001","type":"payment.succeeded","data":{"object":{"id":"pi_abc","amount":4999,"currency":"usd","customerId":"cus_xyz"}}}' | tq '{event: .type, paymentId: .data.object.id, amount: (.data.object.amount / 100), currency: .data.object.currency}'
# output
amount: 49.99
currency: usd
event: payment.succeeded
paymentId: pi_abc
```

## 2. Kubernetes-Style Output

Echo realistic `kubectl get -o json` output. tq extracts and reshapes the data.

List all containers with their image and pod name — including multi-container pods:

```tq
echo '{"apiVersion":"v1","kind":"PodList","items":[{"metadata":{"name":"frontend"},"spec":{"containers":[{"name":"app","image":"node:18"},{"name":"sidecar","image":"envoy:v1.28"}]}},{"metadata":{"name":"backend"},"spec":{"containers":[{"name":"api","image":"python:3.12"}]}}]}' | tq '[.items[] | .metadata.name as $pod | .spec.containers[] | {pod: $pod, container: .name, image}]'
# output
[3]{container,image,pod}:
  app,"node:18",frontend
  sidecar,"envoy:v1.28",frontend
  api,"python:3.12",backend
```

Find pods in the Running phase:

```tq
echo '{"apiVersion":"v1","kind":"PodList","items":[{"metadata":{"name":"web-abc","namespace":"default"},"status":{"phase":"Running"}},{"metadata":{"name":"db-xyz","namespace":"default"},"status":{"phase":"Pending"}},{"metadata":{"name":"job-001","namespace":"batch"},"status":{"phase":"Succeeded"}}]}' | tq '[.items[] | select(.status.phase == "Running") | {name: .metadata.name, ns: .metadata.namespace}]'
# output
[1]{name,ns}:
  web-abc,default
```

Deployment ready vs desired replicas:

```tq
echo '{"apiVersion":"v1","kind":"List","items":[{"metadata":{"name":"web"},"spec":{"replicas":3},"status":{"readyReplicas":3}},{"metadata":{"name":"api"},"spec":{"replicas":5},"status":{"readyReplicas":4}},{"metadata":{"name":"worker"},"spec":{"replicas":2},"status":{"readyReplicas":2}}]}' | tq '[.items[] | {name: .metadata.name, desired: .spec.replicas, ready: .status.readyReplicas}]'
# output
[3]{desired,name,ready}:
  3,web,3
  5,api,4
  2,worker,2
```

Filter resources by label:

```tq
echo '{"apiVersion":"v1","kind":"List","items":[{"metadata":{"name":"web-prod","labels":{"app":"web","env":"prod"}}},{"metadata":{"name":"web-staging","labels":{"app":"web","env":"staging"}}},{"metadata":{"name":"db-prod","labels":{"app":"db","env":"prod"}}}]}' | tq '[.items[] | select(.metadata.labels.env == "prod") | .metadata.name]'
# output
[2]: web-prod,db-prod
```

## 3. Docker-Style Output

Echo realistic `docker inspect` JSON output.

Extract key container information (simplified — real `docker inspect` output includes many more fields):

```tq
echo '{"Id":"a1b2c3d4","Name":"/web-server","State":{"Status":"running","Pid":2847},"Config":{"Image":"nginx:1.25"},"NetworkSettings":{"Networks":{"bridge":{"IPAddress":"172.17.0.3","Gateway":"172.17.0.1"}}}}' | tq '{name: .Name, status: .State.Status, image: .Config.Image, ip: .NetworkSettings.Networks.bridge.IPAddress}'
# output
image: "nginx:1.25"
ip: 172.17.0.3
name: /web-server
status: running
```

Parse environment variables into a key-value map:

```tq
echo '{"Config":{"Env":["NODE_ENV=production","PORT=3000","DB_HOST=postgres","LOG_LEVEL=info"]}}' | tq '.Config.Env | map(split("=") | {(.[0]): .[1]}) | add'
# output
DB_HOST: postgres
LOG_LEVEL: info
NODE_ENV: production
PORT: "3000"
```

List bind-mount points:

```tq
echo '{"Mounts":[{"Source":"/host/data","Destination":"/app/data","Mode":"rw","Type":"bind"},{"Source":"/host/certs","Destination":"/etc/certs","Mode":"ro","Type":"bind"}]}' | tq '[.Mounts[] | {src: .Source, dst: .Destination, mode: .Mode}]'
# output
[2]{dst,mode,src}:
  /app/data,rw,/host/data
  /etc/certs,ro,/host/certs
```

`docker ps`-style summary — truncate ID to 8 characters (simplified — `.Name` is a string in real `docker inspect` output):

```tq
echo '{"containers":[{"Id":"a1b2c3d4e5f6","Name":"/web","Image":"nginx:1.25","Status":"Up 2 hours"},{"Id":"d4e5f6a7b8c9","Name":"/db","Image":"postgres:15","Status":"Up 5 days"},{"Id":"c9d0e1f2a3b4","Name":"/cache","Image":"redis:7","Status":"Exited (0)"}]}' | tq '[.containers[] | {id: .Id[:8], name: .Name, image: .Image, status: .Status}]'
# output
[3]{id,image,name,status}:
  a1b2c3d4,"nginx:1.25",/web,Up 2 hours
  d4e5f6a7,"postgres:15",/db,Up 5 days
  c9d0e1f2,"redis:7",/cache,Exited (0)
```

## 4. Log Processing

Process JSON log lines (one per stdin line).

Filter to errors only, showing service and message:

```tq
printf '{"level":"ERROR","msg":"connection refused","ts":"2024-03-01T10:01:00Z","svc":"auth"}\n{"level":"INFO","msg":"request ok","ts":"2024-03-01T10:01:01Z","svc":"api"}\n{"level":"ERROR","msg":"timeout","ts":"2024-03-01T10:01:02Z","svc":"db"}' | tq 'select(.level == "ERROR") | {svc, msg}'
# output
msg: connection refused
svc: auth
msg: timeout
svc: db
```

Format log lines as human-readable strings with `-r`:

```tq
printf '{"level":"ERROR","msg":"connection refused","ts":"2024-03-01T10:01:00Z"}\n{"level":"WARN","msg":"slow query","ts":"2024-03-01T10:01:05Z"}' | tq -r '"\(.ts) [\(.level)] \(.msg)"'
# output
2024-03-01T10:01:00Z [ERROR] connection refused
2024-03-01T10:01:05Z [WARN] slow query
```

Count total error log lines with `-s`:

```tq
printf '{"level":"ERROR"}\n{"level":"INFO"}\n{"level":"ERROR"}\n{"level":"ERROR"}' | tq -s '[.[] | select(.level == "ERROR")] | length'
# output
3
```

Summarize log counts by level:

```tq
printf '{"level":"ERROR","svc":"auth"}\n{"level":"INFO","svc":"api"}\n{"level":"ERROR","svc":"db"}\n{"level":"WARN","svc":"cache"}\n{"level":"INFO","svc":"web"}' | tq -s '[.[].level] | group_by(.) | map({(.[0]): length}) | add'
# output
ERROR: 2
INFO: 2
WARN: 1
```

## 5. Config Transformation

Rename keys — map internal field names to an external API schema:

```tq
echo '{"first_name":"Alice","last_name":"Smith","emp_id":101,"dept":"engineering"}' | tq '{firstName: .first_name, lastName: .last_name, id: .emp_id, department: .dept}'
# output
department: engineering
firstName: Alice
id: 101
lastName: Smith
```

Merge a base config with environment overrides (second file wins):

```tq
cat <<'EOF' > base.toon
logLevel: info
maxRetries: 3
timeout: 30
EOF
cat <<'EOF' > prod.toon
dbUrl: postgres://prod-db:5432/app
logLevel: warn
maxConnections: 100
EOF
tq -s '.[0] * .[1]' base.toon prod.toon
# output
dbUrl: "postgres://prod-db:5432/app"
logLevel: warn
maxConnections: 100
maxRetries: 3
timeout: 30
```

Extract a single section from a larger config:

```tq
echo '{"server":{"host":"0.0.0.0","port":8080,"tls":true},"database":{"host":"localhost","port":5432,"name":"app"},"cache":{"ttl":300,"maxSize":1000}}' | tq '.database'
# output
host: localhost
name: app
port: 5432
```

Add defaults to a partial config:

```tq
echo '{"host":"prod.example.com"}' | tq '. + {"port": 443, "tls": true, "timeout": 30, "retries": 3}'
# output
host: prod.example.com
port: 443
retries: 3
timeout: 30
tls: true
```

Convert TOON config to compact JSON for an API request body:

```tq
printf 'image: nginx:1.25\nenv: production\nname: web-deploy\nreplicas: 3' | tq --json -c '.'
# output
{"env":"production","image":"nginx:1.25","name":"web-deploy","replicas":3}
```

## 6. Data Reshaping

Convert an array of objects into a lookup map keyed by name:

```tq
echo '{"services":[{"id":"svc-01","name":"auth","port":8001},{"id":"svc-02","name":"api","port":8002},{"id":"svc-03","name":"web","port":8003}]}' | tq '.services | map({(.name): .port}) | add'
# output
api: 8002
auth: 8001
web: 8003
```

Normalize nested data — lift customer sub-object fields to the top level:

```tq
echo '{"orders":[{"orderId":"o1","customer":{"name":"Alice","tier":"gold"},"amount":500},{"orderId":"o2","customer":{"name":"Bob","tier":"silver"},"amount":200}]}' | tq '[.orders[] | {id: .orderId, customer: .customer.name, tier: .customer.tier, amount}]'
# output
[2]{amount,customer,id,tier}:
  500,Alice,o1,gold
  200,Bob,o2,silver
```

Pivot: group events by type, collecting page names:

```tq
echo '{"events":[{"type":"click","page":"home"},{"type":"view","page":"about"},{"type":"click","page":"pricing"},{"type":"view","page":"home"}]}' | tq '.events | group_by(.type) | map({(.[0].type): map(.page)}) | add'
# output
click[2]: home,pricing
view[2]: about,home
```

Flatten team members across all teams into a single list:

```tq
echo '{"teams":[{"name":"eng","members":["Alice","Bob"]},{"name":"ops","members":["Carol","Dave"]}]}' | tq '[.teams[].members[]]'
# output
[4]: Alice,Bob,Carol,Dave
```

Flatten nested config sections into a table:

```tq
echo '{"config":{"server":{"host":"localhost","port":8080},"db":{"host":"db.local","port":5432},"cache":{"host":"redis.local","port":6379}}}' | tq '[.config | to_entries[] | {service: .key, host: .value.host, port: .value.port}]'
# output
[3]{host,port,service}:
  redis.local,6379,cache
  db.local,5432,db
  localhost,8080,server
```

## 7. CSV/TSV Generation

Generate CSV with a header row using `@csv`:

```tq
echo '{"users":[{"name":"Alice","age":30,"city":"Paris"},{"name":"Bob","age":25,"city":"London"},{"name":"Carol","age":35,"city":"Berlin"}]}' | tq -r '["name","age","city"], (.users[] | [.name, .age, .city]) | @csv'
# output
"name","age","city"
"Alice",30,"Paris"
"Bob",25,"London"
"Carol",35,"Berlin"
```

Tab-separated output with `@tsv`:

```tq
echo '{"metrics":[{"host":"web-01","cpu":45,"mem":62},{"host":"web-02","cpu":23,"mem":41},{"host":"db-01","cpu":78,"mem":89}]}' | tq -r '.metrics[] | [.host, (.cpu|tostring), (.mem|tostring)] | @tsv'
# output
web-01	45	62
web-02	23	41
db-01	78	89
```

Custom pipe-delimited output with string interpolation:

```tq
echo '{"records":[{"region":"us-east","service":"api","requests":14200},{"region":"eu-west","service":"api","requests":8900},{"region":"ap-south","service":"web","requests":3400}]}' | tq -r '.records[] | "\(.region)|\(.service)|\(.requests)"'
# output
us-east|api|14200
eu-west|api|8900
ap-south|web|3400
```

## 8. Aggregation Patterns

Count items by category:

```tq
echo '{"sales":[{"region":"us","amount":500},{"region":"eu","amount":300},{"region":"us","amount":200},{"region":"apac","amount":450},{"region":"eu","amount":150}]}' | tq '.sales | group_by(.region) | map({(.[0].region): length}) | add'
# output
apac: 1
eu: 2
us: 2
```

Sum a numeric field by group:

```tq
echo '{"sales":[{"region":"us","amount":500},{"region":"eu","amount":300},{"region":"us","amount":200},{"region":"apac","amount":450},{"region":"eu","amount":150}]}' | tq '.sales | group_by(.region) | map({(.[0].region): (map(.amount) | add)}) | add'
# output
apac: 450
eu: 450
us: 700
```

Compute the average of a numeric array:

```tq
echo '{"latencies":[23,45,12,67,34,89,15,52]}' | tq '.latencies | (add / length)'
# output
42.125
```

Top-3 products by sales volume:

```tq
echo '{"products":[{"name":"Widget","sales":1200},{"name":"Gadget","sales":3400},{"name":"Doohickey","sales":800},{"name":"Thingamajig","sales":2100},{"name":"Whatchamacallit","sales":950}]}' | tq '.products | sort_by(.sales) | reverse | .[0:3] | map({name, sales})'
# output
[3]{name,sales}:
  Gadget,3400
  Thingamajig,2100
  Widget,1200
```

Find the highest- and lowest-CPU pod in one pass:

```tq
echo '{"pods":[{"name":"web-1","cpu":120},{"name":"web-2","cpu":450},{"name":"db-1","cpu":89}]}' | tq '{highest: (.pods | max_by(.cpu) | {name, cpu}), lowest: (.pods | min_by(.cpu) | {name, cpu})}'
# output
highest:
  cpu: 450
  name: web-2
lowest:
  cpu: 89
  name: db-1
```

## 9. Working with Multiple Files

Process each TOON file independently — filter applied to each in order:

```tq
cat <<'EOF' > svc-web.toon
name: web
replicas: 3
status: healthy
EOF
cat <<'EOF' > svc-api.toon
name: api
replicas: 5
status: healthy
EOF
cat <<'EOF' > svc-db.toon
name: db
replicas: 1
status: degraded
EOF
tq '.name' svc-web.toon svc-api.toon svc-db.toon
# output
web
api
db
```

Slurp all files into an array, then query the combined data:

```tq
cat <<'EOF' > svc-web.toon
name: web
replicas: 3
status: healthy
EOF
cat <<'EOF' > svc-api.toon
name: api
replicas: 5
status: healthy
EOF
cat <<'EOF' > svc-db.toon
name: db
replicas: 1
status: degraded
EOF
tq -s 'map({name, status, replicas})' svc-web.toon svc-api.toon svc-db.toon
# output
[3]{name,replicas,status}:
  web,3,healthy
  api,5,healthy
  db,1,degraded
```

Find services that are not healthy across all files:

```tq
cat <<'EOF' > svc-web.toon
name: web
replicas: 3
status: healthy
EOF
cat <<'EOF' > svc-api.toon
name: api
replicas: 5
status: healthy
EOF
cat <<'EOF' > svc-db.toon
name: db
replicas: 1
status: degraded
EOF
tq -s 'map(select(.status == "degraded")) | map(.name)' svc-web.toon svc-api.toon svc-db.toon
# output
[1]: db
```

Rank services by replica count using slurped data:

```tq
cat <<'EOF' > svc-web.toon
name: web
replicas: 3
status: healthy
EOF
cat <<'EOF' > svc-api.toon
name: api
replicas: 5
status: healthy
EOF
cat <<'EOF' > svc-db.toon
name: db
replicas: 1
status: degraded
EOF
tq -s 'map({name, replicas}) | sort_by(.replicas) | reverse' svc-web.toon svc-api.toon svc-db.toon
# output
[3]{name,replicas}:
  api,5
  web,3
  db,1
```

## 10. Building Structured Data

Construct an object from command-line variables using `-n` and `--arg`:

```tq
tq -n --arg env production --arg version "2.1.3" '{env: $env, version: $version, deployedAt: "2024-03-01"}'
# output
deployedAt: 2024-03-01
env: production
version: 2.1.3
```

Generate test fixture data with `range`:

```tq
tq -n '[range(1;4) | {id: ., name: ("user-" + (. | tostring)), active: true}]'
# output
[3]{active,id,name}:
  true,1,user-1
  true,2,user-2
  true,3,user-3
```

Attach a structured JSON object to existing data with `--argjson`:

```tq
echo '{"name":"web","replicas":3}' | tq --argjson limits '{"cpu":"500m","memory":"256Mi"}' '. + {limits: $limits}'
# output
limits:
  cpu: 500m
  memory: 256Mi
name: web
replicas: 3
```

Enrich a TOON document with a runtime variable:

```tq
printf 'name: web\nreplicas: 3\nstatus: healthy' | tq --arg region "us-east-1" '. + {region: $region}'
# output
name: web
region: us-east-1
replicas: 3
status: healthy
```

Generate a service-name to port lookup map programmatically:

```tq
tq -n '[range(3) | {(("svc" + (. | tostring))): (8080 + .)}] | add'
# output
svc0: 8080
svc1: 8081
svc2: 8082
```

## 11. Advanced Patterns

Recursive descent `..` — find all values of a key regardless of nesting depth:

```tq
echo '{"users":[{"name":"Alice"},{"name":"Bob"}],"admin":{"name":"Carol"}}' | tq '[.. | objects | .name? // empty]'
# output
[3]: Carol,Alice,Bob
```

Collect all timeout values from an arbitrarily nested config:

```tq
echo '{"server":{"timeout":30,"retry":3},"database":{"timeout":5,"pool":10},"cache":{"timeout":60,"maxSize":1000}}' | tq '[.. | objects | .timeout? // empty]'
# output
[3]: 60,5,30
```

`try-catch` for resilient processing — emit a fallback object when a required field is absent:

```tq
printf '{"ts":"10:00","level":"ERROR","msg":"crash"}\n{"ts":"10:01","level":"INFO","msg":"start"}\n{"ts":"10:02","level":"ERROR"}' | tq 'try {ts, level, msg: (.msg | ascii_upcase)} catch {error: "malformed"}'
# output
level: ERROR
msg: CRASH
ts: "10:00"
level: INFO
msg: START
ts: "10:01"
error: malformed
```

Skip records that are missing a required field entirely using `try … catch empty`:

```tq
echo '{"items":[{"name":"Alice","score":95},{"name":"Bob"},{"name":"Carol","score":87}]}' | tq '[.items[] | try select(.score) | {name, score}]'
# output
[2]{name,score}:
  Alice,95
  Carol,87
```
