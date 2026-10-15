# dev 环境部署说明

拓扑: **infra 不进 swarm, 业务进 swarm**。

```
宿主机
├── infra (docker compose, bridge dev-game-smash-infra)
│   ├── dev-etcd      单节点, 宿主机 2379/2380
│   ├── dev-redis     宿主机 6379
│   └── dev-mongodb   宿主机 27017
└── swarm stack gamedev (overlay game-overlay)
    ├── gate-1  WS 7001 / pprof 7051
    ├── gate-2  WS 7002 / pprof 7052
    ├── room-1  pprof 7081
    ├── room-2  pprof 7082
    ├── home-1  pprof 7061
    └── home-2  pprof 7063
```

swarm task 经 `host.docker.internal:host-gateway` 访问宿主机 infra 端口。
RPC 端口是动态的(`:0`), 经 overlay 网络互联, 在 etcd 注册的是容器 overlay IP,
无需发布。

## 首次部署

```bash
cd docker/dev
cp .env.example .env          # 按需改镜像名/密码/端口
bash deploy.sh all            # Windows: powershell -File deploy.ps1
```

脚本会做: `docker swarm init`(如未初始化) → 创建 overlay 网络 →
`docker compose -f infra.yml up -d` → 等 etcd 健康 → `docker stack deploy`。

业务镜像需提前准备(仓库根目录):

```bash
make image-gate image-room image-home
# 推送到仓库后, 把 .env 里的 GATE_IMAGE/ROOM_IMAGE/HOME_IMAGE 改为仓库地址
```

## 日常操作

```bash
bash deploy.sh infra    # 只重起 infra
bash deploy.sh stack    # 只重部署业务栈(改了 config/*.toml 或镜像后用这个)

docker stack services gamedev
docker service logs gamedev_gate-1 --tail 100 -f
docker stack rm gamedev
```

## 配置说明

- `config/*.toml` 是 swarm config 的源文件, `stack.yml` 会将其下发到
  task 的 `/app/config/*.toml`(代码读取路径 `./config/`)。
  改完 toml 后重跑 `deploy.sh stack` 即可生效。
- 环境变量优先级高于 toml:
  `ETCD_ADDRESS` > `etcdaddress`, `REDIS_ADDR` > `redisAddr`,
  `MONGO_URI` > `mongodb.uri`, `NODE_NAME` 决定起哪个节点
  (同一镜像可起 gate-1/gate-2)。
- `cacheprefix = "dev"` 用于多环境共用 redis 时隔离 key。
- excels 已烘进业务镜像(Dockerfile `COPY excels`), 不挂载 volume,
  多节点无需同步文件; 日志走容器 stdout, 用 `docker service logs` 查看。

## 排错

- task 访问不到 infra: 检查宿主机防火墙是否放行 2379/6379/27017,
  以及 docker 是否支持 `host-gateway`(Docker 20.10+)。
  不支持时把 `.env` 的 `HOST_IP` 改为宿主机网卡 IP,
  并同步替换 `config/*.toml` 与 `stack.yml` 中的 `host.docker.internal`。
- etcd 起不来: `docker logs dev-etcd`, 检查 2379/2380 是否被占用。
- 端口冲突: 改 `.env` 中的 `ETCD_PORT/REDIS_PORT/MONGO_PORT`,
  并同步改 `config/*.toml` 与 `stack.yml` 的 environment。
