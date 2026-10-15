# production 环境部署说明

拓扑: **infra 不进 swarm, 业务进 swarm**。

```
宿主机(单机演示; 三机部署见 infra.yml 注释)
├── infra (docker compose, bridge prod-game-smash-infra)
│   ├── prod-etcd1    宿主机 12379
│   ├── prod-etcd2    宿主机 22379
│   ├── prod-etcd3    宿主机 32379
│   ├── prod-redis    宿主机 6379
│   └── prod-mongodb  宿主机 27017
└── swarm stack gameprod (overlay game-overlay)
    ├── gate-1  WS 7001 / pprof 7051
    ├── gate-2  WS 7002 / pprof 7052
    ├── room-1  pprof 7081
    ├── room-2  pprof 7082
    ├── home-1  pprof 7061
    └── home-2  pprof 7063
```

与 dev 的区别: etcd 为 **3 节点集群**(`ETCD_INITIAL_CLUSTER` 互指),
业务端 `ETCD_ADDRESS` 配 3 个地址, 挂掉 1 个节点仍可用;
`isdevelop = false`, redis `cacheprefix = "prod"`,
业务栈资源上限与重启次数更高。
