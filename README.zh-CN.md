<div align="center">

# agentattest

**面向 AI 编程代理运行记录的轻量级可验证来源（provenance）层。**

**English** &nbsp;|&nbsp; [简体中文](./README.zh-CN.md)

[![status](https://img.shields.io/badge/status-v0%20%2B%20v1-1f6feb)]()
[![predicate](https://img.shields.io/badge/predicate-v0%20%2B%20v1-blue)]()
[![statement](https://img.shields.io/badge/container-in--toto%20Statement%20v1-1f6feb)]()
[![go](https://img.shields.io/badge/go-1.23-00ADD8)]()
[![schema](https://img.shields.io/badge/schema-JSON%20Schema%20%2B%20CUE-8b5cf6)]()
[![policy](https://img.shields.io/badge/policy-OPA%2FRego-7d3aed)]()
[![cache](https://img.shields.io/badge/cache-SQLite-003B57)]()

</div>

---

## 一句话说清

`agentattest` 把一次 AI 编程代理的运行记录**确定性绑定**到一个具体的 git patch、tree、PR 及审批状态上，输出一个 **in-toto Statement v1**，其 `predicateType` 严格等于 `https://agentattest.dev/predicate/v0`。

**v1**（`https://agentattest.dev/predicate/v1`）在此基础上额外绑定 2026 年已固化成型的前沿 harness 层面——`AGENTS.md` 运行契约、带 schema 摘要的 MCP 工具/服务器身份、harness 原生采集、平台代理身份（如 GitHub Copilot coding agent）以及多代理委派。**v0 保持原样、稳定不变。** 详见 [`docs/FRONTIER_HARNESS_2026.md`](docs/FRONTIER_HARNESS_2026.md)。

本项目**只负责**：自定义 predicate、subject 绑定规则、验证器输入与失败码、默认 Rego 策略，以及 CLI 与 Action 的使用体验。

**密码学、透明日志、运行 trace、SBOM、VEX 一律委托给现有生态**——DSSE、Sigstore/cosign、Fulcio/Rekor、GitHub Artifact Attestations、OpenTelemetry/OpenInference、SPDX、CycloneDX、OpenVEX。**绝不重复造轮子。**

---

## 目录

- [为什么](#为什么)
- [不做的事](#不做的事)
- [架构](#架构)
- [快速开始](#快速开始)
- [命令行](#命令行)
- [数据模型](#数据模型)
- [验证流水线](#验证流水线)
- [验证等级](#验证等级)
- [失败码](#失败码)
- [隐私默认](#隐私默认)
- [项目结构](#项目结构)
- [已有的轮子](#已有的轮子)
- [路线图](#路线图)
- [硬不变量](#硬不变量)
- [许可证](#许可证)
- [贡献](#贡献)

---

## 为什么

AI 编程代理已经在大量生产真实 PR，但生态目前仅有**指纹识别**这一类事后取证手段——从 diff 风格猜测是谁写的。仓库治理真正需要的是**第一方可验证声明**：把一次运行与一份 diff 强绑定；以及一个**确定性策略门**：在 subject、仓库、base commit、身份、隐私、等级等任何一项违规时 fail-closed。

`agentattest` 就是这个门，**而且仅仅是**这个门。

| 问题 | `agentattest` 怎么做 | `agentattest` **不**做什么 |
|---|---|---|
| "这个 PR 是代理写的吗？" | 输出签名声明，把运行绑定到 patch / tree / PR / 产物 | 靠代码风格启发式识别代理 |
| "确实是同一个 workflow 构建的？" | 用经过验证的签名者 / builder / workflow 交叉核对 predicate | 重新实现 Sigstore 或 cosign |
| "原始 prompt 内容泄露了吗？" | 通过 schema + 策略拒绝 `public` 原始证据，拒绝含凭据的 / `data:` URI | 扫描引用工件的字节内容查找 secret |
| "这条 attestation 被重放了吗？" | 把 repo / base / PR / runId / 新鲜度作为确定性验证上下文进行绑定 | 自建透明日志 |

---

## 不做的事

`agentattest` **不会**发明或替代以下任何一项——详见 [`docs/NON_GOALS.md`](docs/NON_GOALS.md)。

- 新的密码学、密钥格式、PKI、时间戳或透明日志。
- 新的 attestation 容器——稳定使用 **in-toto Statement v1**。
- 新的 trace 协议——只通过 URI + 摘要引用 **OpenTelemetry / OpenInference**。
- 新的 SBOM / VEX 格式——以 **SPDX / CycloneDX / OpenVEX** 作为侧车链接。
- 可观测平台、代码质量预言机、密钥管理系统、VSCode 插件（均在 v0 范围外）。
- 将 `agent.name` / `model.name` 当作信任根——它们只是自报 metadata。

**信任来自经过验证的签名者 / builder / workflow 身份 + 重新计算的 subject digest**，绝不来自 predicate 中任何自报字段。

---

## 架构

共 8 层；依赖方向由编排层向内收敛到纯领域层，再经 adapter 向外。禁止的 import 方向详见 [`ARCHITECTURE.md`](ARCHITECTURE.md)。

```
┌──────────────────────── 集成层 ──────────────────────────────┐
│                  cmd/agentattest · action/                   │
└────────────────────────────┬─────────────────────────────────┘
                             │
┌────────────────────────────▼─────────────────────────────────┐
│                    internal/app（编排）                       │
└──────┬───────────────────────────────────────────────┬───────┘
       │                                               │
   ┌───▼─────┐  ┌────────┐  ┌───────────┐  ┌────────┐  │
   │ gitbind │  │ predi- │  │ statement │  │ verify │  │
   │（采集+  │  │ cate   │  │ (in-toto) │  │（5 阶段）│
   │  绑定） │  │ (v0)   │  └─────┬─────┘  └───┬────┘  │
   └───┬─────┘  └────┬───┘        │            │       │
       │            │             │            │       │
       │            │      ┌──────▼──────┐  ┌──▼─────┐ │
       │            │      │  signing    │  │ policy │ │
       │            │      │（DSSE/cosign│  │（Rego）│ │
       │            │      │  /Sigstore）│  └────────┘ │
       │            │      └─────────────┘             │
       │            │                                  │
   ┌───▼────────────▼──────────────────────────────────▼─────┐
   │                  internal/cache（SQLite）                │
   │     refs · digests · timestamps——绝不参与通过/失败判定     │
   └─────────────────────────────────────────────────────────┘
```

**硬规则**

- Predicate 包不得 import Sigstore / Rekor / SQLite / OPA。
- 仅 `internal/signing` 可 import DSSE / cosign / Fulcio / Rekor。
- 缓存代码**永远**不能决定验证成功与否。
- GitHub Action 只是 CLI 的薄壳，**禁止**重新实现验证器。

---

## 快速开始

### 前置条件

- Go ≥ 1.23
- `git` 在 `PATH` 中
- （可选）`cue` 与 `opa`，用于 Go 测试以外的手工校验

### 构建与测试

```bash
go build ./cmd/agentattest
go test ./...
```

### 在本仓库端到端跑一次

```bash
# 1. 初始化本地缓存与 .agentattest 配置
#    （不存 secret，也不存通过/失败判定结果）
./agentattest init --dir .

# 2. 计算确定性 git 绑定
#    仓库 URL · base · head · patch 摘要 · 变更文件
./agentattest digest --repo .

# 3. 输出 in-toto Statement v1
#    predicateType = https://agentattest.dev/predicate/v0
./agentattest predicate create --repo . --out statement.json

# 4. 用验证器上下文 JSON 进行验证
#    （subjects、repoUrl、baseCommit、requiredLevel ……）
./agentattest verify predicate \
    --statement statement.json \
    --context  tests/golden/valid-minimal/context.json
```

验证器在出现任何失败码时**非零**退出，并输出稳定 JSON，格式为 `{ valid, level, failureCodes[] }`。

---

## 命令行

| 命令 | 用途 |
|---|---|
| `agentattest init [--dir DIR]` | 创建 `.agentattest/`，内含 SQLite 缓存与 JSON 配置。缓存**只**存 refs / digests / timestamps，**绝不**存通过/失败判定。 |
| `agentattest digest [--repo DIR]` | 计算仓库 URL、分支、base/head commit、patch SHA-256、变更文件 SHA-256。跨平台确定。 |
| `agentattest predicate create [--repo DIR] [--out PATH] [--repo-url ...] [--base-commit ...] [--agent-name ...] [--agent-version ...]` | 构建 v0 predicate + in-toto Statement v1，`subject[] = { patch.diff: sha256 }`。默认值：evidence-grade、本地执行、不存原始证据。 |
| `agentattest verify predicate --statement PATH --context PATH` | 跑 5 阶段流水线，返回稳定 JSON `{ valid, level, failureCodes[] }`。 |

CLI 仅做参数路由，业务逻辑全部在 `internal/app`；`cmd/agentattest/main.go` 只有 6 行入口。

---

## 数据模型

签名声明本体是 **in-toto Statement v1**。自定义 JSON Schema **仅**作用于内层 `predicate` 对象——外层容器保持与 cosign、GitHub attestation、`slsa-verifier` 的完全互操作。

```json
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [
    { "name": "patch.diff", "digest": { "sha256": "aaaa…aaaa" } },
    { "name": "repo-tree",  "digest": { "sha256": "bbbb…bbbb" } }
  ],
  "predicateType": "https://agentattest.dev/predicate/v0",
  "predicate": {
    "predicateVersion": "v0",
    "runId": "run_01HTYJ7B4A3H8M0S8P7M2N9Q",
    "verificationLevel": "policy-grade",
    "repo": {
      "url": "https://github.com/example/agentattest",
      "baseCommit": "0123…4567",
      "branch": "feature/agent-binding",
      "pullRequest": { "number": 123 }
    },
    "agent": {
      "name": "example-agent",
      "version": "1.0.0",
      "invocationKind": "ci-step",
      "declaredIdentity": "sigstore:github:example-agent"
    },
    "environment": {
      "executionType": "github-actions",
      "builder": {
        "id": "https://github.com/.../workflows/agentattest.yml@refs/heads/main",
        "type": "github-actions-workflow",
        "workflowRef": "...@refs/heads/main",
        "runnerClass": "github-hosted"
      }
    },
    "humanApproval": { "state": "approved-after-generation" },
    "privacy": {
      "redactionPolicy": "default-minimal-v0",
      "rawPrompt":      { "stored": false, "visibility": "none" },
      "rawToolOutputs": { "stored": false, "visibility": "none" },
      "publicTransparencyLog": { "allowed": true, "includesRawContent": false }
    },
    "timestamps": { "startedAt": "2026-04-27T09:12:31Z", "finishedAt": "2026-04-27T09:16:42Z" },
    "evidence": [
      {
        "type": "tool-summary",
        "uri": "artifact://...",
        "digest": { "sha256": "..." },
        "storage": "artifact-store",
        "visibility": "team"
      }
    ]
  }
}
```

### 必填字段

`predicateVersion` · `runId` · `verificationLevel` · `repo` · `agent` · `environment` · `humanApproval` · `privacy` · `timestamps` · `evidence`

### 可选字段

`model` · `runtime`（OTel / OpenInference trace ID） · `changes` · `sidecars`（SPDX / CycloneDX / OpenVEX） · `materials` · `extensions`（URI 键控、**closed**、必须 digest-addressed）

**v1 新增可选字段** · `capture`（采集方式——`harness-native` / `wrapper` / `ci-step` / `manual`） · `agentConfig`（约束本次运行的 `AGENTS.md` / 规则摘要） · `mcpServers` + `tools`（MCP 服务器身份 + 每工具 schema 摘要） · `delegation`（子代理 / 多代理链）

完整 schema 拆分为三个文件，**必须同步演进**：

| 文件 | 职责 |
|---|---|
| [`schemas/agent-provenance-v0.schema.json`](schemas/agent-provenance-v0.schema.json) | 结构、`additionalProperties: false`、类型、URI 模式、GitHub builder 字符串形态 |
| [`schemas/agent-provenance-v0.cue`](schemas/agent-provenance-v0.cue) | 跨字段语义——时间序、`runtime ↔ trace evidence` 绑定、原始可见性耦合 |
| [`schemas/agent-provenance-v1.schema.json`](schemas/agent-provenance-v1.schema.json) | v1 结构——新增 `agentConfig` / `mcpServers` / `tools` / `delegation` / `capture` / `platform-agent` builder |
| [`schemas/agent-provenance-v1.cue`](schemas/agent-provenance-v1.cue) | v1 跨字段语义——`harness-native ⇒ 必须绑定 trace`，并含全部 v0 规则 |
| [`policies/default.rego`](policies/default.rego) | 运行时上下文——仓库 / base 匹配、subject 等集、身份验证、等级升级、重放、新鲜度，以及 v1 的 agentConfig / MCP / delegation 门 |

---

## 验证流水线

5 个有序阶段；每条不变量**只归属一个阶段**，后续阶段不得重新分类早期阶段的失败。

| 阶段 | 负责模块 | 检查内容 |
|---|---|---|
| 01 | Go 预 schema 闸门（`internal/verify/phase01`） | `_type`、`predicateType`、`predicate.predicateVersion` 路由（**v0/v1**，含类型↔版本一致性）——*在* JSON Schema 之前 |
| 02 | JSON Schema 2020-12（`santhosh-tekuri/jsonschema`） | 必填字段、closed object、enum、URI 安全、GitHub builder 形态 |
| 03 | CUE（`cuelang.org/go`） | 跨字段语义：时间序、`runtime ⇒ trace evidence` |
| 04 | OPA Rego（`open-policy-agent/opa`） | 必需等级、subject 等集、repo / base 匹配、已验证签名者 / builder / workflow / issuer、等级升级、公链隐私、witness / 审批、重放、新鲜度 |
| 05 | Go 后置失败码排序（`OrderFailureCodes`） | 去重 + 面向用户的稳定失败码排序 |

阶段 04 是**唯一**会发出 `level_escalation` 的阶段——JSON Schema 与 CUE 有意放行 policy-grade + `local-only` 证据的组合，让 Rego 来输出稳定的失败码。

---

## 验证等级

```
                    evidence-grade  ──►  policy-grade  ──►  high-assurance
                    ──────────────       ────────────       ──────────────
本地执行               允许                禁止                禁止
local-only 证据        允许                禁止                禁止
已验证签名者           可选                必需                必需
已验证 builder         可选                必需                必需
isolated/witnessed     —                   —                  必需
已验证 witness         —                   —                  ≥1
approved-before-
  merge + digest       —                   —                  必需
runner 白名单          —                   —                  必需
```

| 等级 | 适用场景 | 能证明的事 | **不能**证明的事 |
|---|---|---|---|
| `evidence-grade` | 本地 CLI、开发流程、PR 早期证据 | 产出了一份 statement，把一次运行与 subject + 隐私设置绑定 | 本地机器未被入侵、agent metadata 真实可信 |
| `policy-grade` | CI / GitHub merge gate | 已签名 / 已验证信封、subject 严格等集、已验证非本地 builder / 签名者 / workflow / issuer | runner 不可攻陷、代码正确 |
| `high-assurance` | 隔离 runner、有见证（witness）的执行、受保护合并 | policy-grade 全部 **+** 白名单内的 isolated/witnessed builder、≥1 已验证 witness、已验证 `approved-before-merge` digest、新鲜度 / 重放上下文 | hypervisor / 固件 / SaaS 控制面完整性、无漏洞 |

**等级是一份策略 profile，不是绝对信任证明**：本地采集封顶 evidence-grade；CI/GitHub 可达 policy-grade；隔离 runner + witness + 强制 merge gate 才接近 high-assurance。

---

## 失败码

所有失败码均为**稳定字符串**——属于公共契约的一部分。Go 验证器按下表确定性顺序输出（定义在 [`internal/verify/failure_order.go`](internal/verify/failure_order.go)）。Rego deny 集本身无序，展示顺序由 Go 掌握。

| # | 失败码 | 含义 |
|---|---|---|
| 10  | `unsupported_statement_type` | `_type` 不是 in-toto Statement v1 |
| 20  | `predicate_type_mismatch` | `predicateType` ≠ `https://agentattest.dev/predicate/v0` |
| 30  | `unsupported_predicate_version` | `predicateVersion` ≠ `v0`（在 schema 之前被拒绝） |
| 40  | `schema_invalid` | JSON Schema 或 CUE 校验失败 |
| 50  | `signature_invalid` | DSSE / Sigstore 信封签名失败 |
| 60  | `transparency_verification_failed` | 必需的 Rekor / 时间戳 / witness 验证失败 |
| 70  | `subject_digest_mismatch` | statement subjects ≠ context subjects（集合相等） |
| 80  | `repo_url_mismatch` | predicate repo URL ≠ context repo URL |
| 90  | `base_commit_mismatch` | predicate base commit ≠ context base commit |
| 100 | `signer_identity_mismatch` | 已验证签名者缺失或 ≠ `agent.declaredIdentity` |
| 110 | `builder_identity_mismatch` | 已验证 builder / workflow / issuer 缺失或不匹配；self-hosted 未显式开启；high-assurance runner 不在白名单 |
| 120 | `replay_detected` | PR 号 / `runId` 与验证器上下文不一致 |
| 130 | `privacy_violation` | 原始 / trace 证据写入透明日志；非 public 内容写入透明日志；存在 public extension |
| 140 | `level_escalation` | policy-grade 或 high-assurance 搭配本地执行 / `local-only` 证据 / 缺少 builder / 缺少 high-assurance 证据 |
| 150 | `level_below_required` | predicate 等级低于 `context.requiredLevel` |
| 160 | `missing_evidence` | 必需的 trace / approval / witness / 摘要引用缺失 |
| 170 | `stale_attestation` | 超出新鲜度窗口；或窗口 > 0 但 `now` 缺失 |
| 180 | `policy_eval_error` | 策略引擎无法对确定性输入求值 |
| 190 | `cache_untrusted` | SQLite 缓存行与已验证 statement 冲突——缓存被忽略 |

> **多失败码排序**与 golden fixture 分开验证：每个 invalid golden 只期望**一个**失败码。多码排序的 fixture 位于 [`internal/verify/testdata/unsupported-version-subject-mismatch.json`](internal/verify/testdata/unsupported-version-subject-mismatch.json)。

---

## 隐私默认

默认规则：**不存储原始 prompt、原始工具输出、原始检索文档、原始模型响应。** 只存摘要、引用、脱敏摘要与加密 blob。

| 可见性 | 用途 |
|---|---|
| `public` | 摘要、predicate type、仓库 URL、subject 名称等非敏感字段 |
| `team` | 脱敏 trace summary、CI 产物、审批 metadata |
| `secret` | 敏感工具摘要、私有 issue 引用、内部 ID |
| `encrypted` | 原始 prompt、原始工具输出、私有 trace、客户数据 |

由 JSON Schema、CUE、Rego 共同强制的结构性硬规则：

- `rawPrompt.visibility` 与 `rawToolOutputs.visibility` **永远不可**为 `public`。
- 若 `stored == false`，`visibility` **必须**为 `none`。
- `evidence[].uri` 拒绝 `data:` 载荷及含 HTTP(S) `userinfo` 的 URI（如 `https://token:x@host/...`）。
- `extensions` 是 URI 键控、**closed**，每个值必须是 digest-addressed 引用对象——禁止内联 blob，禁止 `public` extension。
- `publicTransparencyLog.includesRawContent` 被约束为 `false`。

> 结构校验**不会**扫描引用工件的内容。脱敏属于工程纪律，而非安全证明。

详见 [`docs/PRIVACY_MODEL.md`](docs/PRIVACY_MODEL.md) 与 [`docs/THREAT_MODEL.md`](docs/THREAT_MODEL.md)。

---

## 项目结构

```
agentattest/
├── cmd/agentattest/                 CLI 入口——6 行，路由到 internal/app
├── internal/
│   ├── app/         编排            init · digest · predicate · verify
│   ├── gitbind/     确定性绑定      仓库 URL · base/head · patch sha256 · 文件 sha256
│   ├── predicate/   v0 谓词         构建 map + StatementFor() 辅助函数
│   ├── statement/   in-toto v1      parse / new / 类型化 Document
│   ├── verify/      流水线          phase01..05、OrderFailureCodes、golden_test、testdata
│   ├── policy/      Rego 适配器     封装 open-policy-agent/opa
│   ├── cache/       SQLite          refs · digests · timestamps（不存通过/失败结果）
│   └── contracts/   路径定位        向上查找 schema/CUE/policy/context
├── schemas/
│   ├── agent-provenance-v0.schema.json     JSON Schema 2020-12（v0）
│   ├── agent-provenance-v0.cue             CUE 跨字段语义（v0）
│   ├── agent-provenance-v1.schema.json     JSON Schema 2020-12（v1，前沿 harness）
│   └── agent-provenance-v1.cue             CUE 跨字段语义（v1）
├── policies/
│   └── default.rego                         OPA Rego 阶段-04 策略
├── tests/golden/                            ~30 个 fixture · context.schema.json
│   ├── valid-minimal/   valid-github-ci/   valid-high-assurance/
│   └── invalid-*/       （每个只期望一个失败码）
├── docs/
│   ├── DATA_MODEL.md           predicate 字段与语义
│   ├── FRONTIER_HARNESS_2026.md 2026 harness 现状评审 + 差距分析
│   ├── VERIFICATION_MODEL.md   等级、阶段、失败码
│   ├── PRIVACY_MODEL.md        默认与可见性分级
│   ├── THREAT_MODEL.md         威胁、缓解、残余风险
│   ├── NON_GOALS.md            硬范围边界
│   └── EXISTING_WHEELS.md      组合关系图
├── AGENTS.md                   入口地图与严格规则
├── ARCHITECTURE.md             分层、模块边界、禁止 import
├── TASKS.md                    v0 → v1 → 集成里程碑与验收标准
└── CLAUDE.md                   AI 代理在本仓库上的工作笔记
```

---

## 已有的轮子

`agentattest` 是一个**编排层**。下表所列能力全部以现成形态复用，**禁止**重新实现。

| 标准 / 工具 | 角色 |
|---|---|
| **in-toto Statement v1** | 外层容器 `_type / subject / predicateType / predicate` |
| **DSSE** | Statement 签名信封 |
| **Sigstore / cosign / Fulcio / Rekor** | 无密钥签名、OIDC 短证书、透明日志 |
| **GitHub Artifact Attestations** · `actions/attest` · `gh attestation` | GitHub 原生 attestation 存储与校验 |
| **SLSA provenance 词汇** | builder、workflow、materials 术语 |
| **OpenTelemetry / OpenInference** | 运行时 trace 传输与 AI 语义约定 |
| **SPDX / CycloneDX / OpenVEX** | SBOM / BOM / VEX 侧车 |
| **OPA / Rego** | 策略即代码（阶段 04） |
| **CUE** | Schema 与跨字段约束（阶段 03） |
| **JSON Schema 2020-12** | 结构校验（阶段 02） |
| **SQLite**（`modernc.org/sqlite`） | 本地缓存：仅 refs、digests、timestamps |
| **Git** | 来源身份——commit、ref、规范化 patch |

详见 [`docs/EXISTING_WHEELS.md`](docs/EXISTING_WHEELS.md)。

---

## 路线图

任务全程跟踪在 [`TASKS.md`](TASKS.md)，每条都有**确定性**验收标准——*"禁止任何任务依赖 AI 的主观判断来决定是否安全。"*

| 里程碑 | 范围 | 状态 |
|---|---|---|
| **v0 基础** | 骨架 · 谓词类型 · git 绑定 · 缓存 · in-toto 组装 · Rego 策略 · Golden 测试套 · 隐私门 | **已交付**——33 个 golden fixture 通过 |
| **v0 签名** | `internal/signing`：DSSE / Sigstore / GitHub-attestation 适配器，产出已验证的验证器上下文 | 未开始 |
| **v1 契约** | `agentConfig` · `mcpServers`/`tools` · `delegation` · `capture` · `platform-agent`——绑定 2026 harness 层面 | **本次升级已交付**——8 个新 golden fixture 通过 |
| **v0.1 集成** | GitHub Action · attestation 验证 · PR 摘要 · harness 采集 SDK · registry | 计划中 |

---

## 硬不变量

以下不变量由代码**与**评审双重强制——任一违反即为破坏性变更。

- `predicateType` **严格等于** `https://agentattest.dev/predicate/v0`。
- `predicate.predicateVersion` **严格等于** `v0`。不支持的版本在**阶段 01**被拒——早于 JSON Schema / CUE / Rego。
- 所有 predicate 对象 `additionalProperties: false`。`extensions` 虽为 URI 键控，但每个值都是 closed 的 digest-addressed 引用。
- 修改 schema 而不同步修改 CUE **且**不更新 golden fixture，即视为破坏性变更。
- 失败码是稳定字符串；改名即破坏公共契约。
- 原始 prompt、原始工具输出的 visibility **永远不可**为 `public`。`stored == false` 时 visibility 必为 `none`。
- `local-only` 证据**仅**允许在 evidence-grade 等级出现。Rego 拥有这条不变量，发出 `level_escalation`。
- subject digest 按**集合相等**校验，不允许超集。多余的 statement subject 一律拒绝。
- 已验证信封 / 证书数据**优先于** predicate 中的任何字段。缓存**永远**不参与通过/失败判定。
- 仅 `internal/signing` 可 import DSSE / cosign / Fulcio / Rekor / GitHub-attestation 库；其他包仅 hashing / digest 辅助函数可用 `crypto/*`。
- 外部命令使用 **argv 数组**，**严禁** shell 拼接字符串。

对 predicate 契约的破坏性变更必须使用**新的 predicate URI** 与**新的 golden fixture**——禁止原址修改。

---

## 许可证

本项目采用 Apache License 2.0 许可。详见 [LICENSE](LICENSE)。

---

## 贡献

任何改动之前请先阅读：

1. [`AGENTS.md`](AGENTS.md)——入口地图与严格规则。
2. [`ARCHITECTURE.md`](ARCHITECTURE.md)——模块边界、允许 / 禁止的 import。
3. 改动所涉及模块对应的文档（见 [`CLAUDE.md`](CLAUDE.md) 中的对照表）。
4. [`docs/NON_GOALS.md`](docs/NON_GOALS.md)——**禁止**未读此文就讨论范围扩张。

Predicate / Schema / CUE / 策略变更必须满足：

- Schema **与** CUE 同步更新。
- 在 [`tests/golden/`](tests/golden/) 中至少新增或更新一个 fixture。
- 失败码使用已有稳定字符串；新增失败码必须在 [`docs/VERIFICATION_MODEL.md`](docs/VERIFICATION_MODEL.md) 中说明。
- Linux、macOS、Windows 上 `go test ./...` 均通过。

---

<div align="center">

**信任来自已验证的信封、签名者 / builder / workflow 身份与重新计算的 subject digest——而非 predicate 中任何自报字段。**

**English** &nbsp;|&nbsp; [简体中文](./README.zh-CN.md)

</div>