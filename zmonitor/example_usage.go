package zmonitor

// 这个文件展示如何使用监控接口，不会被编译

/*
## 使用示例

### 1. 基础用法：直接调用组件的 GetMonitorData()

```go
// 监控 Actor
actor := actor.NewActor(config)
data := actor.GetMonitorData()
fmt.Printf("Actor %s: %+v\n", data.Name, data.Metrics)

// 监控 Session
session := session.NewSession(...)
data = session.GetMonitorData()
fmt.Printf("Session %s: %+v\n", data.Name, data.Metrics)

// 监控 GateServer
gate := gateserver.NewServer(...)
data = gate.GetMonitorData()
fmt.Printf("Gate %s: %+v\n", data.Name, data.Metrics)

// 监控系统
systemData := zmonitor.CollectSystemMonitor()
fmt.Printf("System: %+v\n", systemData.MemStats)
```

### 2. 使用 Monitor Manager（集中管理）

```go
// 创建监控管理器
mgr := zmonitor.NewManager()

// 注册组件
mgr.Register("actor_10001", actor1)
mgr.Register("actor_10002", actor2)
mgr.Register("session_123", session1)
mgr.Register("gate_1", gate)

// 获取所有监控数据
allData := mgr.GetAll()
for _, data := range allData {
    fmt.Printf("%s: %+v\n", data.Name, data.Metrics)
}

// 按类型筛选
actors := mgr.GetByType("actor")
sessions := mgr.GetByType("session")
gates := mgr.GetByType("gate")
```

### 3. HTTP API 集成（推荐）

```go
import (
    "encoding/json"
    "net/http"
    "github.com/aiyang-zh/zhenyi/zmonitor"
)

// 创建 HTTP 服务器
func StartMonitorServer(mgr *zmonitor.Manager, port int) {
    // API: 获取所有监控数据
    http.HandleFunc("/api/monitor/all", func(w http.ResponseWriter, r *http.Request) {
        data := mgr.GetAll()
        json.NewEncoder(w).Encode(data)
    })

    // API: 按类型获取
    http.HandleFunc("/api/monitor/actors", func(w http.ResponseWriter, r *http.Request) {
        data := mgr.GetByType("actor")
        json.NewEncoder(w).Encode(data)
    })

    http.HandleFunc("/api/monitor/sessions", func(w http.ResponseWriter, r *http.Request) {
        data := mgr.GetByType("session")
        json.NewEncoder(w).Encode(data)
    })

    http.HandleFunc("/api/monitor/gates", func(w http.ResponseWriter, r *http.Request) {
        data := mgr.GetByType("gate")
        json.NewEncoder(w).Encode(data)
    })

    // API: 获取系统监控
    http.HandleFunc("/api/monitor/system", func(w http.ResponseWriter, r *http.Request) {
        data := zmonitor.CollectSystemMonitor()
        json.NewEncoder(w).Encode(data)
    })

    // API: 获取单个组件
    http.HandleFunc("/api/monitor/component", func(w http.ResponseWriter, r *http.Request) {
        id := r.URL.Query().Get("id")
        data, ok := mgr.Get(id)
        if !ok {
            http.Error(w, "Component not found", 404)
            return
        }
        json.NewEncoder(w).Encode(data)
    })

    http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

// 在 main.go 中使用
func main() {
    mgr := zmonitor.NewManager()

    // 创建组件
    actor1 := actor.NewActor(...)
    gate := gateserver.NewServer(...)

    // 注册到监控
    mgr.Register("actor_10001", actor1)
    mgr.Register("gate_1", gate)

    // 启动监控服务器
    go StartMonitorServer(mgr, 8080)

    // 启动业务服务
    actor1.Run(...)
    gate.Init(...)
}
```

### 4. Prometheus 集成（可选）

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus 导出器
type PrometheusExporter struct {
    mgr *zmonitor.Manager

    // Actor 指标
    actorProcessed *prometheus.GaugeVec
    actorQPS       *prometheus.GaugeVec
    actorLatency   *prometheus.GaugeVec

    // Session 指标
    sessionCount   *prometheus.GaugeVec
    sessionBytes   *prometheus.GaugeVec

    // Gate 指标
    gateQPS        *prometheus.GaugeVec
    gateOnline     *prometheus.GaugeVec
}

func NewPrometheusExporter(mgr *zmonitor.Manager) *PrometheusExporter {
    e := &PrometheusExporter{
        mgr: mgr,
        actorProcessed: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{Name: "zhenyi_actor_processed_total"},
            []string{"actor_id", "actor_type"},
        ),
        // ... 定义其他指标
    }

    // 注册指标
    prometheus.MustRegister(e.actorProcessed)
    // ...

    return e
}

func (e *PrometheusExporter) Collect() {
    // 定期采集数据
    for _, data := range e.mgr.GetAll() {
        switch data.Type {
        case "actor":
            e.actorProcessed.WithLabelValues(
                data.ID,
                data.Tags["topic"],
            ).Set(float64(data.Metrics["processedMsg"].(int64)))
            // ...
        }
    }
}

// 启动 Prometheus 服务
func StartPrometheus(mgr *zmonitor.Manager) {
    exporter := NewPrometheusExporter(mgr)

    // 每 5 秒采集一次
    go func() {
        ticker := time.NewTicker(5 * time.Second)
        for range ticker.C {
            exporter.Collect()
        }
    }()

    // 启动 HTTP 服务
    http.Handle("/metrics", promhttp.Handler())
    http.ListenAndServe(":9090", nil)
}
```

### 5. 自定义监控面板

使用者可以基于 HTTP API 创建自己的监控面板（Web/Desktop/Mobile），
推荐使用：
- Grafana (Prometheus 数据源)
- ECharts / Chart.js (自定义 Web 面板)
- Prometheus + AlertManager (告警)

### 6. 动态注册/注销（重要）

```go
// SessionManager 中管理 Session 监控
type SessionManager struct {
    monitor *zmonitor.Manager
}

func (mgr *SessionManager) AddSession(session *Session) {
    // 注册到监控
    mgr.monitor.Register(
        fmt.Sprintf("session_%d", session.GetSessionId()),
        session,
    )
}

func (mgr *SessionManager) RemoveSession(sessionId int64) {
    // 注销监控
    mgr.monitor.Unregister(fmt.Sprintf("session_%d", sessionId))
}
```

### 7. 告警示例

```go
// 定期检查并告警
func MonitorAlert(mgr *zmonitor.Manager) {
    ticker := time.NewTicker(10 * time.Second)
    for range ticker.C {
        for _, data := range mgr.GetAll() {
            switch data.Type {
            case "actor":
                // 检查消息队列积压
                mailCount := data.Metrics["mailCount"].(int64)
                if mailCount > 10000 {
                    logger.Error("Actor mailbox overflow!",
                        zap.String("actor", data.Name),
                        zap.Int64("mailCount", mailCount))
                }

                // 检查慢消息
                slowCount := data.Metrics["slowCount"].(int64)
                if slowCount > 100 {
                    logger.Warn("Too many slow messages",
                        zap.String("actor", data.Name),
                        zap.Int64("slowCount", slowCount))
                }

            case "session":
                // 检查长时间无活动的 Session
                lastActiveMs := data.Metrics["lastActiveMs"].(int64)
                idleTime := time.Now().UnixMilli() - lastActiveMs
                if idleTime > 300000 { // 5分钟
                    logger.Info("Idle session detected",
                        zap.String("session", data.Name),
                        zap.Int64("idleMs", idleTime))
                }

            case "gate":
                // 检查内存使用
                memAllocMB := data.Metrics["memAllocMB"].(float64)
                if memAllocMB > 2048 { // 2GB
                    logger.Warn("High memory usage",
                        zap.String("gate", data.Name),
                        zap.Float64("memMB", memAllocMB))
                }
            }
        }
    }
}
```

## 最佳实践

1. **不要在热点路径中调用 GetMonitorData()**
   - 监控采集应该由独立的 goroutine 定期执行（5-10秒一次）
   - 避免在消息处理循环中调用

2. **使用 Monitor Manager 集中管理**
   - 便于统一导出监控数据
   - 支持动态注册/注销

3. **根据需求选择导出方式**
   - 轻量级：HTTP + JSON
   - 生产级：Prometheus + Grafana
   - 定制化：自定义面板

4. **监控数据与告警分离**
   - 监控负责采集和展示
   - 告警逻辑单独实现

5. **注意内存占用**
   - Session 数量大时（10万+），考虑采样或分批采集
   - 不要在内存中保存所有历史数据
*/
