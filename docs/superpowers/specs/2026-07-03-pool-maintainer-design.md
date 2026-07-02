# Sub2API 号池维护工具设计

## 背景

当前号池由少量自建号和 4-5 个上游 Sub2API 中转站账号组成。上游站点本身也按分组提供不同模型调用倍率，本地会为每个上游站点的每个上游分组创建一个 API Key，并在本地 Sub2API 中创建对应账号。

上游模型调用倍率只能登录上游 Sub2API 网站后台查看，不能假设有稳定 API。由于上游倍率可能变化，且可能高于本地对用户销售的模型调用倍率，需要提高维护效率，减少手工检查、分组调整、优先级调整和停用高成本账号的工作量。

## 目标

- 第一版先做 Windows 本地外部维护工具，不直接集成到 Sub2API 后台。
- 尽量自动登录并读取上游 Sub2API 页面中的分组倍率。
- 用配置文件维护上游站点、本地账号匹配规则、账号允许进入的本地销售分组。
- 生成 HTML 审核报告和 JSON 应用计划，供人工确认。
- 人工确认后通过本地 Sub2API Admin API 应用变更，不直接修改数据库。
- 覆盖账号名倍率后缀、`rate_multiplier`、所属分组、`priority`、`schedulable` 五类变更。
- 采集失败时保持生产账号现状，只在报告中标红提示需要重新登录或人工确认。

## 非目标

- 第一版不做后台页面。
- 第一版不做无人值守自动应用。
- 第一版不保存上游站点账号密码。
- 第一版不直接连接生产 PostgreSQL 修改账号。
- 第一版不把便宜账号自动扩散到所有高倍率销售分组，必须受配置白名单限制。

## 总体架构

工具由四个模块组成：

1. 配置文件模块
   负责读取本地 Sub2API 地址、Admin Token 环境变量名、销售分组、上游站点、账号匹配规则、自建号识别规则和分组白名单。

2. 采集器
   在 Windows 本地通过浏览器 profile 打开上游 Sub2API 后台页面。首次或登录失效时由管理员手工登录，后续复用 Cookie/session。采集器从页面 DOM、表格和文本中提取“上游分组名 -> 模型调用倍率”。

3. 规则引擎
   将采集到的上游倍率、本地账号现状和配置白名单合并，计算建议变更。规则引擎不直接修改生产数据。

4. 审核与应用器
   `collect` 生成 HTML 报告和 JSON 计划。`apply` 读取 JSON 计划，通过 Admin API 应用已确认的变更，并生成应用结果。

## 配置文件

第一版配置文件建议放在外部工具目录，例如：

```text
tools/pool-maintainer/pool-maintainer.yaml
```

示例结构：

```yaml
local_sub2api:
  base_url: "https://api.zhouz.online"
  admin_token_env: "SUB2API_ADMIN_TOKEN"

policy:
  safety_margin: 0.02
  sales_groups:
    - name: "0.12"
      group_id: 101
      rate: 0.12
    - name: "0.18"
      group_id: 102
      rate: 0.18
    - name: "0.25"
      group_id: 103
      rate: 0.25
  priority:
    self_built: 1
    upstream_start: 5
    upstream_step: 5

upstreams:
  - id: "mdkj"
    base_url: "https://api.mdkj.lol"
    pricing_page_url: "https://api.mdkj.lol/"
    browser_profile: "mdkj"
    group_name_aliases:
      pro: ["pro", "Pro", "PRO"]

accounts:
  - match_name: "https://api.mdkj.lol-pro-*"
    upstream_id: "mdkj"
    upstream_group: "pro"
    allowed_sales_groups: ["0.18", "0.25"]

self_built_accounts:
  - match_name: "self-*"
    allowed_sales_groups: ["0.12", "0.18", "0.25"]
```

配置规则：

- Admin Token 只从环境变量读取，不写入配置、报告或计划。
- `pricing_page_url` 可以先手工配置；如果未配置具体页面，工具可打开站点首页并允许管理员手动导航到倍率页面。
- `accounts.allowed_sales_groups` 是账号分组白名单，自动化只在白名单范围内建议调整。
- 第一版优先通过账号名匹配，例如 `https://api.mdkj.lol-pro-0.2`。后续可升级为稳定账号 ID。
- 自建号独立配置，使用 `policy.self_built_rate` 作为成本，仍按白名单和成本档位参与混合调度。

## 采集流程

命令：

```powershell
pool-maintainer open-browser --config tools\pool-maintainer\pool-maintainer.yaml --profiles-dir runs\pool-maintainer-profiles
pool-maintainer collect --config tools\pool-maintainer\pool-maintainer.yaml --html-dir runs\pool-maintainer-snapshots --out runs\2026-07-03-1530
```

流程：

1. `open-browser` 为每个上游站点启动或复用本地浏览器 profile，并打开 `pricing_page_url`。
2. 管理员在浏览器中登录或导航到倍率页面，然后把页面另存为 `<upstream_id>.html`，放入 `--html-dir` 指向的快照目录。
3. `collect` 只读取 `--html-dir` 中的 HTML 快照和本地 Admin API 账号状态，不控制浏览器、不等待页面交互。
4. 快照解析时：
   - 如果 HTML 中出现未登录关键词，或找不到倍率信息，则标记为 `need_login`。
   - 优先读取文本化后的表格和卡片内容。
   - 其次扫描页面文本中的分组名和倍率。
5. 保存报告和应用计划。

每次运行输出目录示例：

```text
runs/2026-07-03-1530/
  apply-plan.json
  report.html
  apply-result.json
```

采集失败、登录失效、倍率解析冲突时，对应账号不进入生产变更计划，只在报告中标红。

## 规则计算

### 账号匹配

本地账号名第一版采用约定格式：

```text
https://api.mdkj.lol-pro-0.2
```

含义：

- 上游站点：`https://api.mdkj.lol`
- 上游分组：`pro`
- 旧命名倍率：`0.2`

旧命名倍率只作为当前状态展示和重命名对比，不作为真实成本依据。真实成本使用本次采集到的上游倍率。

### 准入线

本地销售分组为 `0.12`、`0.18`、`0.25`。安全边际固定为 `0.02`。

准入线：

- `0.12` 分组：上游倍率 `<= 0.10`
- `0.18` 分组：上游倍率 `<= 0.16`
- `0.25` 分组：上游倍率 `<= 0.23`

如果上游倍率高于 `0.23`，在当前销售分组体系下不应继续参与调度，规则引擎建议 `schedulable=false`，并从销售分组中移除。

### 白名单约束

账号即使满足更高倍率分组的准入线，也只能进入配置文件中 `allowed_sales_groups` 指定的分组。自动化不会把低成本账号自动加入所有可盈利分组。

### 优先级计算

优先级按每个本地销售分组单独计算：

- 自建号使用 `policy.self_built_rate` 作为成本，和上游账号一起按成本从低到高排序。
- 最便宜成本档位为 `priority=5`。
- 后续每个不同倍率档位增加 `5`，即 `10`、`15`、`20`。
- 同成本账号使用同一个 `priority`，由现有调度器的负载率、LRU 和粘性会话机制分散。
- `policy.priority.self_built` 第一版仅保留为兼容字段，不作为绝对优先级覆盖。

### 同步字段

规则引擎为每个账号生成以下建议：

- 账号名末尾倍率更新为采集到的新倍率。
- `rate_multiplier` 更新为采集到的新倍率；自建号使用固定最低成本值。
- 所属分组更新为白名单内且满足准入线的目标分组集合。
- `priority` 更新为对应销售分组内的成本排序结果。
- 没有任何可进入分组时，建议 `schedulable=false`。

## 审核报告和应用计划

`collect` 输出两类核心文件：

1. `report.html`
   面向管理员审核，包含：
   - 本次采集总览。
   - 成功站点、失败站点、需要重新登录站点。
   - 上游倍率变化：旧命名倍率与新采集倍率。
   - 分组变化：当前分组到建议分组。
   - 优先级变化：当前 priority 到建议 priority。
   - 调度状态变化：是否建议 `schedulable=false`。
   - 风险标记：采集失败、倍率超线、账号无法匹配、白名单缺失、计划冲突。

2. `apply-plan.json`
   面向程序执行，包含：
   - 每个账号的账号 ID、当前值、目标值。
   - 变更原因。
   - 数据来源和采集时间。
   - 采集站点和上游分组。
   - 应用前漂移检查所需的当前状态摘要。

应用命令：

```powershell
pool-maintainer apply --plan runs/2026-07-03-1530/apply-plan.json
```

应用策略：

- 默认不自动应用，必须管理员显式执行 `apply`。
- `apply` 通过本地 Sub2API Admin API 修改账号。
- `apply` 前重新从 Admin API 拉取账号当前状态。
- 如果账号已被手工修改导致计划漂移，则跳过该账号并在结果中标记冲突。
- 应用后再次读取账号列表，验证账号名、`rate_multiplier`、分组、`priority`、`schedulable` 是否符合计划。
- 生成 `apply-result.json` 记录成功、失败和冲突账号。

## 失败处理

- 上游页面采集失败：保持账号现状，只在报告中标红。
- 登录失效：暂停等待人工登录，仍失败则保持账号现状。
- 页面倍率识别冲突：不生成该上游分组对应账号的生产变更。
- Admin API 应用失败：记录失败账号，继续处理其它账号，但不会重复盲目重试。
- Token 缺失：终止 apply，并提示设置 Admin Token 环境变量。

## 测试和验收

### 配置解析测试

- 能读取 4-5 个上游站点配置。
- 能读取账号匹配规则、自建号规则和白名单。
- 未知销售分组、重复上游 ID、缺失 `admin_token_env` 时给出明确错误。

### 规则引擎测试

- `0.12`、`0.18`、`0.25` 的安全边际计算正确。
- 白名单限制优先于成本低价扩散。
- 自建号 priority 固定为 `1`。
- 每个分组内上游账号按倍率从 `5` 起步，每个不同倍率档位 `+5`。
- 超过最高准入线时建议 `schedulable=false`。
- 旧账号名倍率不影响真实成本判断。

### 采集器测试

- 使用保存的 HTML 样本提取“分组名 -> 倍率”。
- 登录失效页面只生成 `need_login` 或失败标记，不生成账号变更。
- 站点级选择器覆盖能只影响对应上游站点。

### Admin API 应用测试

- dry-run 不修改账号。
- apply 前能检测账号状态漂移。
- apply 后能重新读取并验证账号状态。
- API 失败时结果文件能定位失败账号和失败原因。

### 第一版完成标准

- Windows 本地能运行 `collect` 并生成 HTML 报告和 JSON 计划。
- 至少用 1 个上游站点跑通登录后的倍率采集。
- 能对测试账号生成正确变更计划。
- `dry-run` 能展示将调用的 Admin API。
- 管理员确认后，`apply` 能修改测试账号并验证结果。

## 后续集成方向

当外部工具规则稳定后，可将配置、采集记录、审核报告和应用动作迁移到 Sub2API 后台页面。迁移时保留 JSON 计划结构作为后台任务和审计日志的数据模型，减少重做规则引擎的成本。
