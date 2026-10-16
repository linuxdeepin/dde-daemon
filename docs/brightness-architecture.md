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

### 3.4 Scale 应用策略（存显示值、切换时换算）

采用**"存显示值、切换时换算一次"**模型：

- **显示值**：写入硬件、前端 `Brightness` 属性显示、并**保存到配置**的值
- 缩放**只在节能开关切换或降低比例变化的那一刻一次性换算**；唤醒、刷新、
  配置应用等恢复路径**直接使用配置里保存的显示值**，不再反复施加缩放

> 为什么放弃旧的"存逻辑值 × scale"模型：`scale = 0.9` 时逻辑值最大为 1.0，
> 显示值最大只能到 `1.0 × 0.9 = 0.9`，节能开启时**永远无法显示 100%**；
> 且 `unscale` 反算的基准被 `isValidBrightness`（≤ 1.0）钳制后往返丢失，
> 待机唤醒后 `RefreshBrightness` 重新乘 scale 会把用户设置的 100% 降到 90%。

节能开关/比例变化时的换算函数：

```go
// 先按旧系数还原逻辑亮度（上限 100%），再乘新系数，钳制到 [0.1, 1.0]
func rescaleBrightness(displayed, oldScale, newScale float64) float64 {
    if oldScale <= 0 {
        return displayed // 旧系数为 0 不可逆，保持原值
    }
    logical := min(displayed/oldScale, 1.0)
    return clamp(logical*newScale, minBrightness, 1.0)
}
```

对应需求：

- 开启节能（oldScale=1.0 → 1-X%）：显示 = 当前 × (1-X%)
- 关闭节能（oldScale=1-X% → 1.0）：显示 = 当前 / (1-X%)，上限 100%
- 换算后低于 10% 显示 10%（低于总亮度 10% 时显示 10%）
- 关闭节能以 10% 为基准提高 = 10% / (1-X%)
- 改降低比例时按"提高后的值"折算，逻辑值超 100% 按 100% 上限折算

自动亮度的推荐值仍是实时重算，用 `scaleBrightness(recommended, scale)` 连续缩放；
因其不作为显示基准往返读取，不存在丢失问题。

### 3.5 各亮度写入路径

| 路径 | 写入值 | 缩放 | 保存配置 |
|---|---|---|---|
| `SetBrightness(V)` | 直接写 `V` | 否 | 否 |
| `SetAndSaveBrightness(V)` | 直接写 `V` | 否 | 是，存显示值 `V` |
| `ChangeBrightness` | 基于显示值加减步长 | 否 | 是，存新显示值 |
| `RefreshBrightness` | `config.Brightness`（显示值） | 否 | 否 |
| 配置应用（新显示器接入、模式切换） | `config.Brightness`（显示值） | 否 | 否 |
| 自动亮度推荐值 | `scaleBrightness(recommended, scale)` | 是 | 渐变完成后存显示值 |
| Scale 变化（节能开关/比例变化） | `rescaleBrightness(显示值, oldScale, newScale)` | 换算 | 显示值变化时落盘 |
| 色温 gamma 重设 | `monitor.Brightness`（显示值） | 否 | 否 |
| 熄屏半亮（screenBlack） | `oldBrightness * 0.5` 或 `0.02` | 否 | 否 |

### 3.6 自动亮度与缩放同时生效

自动亮度运行期间 scale 变化的行为：

```text
自动推荐值 R（逻辑值）
    ↓ × scale
缩放后目标 T（显示值）
    ↓
transition.Update(T)     ← 平滑渐变到新目标
```

渐变完成时保存的是显示值 `T = scaleBrightness(R, scale)`，
这样自动亮度关闭后恢复路径可直接沿用，节能开关切换时也按显示值统一换算。

手动设置亮度时，自动亮度被禁用，后续 scale 变化按显示值换算。

### 3.7 低电量

低电量通过 `PowerSavingModeAutoWhenBatteryLow` 自动触发节能模式，走同样的换算路径。Display1 不需要额外处理。

---

## 4. 配置与持久化

### 4.1 配置内容

`SysMonitorConfig.Brightness` 保存**显示值**（屏幕实际显示、含节能缩放后的值）。

`Manager.Brightness` 属性同样是**显示值**，供前端 D-Bus 消费者显示，两者一致。

缩放系数（`m.brightnessScale`）**不持久化**：启动时从 Power1 读取当前节能状态，
仅记录系数供自动亮度使用，不换算已保存的显示值。若节能状态相对上次关机发生变化，
亮度会在下一次节能开关/比例变化时重新同步。

### 4.2 保存时序

| 触发 | 保存值 | 调用 |
|---|---|---|
| `SetAndSaveBrightness(V)` | `V`（用户输入的显示值） | `saveBrightnessInCfg` |
| `ChangeBrightness` | 新步进显示值 | `saveBrightnessInCfg` |
| 自动亮度渐变完成 | `scaleBrightness(recommended, scale)`（显示值） | `saveBrightnessInCfg` |
| Scale 变化（节能开关/比例变化）且显示值改变 | `rescaleBrightness(...)` | `saveBrightnessInCfg` |

`RefreshBrightness`、配置应用、色温重设 **不保存配置**。

---

## 5. 前向兼容

配置格式不变，`SysMonitorConfig.Brightness` 为显示值。升级后：

- 首次启动时 `initBrightnessScale` 读取 Power1 属性，仅记录缩放系数，不换算已存显示值
- 节能关闭（`scale = 1.0`）时显示值即逻辑值，行为与旧版本完全一致
- 若从旧"存逻辑值"版本升级且升级时节能恰好开启：配置里的旧逻辑值会被当作显示值
  直接恢复（略偏亮），在下一次节能开关/比例变化时按新模型重新换算归位

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
