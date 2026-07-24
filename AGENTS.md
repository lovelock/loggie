# Loggie 项目记忆

## 持久化层（BadgerDB）

### 用途
持久化层记录的是**文件读取偏移量**（不是 Kafka topic offset）。每个被 tail 的日志文件对应一条 Registry 记录，唯一标识为 `(JobUid={inode}-{device}, SourceName, PipelineName)`。

每个文件存储的数据：
- `Offset`（字节偏移量）+ `LineNumber`（行号）— 重启后 Seek 继续读取
- `Filename` — 文件 rename 时更新
- `CollectTime`、`Version`

### 架构
- BadgerDB 是唯一持久化引擎（v2.0.0 起移除 SQLite，不再需要 CGO）
- `DbEngine` 接口定义在 `pkg/util/persistence/reg/engine.go`
- `DbHandler` 编排批量写入 + 内存索引，位于 `pkg/util/persistence/persistence.go`
- `registryIndex map[string]int`（compositeKey→Id）在 `DbHandler` 中，O(1) 查找替代旧 `FindAll()` 热路径
- 调用方：仅 `pkg/source/file/*`（文件源组件）

### 关键路径
- **写入热路径**（每 2s flush）：`write()` → `compressStats()` → 内存索引查 Id → `db.Update()`（批量）
- **注册偏移量**：`upsertOffsetByJobWatchId()` → 内存索引查 Id → `Update` 或 `Insert`
- **清理**（每小时）：`cleanData()` → `FindAll()` → 检查 `CollectTime` → `Delete` 过期记录
- **启动恢复**：`NewDbHandler()` → `FindAll()` → 重建内存索引

### 数据量
很小：几百到几千条记录（每个活跃文件一条），21 天不活跃后清理。

---

## 已完成的优化（v2.0.0）

### A. Binary 编码（替代 JSON）
- `pkg/util/persistence/reg/encode.go`：`encoding/binary` + 固定 schema
- 21 字节 header（1B version + 10B fixed + 5×2B length）+ length-prefixed strings
- 每条记录 ~100 bytes（JSON ~200 bytes）
- `Value()` 返回 binary，`DecodeRegistry()` 反序列化

### B. 长度前缀 Key（替代 `/` 分隔）
- `GenKey()` 使用 `[len:2][data]` 编码每个组件
- 避免 K8s 名称中的 `/` 造成 key 冲突

### C. 内存索引（替代 `FindAll()` 热路径）
- `registryIndex map[string]int` 在 `DbHandler.write()` 中做 O(1) 查找
- 启动时从 `FindAll()` 重建索引
- 所有 Insert/Update/Delete 操作同步更新索引

### D. 移除 SQLite + CGO
- 删除 `driver/sqlite.go` 和 `driver/sqlite_test.go`
- 删除 `Dockerfile.badger`（主 Dockerfile 已无 CGO）
- Makefile 移除 `CGO_ENABLED=1`，合并 `build-in-badger` 到 `build`
- 移除 `go-sqlite3` 依赖，`go mod tidy`
- 移除 `DriverSqlite` 常量、`_DRIVER_` 变量、`build tag` 互斥编译

### E. 库升级
- badger v3.2103.5 → v4.9.5（修复 compaction bug、L0 backpressure、value-log GC）
- ristretto v0.1.1 → v2.2.0（via badger v4）
- go-sqlite3 v1.14.48（已移除）

---

## Benchmark（Ryzen 5 3600, Go 1.26, badger v4.9.5）

### 单条操作
```
操作           v3 原始         v4 优化后       变化
Insert        11.9μs/2731B    ~11μs/2167B     -8%/-21%
Update        14.6μs/2718B    ~11μs/2188B     -25%/-19%
FindBy (10k)   6.7μs/1.9KB     5.9μs/1.8KB    -12%/-5%
GenKey          86ns/64B        41ns/48B       -52%/-25%
Value()        969ns/200B       69ns/96B       -93%/-52%
```

### FindAll（全量读）
```
数据量     v3 (修bug后)       v4             变化
100 条    121μs/87KB/1216allocs
1000 条   920μs/557KB/7621allocs
10000 条  12.4ms/7.3MB/73k     ~10ms/7.3MB/71k   -18%
```

### write() 热路径
```
旧：FindAll() + contain() = 5-8μs
新：map[] 索引查找 = 51ns（100x+ 提升）
```

### 10k 文件批量操作
```
操作             v3             v4.9.5         变化
Insert10k       19.4ms/11.6MB   ~22.8ms/11.5MB  +17%（v4 写入更保守）
Update10k       20.6ms/18.5MB   ~26.4ms/18.6MB  +28%（v4 写入更保守）
FindAll10k      12.4ms/7.3MB    ~10ms/7.3MB     -18%
FindBy (10k)     6.7μs           5.9μs          -12%
FlushWithIndex  23.8ms/13.2MB   ~22.7ms/13.1MB  -5%
```

### 对比 SQLite（v1.6.0 基线）
```
操作            SQLite          BadgerDB(v4)   差距
Insert          122μs           ~11μs          11x
Update          102μs           ~11μs          9x
FindAll(100)    534μs/60KB      121μs/87KB     4x（v4 反序列化更重）
FindAll(1000)   5.1ms/690KB     920μs/557KB    6x
```

### v3→v4 客观评价
- **明确变好**：单条操作速度（-10~25%）、FindAll 读取（-18%）、FindBy（-12%）、内存使用（-20%）
- **明确变差**：批量写入 Insert10k/Update10k（+17~28%），v4 ristretto/v2 缓冲策略更保守
- **净收益**：v4 的生产稳定性修复（compaction bug、L0 backpressure、GC 改进）值得接受写入退化

---

## 项目结构（持久化层）

```
pkg/util/persistence/
├── config.go            # DbConfig, SetDefaults() → ./data/badger
├── persistence.go       # DbHandler: 内存索引, write(), run() 循环
├── persistence_test.go  # 辅助函数测试
├── state.go             # State struct
└── reg/
    ├── engine.go        # DbEngine 接口
    ├── registry.go      # Registry: Key(), Value(), Merge(), GenKey()
    ├── encode.go        # Binary encode/decode (version 1)
    └── registry_test.go # Registry 单元测试
```

### 已删除的文件
- `pkg/util/persistence/driver/sqlite.go` — SQLite 驱动（v2.0.0 移除）
- `pkg/util/persistence/driver/sqlite_test.go` — SQLite 测试（v2.0.0 移除）
- `Dockerfile.badger` — 旧的无 CGO Dockerfile（v2.0.0 合并到主 Dockerfile）

### 仍在的文件
- `pkg/util/persistence/driver/badger.go` — BadgerDB 引擎
- `pkg/util/persistence/driver/badger_test.go` — BadgerDB 测试 + 10k 压测

---

## 构建

### 本地构建
```bash
go build -o loggie cmd/loggie/main.go                    # native
GOOS=linux GOARCH=arm64 go build -o loggie-linux-arm64 cmd/loggie/main.go
GOOS=darwin GOARCH=arm64 go build -o loggie-darwin-arm64 cmd/loggie/main.go
```

### CI/CD
- `.github/workflows/release.yaml`：推送 `v*` tag 时自动构建 4 平台二进制 + GitHub Release
- `.github/workflows/build-docker-image.yml`：推送 tag/分支时构建 Docker 镜像
- `.github/workflows/makefile.yml`：PR 时跑 lint + test
- `Makefile`：`make build` 无需 CGO，`make test`，`make fmt`

### 测试
```bash
go test ./pkg/util/persistence/... -v    # 持久化层测试
go test ./pkg/util/persistence/... -bench=. -benchmem  # benchmark
```

### 注意事项
- go.mod 要求 Go 1.26+（sonic v1.15.2 需要）
- GitHub Actions runner 需要 `setup-go@v5` + `go-version: '1.26'`
- 旧 `release.yml` 已删除（用 Go 1.18，无法解析 go 1.26）

---

## 发布历史

### v2.0.0（当前）
- 移除 SQLite/CGO，BadgerDB 唯一引擎
- Binary 编码 + 长度前缀 Key + 内存索引
- badger v3→v4.9.5
- 4 平台交叉编译（linux/darwin × amd64/arm64）
- CGO 不再需要，`go build` 直接可用

### v1.6.0
- SQLite 为默认引擎
- 需要 CGO 交叉编译器
