# 自动亮度与节能亮度缩放设计

## 1. 架构总览

亮度控制由三个模块协作完成：

- `org.deepin.dde.AmbientBrightness1` — 环境光传感器，计算推荐亮度
- `org.deepin.dde.Display1` — 内置屏亮度仲裁，执行亮度写入，应用节能缩放
- `org.deepin.dde.Power1` — 电源策略，发布节能状态

```text
AmbientBrightness1 ──RecommendedBrightness──▶ Display1 ──▶ 背光硬件
                                                    ▲
Power1 ──PowerSavingModeEnabled ────────────────────┤
      ──PowerSavingModeBrightnessDropPercent ────────┤
                                                    │
                                                    │ （监听变化，本地算 scale）
```

---

## 2. 自动亮度（AmbientBrightness1 + Display1）

### 2.1 职责分离

| 模块 | 职责 |
|---|---|
| `AmbientBrightness1` | 独占环境光传感器，持久化用户开关，完成 lux 滤波、滞回、防抖和推荐亮度计算 |
| `Display1` | 消费推荐值，串行执行自动和手动亮度事务，写入内置屏 |

### 2.2 AmbientBrightness1 D-Bus 契约

| 属性 | 类型 | 用途 |
|---|---|---|
| `Enabled` | `bool` | 用户持久化的自动亮度开关 |
| `State` | `string` | `Unavailable`、`WaitingForSample` 或 `Active` |
| `Supported` | `bool` | 当前是否已成功 Claim 环境光传感器 |
| `RecommendedBrightness` | `double` | 目标亮度，范围 `[0.0, 1.0]` |

### 2.3 Display1 兼容接口

| 接口 | 说明 |
|---|---|
| `AutoBrightnessSupported` | 镜像 `AmbientBrightness1.Supported` |
| `AutoBrightnessEnabled` | 镜像 `AmbientBrightness1.Enabled` |
| `SetAutoBrightnessEnabled(bool)` | 代理调用 `AmbientBrightness1.Enable(bool)` |

Display1 不再持久化独立的自动亮度开关。

### 2.4 应用条件

同时满足以下条件才应用推荐值：

- `Enabled = true`
- `State = "Active"`
- `Supported = true`
- 当前用户会话 active
- Display1 未处于 hold（休眠、合盖）
- 推荐值有限且位于 `[0.0, 1.0]`

### 2.5 亮度事务

同一内置屏只允许一个自动渐变 worker：

- 第一个自动目标从当前亮度渐变到目标；
- 自动过程中收到新推荐值时调用 `Update(target)`，从事务当前值重新计时；
- `Stop()` 等待正在执行的硬件写入和旧 worker 完全退出。

手动亮度是无渐变事务：

1. 关闭自动应用门控；
2. 停止当前自动事务；
3. 调用 `AmbientBrightness1.Enable(false)`；
4. 直接写入目标亮度；
5. 保存手动亮度配置。

---

## 3. 节能亮度缩放

### 3.1 方案

Power1 通过系统总线发布节能状态和降低百分比。Display1 监听这两个属性，本地计算缩放系数并应用到亮度写入。

**不新增 D-Bus 属性**，不增加配置字段。

### 3.2 Power1 属性

Display1 监听以下两个系统总线属性：

| 属性 | 类型 | 来源 |
|---|---|---|
| `PowerSavingModeEnabled` | `bool` | `org.deepin.dde.Power1` |
| `PowerSavingModeBrightnessDropPercent` | `uint32` | `org.deepin.dde.Power1`，范围 `[0, 100]` |

### 3.3 缩放系数计算

```go
func calcBrightnessScale(enabled bool, dropPercent uint32) float64 {
    if !enabled {
        return 1.0
    }
    return 1.0 - float64(dropPercent)/100.0
}
```

| 节能状态 | dropPercent | scale |
|---|---|---|
| 关闭 | — | 1.0 |
| 开启 | 20 | 0.8 |
| 开启 | 0 | 1.0 |

### 3.4 Scale 应用策略

亮度有"逻辑值"和"实际值"两个概念：

- **逻辑值**：配置中保存的原始亮度，不受节能影响
- **实际值**：写入硬件的亮度，前端 `Brightness` 属性显示的值

```go
func scaleBrightness(base, scale float64) float64 {
    if base <= 0.1 {
        return 0.1           // 最低亮度不缩放
    }
    v := base * scale
    return max(0.1, min(1.0, v))
}
```

### 3.5 各亮度写入路径

| 路径 | 写入值 | 缩放 | 保存配置 |
|---|---|---|---|
| `SetBrightness(V)` | 直接写 `V` | 否 | 否 |
| `SetAndSaveBrightness(V)` | 直接写 `V` | 否 | 是，存 `V` |
| `ChangeBrightness` | 基于实际值加减步长 | 否 | 是，存新值 |
| `RefreshBrightness` | `scaleBrightness(config.Brightness, scale)` | 是 | 否 |
| 配置应用（新显示器接入、模式切换） | `scaleBrightness(config.Brightness, scale)` | 是 | 否 |
| 自动亮度推荐值 | `scaleBrightness(recommended, scale)` | 是 | 渐变完成后保存 `recommended` |
| Scale 变化（节能开关/比例变化） | `scaleBrightness(config.Brightness, newScale)` | 是 | 否 |
| 色温 gamma 重设 | `monitor.Brightness`（实际值） | 否 | 否 |
| 熄屏半亮（screenBlack） | `oldBrightness * 0.5` 或 `0.02` | 否 | 否 |

### 3.6 自动亮度与缩放同时生效

自动亮度运行期间 scale 变化的行为：

```text
自动推荐值 R（逻辑值）
    ↓ × scale
缩放后目标 T（实际值）
    ↓
transition.Update(T)     ← 平滑渐变到新目标
```

渐变完成时保存的是 `R`（原始推荐值），不是 `T`（缩放后目标值）。

手动设置亮度时，自动亮度被禁用，后续 scale 变化不影响手动值。

### 3.7 低电量

低电量通过 `PowerSavingModeAutoWhenBatteryLow` 自动触发节能模式，走同样的缩放路径。Display1 不需要额外处理。

---

## 4. 配置与持久化

### 4.1 配置内容

`SysMonitorConfig.Brightness` 始终保存**逻辑值**（未缩放）。

`Manager.Brightness` 属性保存**实际值**（缩放后），供前端 D-Bus 消费者显示。

### 4.2 保存时序

| 触发 | 保存值 | 调用 |
|---|---|---|
| `SetAndSaveBrightness(V)` | `V`（用户输入） | `saveBrightnessInCfg` |
| `ChangeBrightness` | 新步进值 | `saveBrightnessInCfg` |
| 自动亮度渐变完成 | `recommendedBrightness` | `saveBrightnessInCfg` |

Scale 变化、`RefreshBrightness`、配置应用、色温重设 **不保存配置**。

---

## 5. 前向兼容

旧版本（无节能缩放）的配置格式不变，`SysMonitorConfig.Brightness` 语义始终是逻辑值。升级后：

- 首次启动时 `initBrightnessScale` 读取 Power1 属性
- 若节能已开启，立即通过 `applyBrightnessScale()` 降低亮度
- 若节能关闭，`scale = 1.0`，行为与旧版本完全一致

旧 session/power1 的 `PowerSavingModeBrightnessData`、`multiBrightnessWithPsm`、`saveBrightnessWhilePsm` 已全部移除，无需迁移。

---

## 6. 关键文件

| 文件 | 职责 |
|---|---|
| `display1/brightness_scale.go` | 缩放系数管理、监听、应用 |
| `display1/auto_brightness.go` | 自动亮度消费、渐变、保存 |
| `display1/brightness.go` | 底层亮度写入、配置保存 |
| `display1/brightness/brightness_transition.go` | 单 worker 亮度渐变 |
| `display1/recommendation_client.go` | AmbientBrightness1 D-Bus 客户端 |
